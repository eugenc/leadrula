package intake

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/echayko/leadrula/backend/internal/auth"
	"github.com/echayko/leadrula/backend/internal/appointments"
	"github.com/echayko/leadrula/backend/internal/leads"
	"github.com/echayko/leadrula/backend/internal/routing"
	"github.com/echayko/leadrula/backend/pkg/httpx"
)

func (s *Service) ingestAppointmentFromSource(ctx context.Context, tx pgx.Tx, publisherID int64, src *routing.Source, slug string, raw map[string]any, maps []routing.SourceFieldMapEntry) (*IngestResult, error) {
	deliveryMode := src.DeliveryMode
	if deliveryMode == "" {
		deliveryMode = "contract"
	}
	switch deliveryMode {
	case "contract":
		if src.ContractID == nil || *src.ContractID == 0 {
			return nil, httpx.Validation("appointment source has no contract configured")
		}
	case "publisher", "publisher_pipeline":
		if src.CalendarID == nil || *src.CalendarID == 0 {
			return nil, httpx.Validation("appointment source has no calendar configured")
		}
	default:
		return nil, httpx.Validation("invalid appointment source delivery_mode")
	}

	rawJSON, _ := json.Marshal(raw)
	flat := flattenPayload(raw)
	authorName := sourceAuthorName(src.Name, slug)
	phone := extractPhoneFromPayload(flat, maps)
	phoneMode := src.PhoneMatchMode
	if phoneMode == "" {
		phoneMode = "update_and_book"
	}

	var leadID int64
	var publicID string
	var err error

	switch phoneMode {
	case "reject_duplicate":
		if phone != "" {
			_, lookupErr := s.leads.GetByPhoneNormalizedForPublisher(ctx, tx, publisherID, phone)
			if lookupErr == nil {
				return nil, httpx.Conflict("lead with this phone already exists")
			}
			var appErr *httpx.AppError
			if lookupErr != nil && !(errors.As(lookupErr, &appErr) && appErr.Code == httpx.CodeNotFound) {
				return nil, lookupErr
			}
		}
		leadID, publicID, err = s.insertMappedLead(ctx, tx, publisherID, slug, rawJSON, flat, maps, authorName)
		if err != nil {
			return nil, err
		}
	case "book_existing":
		if phone != "" {
			if existing, found, lookupErr := s.lookupExistingLeadByPhone(ctx, tx, publisherID, phone); lookupErr != nil {
				return nil, lookupErr
			} else if found {
				leadID = existing.ID
				publicID = existing.PublicID
				break
			}
		}
		leadID, publicID, err = s.insertMappedLead(ctx, tx, publisherID, slug, rawJSON, flat, maps, authorName)
		if err != nil {
			return nil, err
		}
	default: // update_and_book
		if phone != "" {
			if existing, updated, updateErr := s.updateExistingLeadByPhone(ctx, tx, publisherID, phone, 0, authorName, flat, maps); updateErr != nil {
				return nil, updateErr
			} else if updated {
				leadID = existing.ID
				publicID = existing.PublicID
				break
			}
		}
		leadID, publicID, err = s.insertMappedLead(ctx, tx, publisherID, slug, rawJSON, flat, maps, authorName)
		if err != nil {
			return nil, err
		}
	}

	lead, err := s.leads.GetByID(ctx, tx, leadID)
	if err != nil {
		return nil, err
	}
	slotStart, err := parseAppointmentSlotStart(flat, lead)
	if err != nil {
		return nil, err
	}

	p := &auth.Principal{AccountID: publisherID, AccountType: "publisher"}
	bookParams, err := s.resolveAppointmentBookParams(ctx, publisherID, src, slug, leadID, slotStart, flat)
	if err != nil {
		return nil, err
	}

	var result *appointments.BookingTxResult
	if deliveryMode == "contract" {
		result, err = s.appointments.BookFromSourceIngest(ctx, tx, p, bookParams)
	} else {
		result, err = s.appointments.BookFromSourceIngestCalendar(ctx, tx, p, bookParams)
	}
	if err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	s.notif.SendEmails(result.Emails)

	bookingID := result.BookingID
	return &IngestResult{
		LeadID:    publicID,
		Status:    result.Status,
		BookingID: &bookingID,
	}, nil
}

func (s *Service) resolveAppointmentBookParams(ctx context.Context, publisherID int64, src *routing.Source, slug string, leadID int64, slotStart time.Time, flat map[string]any) (appointments.BookParams, error) {
	deliveryMode := src.DeliveryMode
	if deliveryMode == "" {
		deliveryMode = "contract"
	}

	var bookParams appointments.BookParams
	var err error
	switch deliveryMode {
	case "contract":
		bookParams, err = s.appointments.ResolveSlotFromStart(ctx, *src.ContractID, slotStart)
	default:
		bookParams, err = s.appointments.ResolveSlotFromPublisherCalendar(ctx, publisherID, *src.CalendarID, slotStart)
	}
	if err != nil {
		return appointments.BookParams{}, err
	}

	bookParams.LeadID = leadID
	bookParams.Source = slug
	bookParams.DeliveryMode = deliveryMode
	if src.PublisherPipelineID != nil {
		bookParams.PublisherPipelineID = *src.PublisherPipelineID
	}
	if src.PublisherStageID != nil {
		bookParams.PublisherStageID = *src.PublisherStageID
	}
	if extID := toText(flat["external_event_id"]); extID != "" {
		bookParams.ExternalEventID = extID
	}
	return bookParams, nil
}

func (s *Service) insertMappedLead(ctx context.Context, tx pgx.Tx, publisherID int64, slug string, rawJSON []byte, flat map[string]any, maps []routing.SourceFieldMapEntry, authorName string) (int64, string, error) {
	leadID, publicID, err := s.leads.InsertLead(ctx, tx, publisherID, publisherID, slug, rawJSON)
	if err != nil {
		return 0, "", err
	}
	if err := applyPayloadMappings(ctx, tx, s.leads, publisherID, leadID, authorName, flat, maps); err != nil {
		return 0, "", err
	}
	return leadID, publicID, nil
}

func (s *Service) lookupExistingLeadByPhone(ctx context.Context, tx pgx.Tx, publisherID int64, phone string) (*leads.Lead, bool, error) {
	existing, err := s.leads.GetByPhoneNormalizedForPublisher(ctx, tx, publisherID, phone)
	if err != nil {
		var appErr *httpx.AppError
		if errors.As(err, &appErr) && appErr.Code == httpx.CodeNotFound {
			return nil, false, nil
		}
		return nil, false, err
	}
	return existing, true, nil
}

func parseAppointmentSlotStart(flat map[string]any, lead *leads.Lead) (time.Time, error) {
	if raw, ok := flat["slot_start"]; ok {
		if s := toText(raw); s != "" {
			t, err := time.Parse(time.RFC3339, s)
			if err != nil {
				return time.Time{}, httpx.Validation("invalid slot_start")
			}
			return t, nil
		}
	}
	if lead != nil && lead.ActionAt != nil {
		return *lead.ActionAt, nil
	}
	return time.Time{}, httpx.Validation("slot_start is required")
}
