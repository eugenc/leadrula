package appointments

import (
	"context"
	"strings"
	"time"

	"github.com/echayko/leadrula/backend/internal/auth"
	"github.com/echayko/leadrula/backend/internal/leads"
	"github.com/echayko/leadrula/backend/internal/notifications"
	"github.com/echayko/leadrula/backend/pkg/httpx"
	"github.com/jackc/pgx/v5"
)

type preparedBooking struct {
	active    activeCalendar
	slotStart time.Time
	dur       int
	cap       int
	buyerID   int64
}

type BookingTxResult struct {
	BookingID int64
	Emails    []notifications.EmailJob
	Status    string
}

func (s *Service) prepareContractBooking(ctx context.Context, p *auth.Principal, params BookParams, asBuyer bool) (preparedBooking, error) {
	if params.ContractID == 0 {
		return preparedBooking{}, httpx.Validation("contract_id is required")
	}
	if params.DeliveryMode != "contract" && params.DeliveryMode != "publisher_pipeline" {
		return preparedBooking{}, httpx.Validation("delivery_mode must be contract or publisher_pipeline")
	}
	if params.DeliveryMode == "publisher_pipeline" && (params.PublisherPipelineID == 0 || params.PublisherStageID == 0) {
		return preparedBooking{}, httpx.Validation("publisher pipeline and stage required")
	}
	var buyerID int64
	var err error
	if asBuyer {
		if err := s.contractOwnedByBuyer(ctx, p.AccountID, params.ContractID); err != nil {
			return preparedBooking{}, err
		}
		buyerID = p.AccountID
	} else {
		buyerID, err = s.contractBuyerID(ctx, p.AccountID, params.ContractID)
		if err != nil {
			return preparedBooking{}, err
		}
	}
	target := parseBookingTarget(params.BookingTarget)
	ok, err := s.bookingCalendarConfigured(ctx, params.ContractID, asBuyer, target)
	if err != nil {
		return preparedBooking{}, err
	}
	if !ok {
		return preparedBooking{}, httpx.Validation("appointment calendar is not configured")
	}
	active, err := s.resolveBookingCalendar(ctx, params.ContractID, asBuyer, target)
	if err != nil {
		return preparedBooking{}, err
	}
	var calTimezone string
	switch active.Source {
	case calendarSourceBuyer:
		cal, err := s.loadCalendarByID(ctx, active.CalendarID)
		if err != nil {
			return preparedBooking{}, err
		}
		calTimezone = cal.Timezone
	case calendarSourcePublisher:
		cal, err := s.loadPublisherCalendarByID(ctx, active.CalendarID)
		if err != nil {
			return preparedBooking{}, err
		}
		calTimezone = cal.Timezone
	default:
		return preparedBooking{}, httpx.Validation("appointment calendar is not configured")
	}
	loc := loadLocation(calTimezone)
	slotStart := params.SlotStart.In(loc)
	if !bookingWindowOK(slotStart, time.Now()) {
		return preparedBooking{}, httpx.Validation("slot is outside booking window")
	}

	var dur int
	var cap int
	if params.CustomTime {
		dur, err = validateCustomDurationMin(params.DurationMin)
		if err != nil {
			return preparedBooking{}, err
		}
	} else {
		switch active.Source {
		case calendarSourceBuyer:
			dur, cap, err = s.validateBuyerBookingSlot(ctx, p, params, asBuyer, slotStart, loc)
		case calendarSourcePublisher:
			dur, cap, err = s.validatePublisherBookingSlot(ctx, p, params, asBuyer, slotStart, loc)
		default:
			return preparedBooking{}, httpx.Validation("appointment calendar is not configured")
		}
		if err != nil {
			return preparedBooking{}, err
		}
	}
	return preparedBooking{active: active, slotStart: slotStart, dur: dur, cap: cap, buyerID: buyerID}, nil
}

// BookFromSourceIngest books an appointment for an existing lead inside the caller transaction.
func (s *Service) BookFromSourceIngest(ctx context.Context, tx pgx.Tx, p *auth.Principal, params BookParams) (*BookingTxResult, error) {
	if params.LeadID == 0 {
		return nil, httpx.Validation("lead_id is required")
	}
	prep, err := s.prepareContractBooking(ctx, p, params, false)
	if err != nil {
		return nil, err
	}
	return s.executeBookAppointmentTx(ctx, tx, p, params, false, prep, params.LeadID)
}

// BookFromSourceIngestCalendar books on a publisher calendar inside the caller transaction.
func (s *Service) BookFromSourceIngestCalendar(ctx context.Context, tx pgx.Tx, p *auth.Principal, params BookParams) (*BookingTxResult, error) {
	if params.LeadID == 0 {
		return nil, httpx.Validation("lead_id is required")
	}
	if params.CalendarID == 0 {
		return nil, httpx.Validation("calendar_id is required")
	}
	if params.DeliveryMode != "publisher" && params.DeliveryMode != "publisher_pipeline" {
		return nil, httpx.Validation("delivery_mode must be publisher or publisher_pipeline")
	}
	if params.DeliveryMode == "publisher_pipeline" && (params.PublisherPipelineID == 0 || params.PublisherStageID == 0) {
		return nil, httpx.Validation("publisher pipeline and stage required")
	}

	cal, err := s.loadPublisherCalendar(ctx, p.AccountID, params.CalendarID)
	if err != nil {
		return nil, err
	}
	ok, err := s.publisherCalendarConfigured(ctx, params.CalendarID)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, httpx.Validation("appointment calendar is not configured")
	}
	loc := loadLocation(cal.Timezone)
	slotStart := params.SlotStart.In(loc)
	if !bookingWindowOK(slotStart, time.Now()) {
		return nil, httpx.Validation("slot is outside booking window")
	}

	var dur int
	var cap int
	if params.CustomTime {
		dur, err = validateCustomDurationMin(params.DurationMin)
		if err != nil {
			return nil, err
		}
	} else {
		dur, cap, err = s.validatePublisherCalendarBookingSlot(ctx, p, params, slotStart, loc)
		if err != nil {
			return nil, err
		}
	}

	if !params.CustomTime {
		booked, err := s.countPublisherCalendarSlotOccupancyTx(ctx, tx, params.CalendarID, params.PublisherSlotID, slotStart)
		if err != nil {
			return nil, err
		}
		if booked >= cap {
			return nil, httpx.Validation("slot is full")
		}
	}

	lead, err := s.leads.GetByID(ctx, tx, params.LeadID)
	if err != nil {
		return nil, err
	}
	if lead.PublisherID != p.AccountID && lead.OwnerAccountID != p.AccountID {
		return nil, httpx.NotFound("lead not found")
	}

	if _, err := tx.Exec(ctx, `DELETE FROM lead_appointment_bookings WHERE lead_id=$1`, params.LeadID); err != nil {
		return nil, err
	}

	extID := strings.TrimSpace(params.ExternalEventID)
	extProvider := strings.TrimSpace(params.ExternalProvider)
	if extProvider == "" && extID != "" {
		extProvider = "voiceuni"
	}
	if extID != "" {
		var existingID int64
		err = tx.QueryRow(ctx,
			`SELECT id FROM lead_appointment_bookings
			 WHERE external_provider_slug=$1 AND external_event_id=$2`,
			extProvider, extID).Scan(&existingID)
		if err == nil && existingID > 0 {
			_, err = tx.Exec(ctx, `DELETE FROM lead_appointment_bookings WHERE lead_id=$1 AND id <> $2`, params.LeadID, existingID)
		}
	}

	publisherSlotID := any(params.PublisherSlotID)
	if params.CustomTime {
		publisherSlotID = nil
	}
	var bookingID int64
	err = tx.QueryRow(ctx,
		`INSERT INTO lead_appointment_bookings(
		   publisher_calendar_id, lead_id, publisher_slot_id, slot_start, duration_min, booked_by_user_id, delivery_mode,
		   external_event_id, external_provider_slug, custom_time)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10) RETURNING id`,
		params.CalendarID, params.LeadID, publisherSlotID, slotStart, dur, p.UserID, params.DeliveryMode,
		nullStr(extID), nullStr(extProvider), params.CustomTime).Scan(&bookingID)
	if err != nil {
		return nil, err
	}

	var emails []notifications.EmailJob
	status := "booked"
	if params.DeliveryMode == "publisher_pipeline" && lead.Status == "review" && lead.OwnerAccountID == p.AccountID {
		if err := s.leads.PlaceInPipeline(ctx, tx, params.LeadID, p.AccountID, params.PublisherPipelineID, params.PublisherStageID, nil); err != nil {
			return nil, err
		}
		if err := s.leads.LogPipelinePlacement(ctx, tx, params.LeadID, leads.ActorFromPrincipal(p), params.PublisherPipelineID, params.PublisherStageID); err != nil {
			return nil, err
		}
	}

	updatedLead, err := s.leads.GetByID(ctx, tx, params.LeadID)
	if err != nil {
		return nil, err
	}
	priorActionAt := updatedLead.ActionAt
	if err := s.leads.SetActionAt(ctx, tx, params.LeadID, &slotStart); err != nil {
		return nil, err
	}
	if err := leads.LogActionAtChange(ctx, tx, s.leads, params.LeadID, updatedLead.OwnerAccountID, leads.ActorFromPrincipal(p), priorActionAt, &slotStart); err != nil {
		return nil, err
	}

	emails, err = s.finalizeBooking(ctx, tx, p, params.LeadID, lead, slotStart, p.AccountID, 0, emails)
	if err != nil {
		return nil, err
	}

	return &BookingTxResult{BookingID: bookingID, Emails: emails, Status: status}, nil
}

func (s *Service) executeBookAppointmentTx(ctx context.Context, tx pgx.Tx, p *auth.Principal, params BookParams, asBuyer bool, prep preparedBooking, leadID int64) (*BookingTxResult, error) {
	active := prep.active
	slotStart := prep.slotStart
	dur := prep.dur
	cap := prep.cap
	buyerID := prep.buyerID

	if !params.CustomTime {
		var booked int
		var err error
		switch active.Source {
		case calendarSourceBuyer:
			booked, err = s.countBuyerSlotOccupancyTx(ctx, tx, params.BuyerSlotID, slotStart)
		case calendarSourcePublisher:
			booked, err = s.countPublisherSlotOccupancyTx(ctx, tx, params.ContractID, params.PublisherSlotID, slotStart)
		}
		if err != nil {
			return nil, err
		}
		if booked >= cap {
			return nil, httpx.Validation("slot is full")
		}
	}

	var lead *leads.Lead
	var err error
	if leadID != 0 {
		lead, err = s.leads.GetByID(ctx, tx, leadID)
		if err != nil {
			return nil, err
		}
		if asBuyer {
			if lead.OwnerAccountID != p.AccountID {
				return nil, httpx.NotFound("lead not found")
			}
		} else if lead.PublisherID != p.AccountID && lead.OwnerAccountID != p.AccountID {
			return nil, httpx.NotFound("lead not found")
		}
	} else {
		if strings.TrimSpace(params.FirstName) == "" || strings.TrimSpace(params.LastName) == "" {
			return nil, httpx.Validation("first_name and last_name are required")
		}
		if strings.TrimSpace(params.Phone) == "" && strings.TrimSpace(params.Email) == "" {
			return nil, httpx.Validation("phone or email is required")
		}
		if asBuyer {
			leadID, err = s.createLeadForBuyerBooking(ctx, tx, p, params.ContractID, params)
		} else {
			leadID, err = s.createLeadForBooking(ctx, tx, p, params)
		}
		if err != nil {
			return nil, err
		}
		lead, err = s.leads.GetByID(ctx, tx, leadID)
		if err != nil {
			return nil, err
		}
	}

	if _, err := tx.Exec(ctx, `DELETE FROM lead_appointment_bookings WHERE lead_id=$1`, leadID); err != nil {
		return nil, err
	}

	extID := strings.TrimSpace(params.ExternalEventID)
	extProvider := strings.TrimSpace(params.ExternalProvider)
	if extProvider == "" && extID != "" {
		extProvider = "voiceuni"
	}
	if extID != "" {
		var existingID int64
		err = tx.QueryRow(ctx,
			`SELECT id FROM lead_appointment_bookings
			 WHERE external_provider_slug=$1 AND external_event_id=$2`,
			extProvider, extID).Scan(&existingID)
		if err == nil && existingID > 0 {
			_, err = tx.Exec(ctx, `DELETE FROM lead_appointment_bookings WHERE lead_id=$1 AND id <> $2`, leadID, existingID)
		}
	}

	var bookingID int64
	switch active.Source {
	case calendarSourceBuyer:
		buyerSlotID := any(params.BuyerSlotID)
		if params.CustomTime {
			buyerSlotID = nil
		}
		err = tx.QueryRow(ctx,
			`INSERT INTO lead_appointment_bookings(
			   contract_id, lead_id, buyer_slot_id, slot_start, duration_min, booked_by_user_id, delivery_mode,
			   external_event_id, external_provider_slug, custom_time)
			 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10) RETURNING id`,
			params.ContractID, leadID, buyerSlotID, slotStart, dur, p.UserID, params.DeliveryMode,
			nullStr(extID), nullStr(extProvider), params.CustomTime).Scan(&bookingID)
	case calendarSourcePublisher:
		publisherSlotID := any(params.PublisherSlotID)
		if params.CustomTime {
			publisherSlotID = nil
		}
		err = tx.QueryRow(ctx,
			`INSERT INTO lead_appointment_bookings(
			   contract_id, lead_id, publisher_slot_id, slot_start, duration_min, booked_by_user_id, delivery_mode,
			   external_event_id, external_provider_slug, custom_time)
			 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10) RETURNING id`,
			params.ContractID, leadID, publisherSlotID, slotStart, dur, p.UserID, params.DeliveryMode,
			nullStr(extID), nullStr(extProvider), params.CustomTime).Scan(&bookingID)
	}
	if err != nil {
		return nil, err
	}

	deps := leads.RouteApplyDeps{Repo: s.leads, Accounts: s.accounts, Notif: s.notif}
	var emails []notifications.EmailJob
	status := "booked"

	if params.DeliveryMode == "contract" && lead.Status != "distributed" && lead.Status != "closed" {
		delivery := ""
		_ = tx.QueryRow(ctx,
			`SELECT COALESCE(cc.delivery,'') FROM contract_compensations cc
			 WHERE cc.contract_id=$1 AND cc.trigger='per_lead' ORDER BY cc.position, cc.id LIMIT 1`,
			params.ContractID).Scan(&delivery)
		em, err := leads.DistributeToContract(ctx, tx, deps, params.ContractID, delivery, leadID, "appointment booked")
		if err != nil {
			return nil, err
		}
		emails = append(emails, em...)
		status = "distributed"
	} else if !asBuyer && params.DeliveryMode == "publisher_pipeline" && lead.Status == "review" && lead.OwnerAccountID == p.AccountID {
		if err := s.leads.PlaceInPipeline(ctx, tx, leadID, p.AccountID, params.PublisherPipelineID, params.PublisherStageID, nil); err != nil {
			return nil, err
		}
		if err := s.leads.LogPipelinePlacement(ctx, tx, leadID, leads.ActorFromPrincipal(p), params.PublisherPipelineID, params.PublisherStageID); err != nil {
			return nil, err
		}
		if err := s.leads.SetPreassignedBuyer(ctx, p.AccountID, leadID, &buyerID); err != nil {
			return nil, err
		}
	}

	updatedLead, err := s.leads.GetByID(ctx, tx, leadID)
	if err != nil {
		return nil, err
	}
	priorActionAt := updatedLead.ActionAt
	if err := s.leads.SetActionAt(ctx, tx, leadID, &slotStart); err != nil {
		return nil, err
	}
	if err := leads.LogActionAtChange(ctx, tx, s.leads, leadID, updatedLead.OwnerAccountID, leads.ActorFromPrincipal(p), priorActionAt, &slotStart); err != nil {
		return nil, err
	}

	leadName := strings.TrimSpace(lead.FirstName + " " + lead.LastName)
	adminIDs, err := s.accounts.AdminUserIDs(ctx, tx, buyerID)
	if err != nil {
		return nil, err
	}
	notifyEmails, err := s.notif.Deliver(ctx, tx, notifications.DeliverParams{
		AccountID: buyerID,
		UserIDs:   adminIDs,
		EventType: "new_appointment",
		Payload: map[string]any{
			"lead_id":     leadID,
			"contract_id": params.ContractID,
			"slot_start":  slotStart.Format(time.RFC3339),
			"lead_name":   leadName,
		},
	})
	if err != nil {
		return nil, err
	}
	emails = append(emails, notifyEmails...)

	return &BookingTxResult{BookingID: bookingID, Emails: emails, Status: status}, nil
}
