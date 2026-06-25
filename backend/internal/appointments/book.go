package appointments

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/echayko/leadrula/backend/internal/auth"
	"github.com/echayko/leadrula/backend/internal/leads"
	"github.com/echayko/leadrula/backend/internal/notifications"
	"github.com/echayko/leadrula/backend/pkg/httpx"
	"github.com/jackc/pgx/v5"
)

type BookParams struct {
	ContractID          int64
	BuyerSlotID         int64
	SlotStart           time.Time
	DeliveryMode        string
	LeadID              int64
	FirstName           string
	LastName            string
	Phone               string
	Email               string
	Source              string
	PublisherPipelineID int64
	PublisherStageID    int64
}

func (s *Service) Book(ctx context.Context, p *auth.Principal, params BookParams) (*BookingRow, error) {
	if params.DeliveryMode != "contract" && params.DeliveryMode != "publisher_pipeline" {
		return nil, httpx.Validation("delivery_mode must be contract or publisher_pipeline")
	}
	if params.DeliveryMode == "publisher_pipeline" && (params.PublisherPipelineID == 0 || params.PublisherStageID == 0) {
		return nil, httpx.Validation("publisher pipeline and stage required")
	}
	buyerID, err := s.contractBuyerID(ctx, p.AccountID, params.ContractID)
	if err != nil {
		return nil, err
	}
	ok, err := s.contractCalendarConfigured(ctx, params.ContractID)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, httpx.Validation("buyer has not configured availability")
	}
	calID, err := s.contractCalendarID(ctx, params.ContractID)
	if err != nil {
		return nil, err
	}
	cal, err := s.loadCalendarByID(ctx, calID)
	if err != nil {
		return nil, err
	}
	_ = buyerID
	loc := loadLocation(cal.Timezone)
	slotStart := params.SlotStart.In(loc)
	if !bookingWindowOK(slotStart, time.Now()) {
		return nil, httpx.Validation("slot is outside booking window")
	}
	contractSlots, err := s.ListContractSlots(ctx, p.AccountID, params.ContractID)
	if err != nil {
		return nil, err
	}
	var match *ContractSlot
	for i := range contractSlots {
		cs := &contractSlots[i]
		if cs.BuyerSlotID == params.BuyerSlotID && cs.Enabled && !cs.Disabled {
			match = cs
			break
		}
	}
	if match == nil {
		return nil, httpx.Validation("slot not available for this contract")
	}
	expected, err := combineDateAndTime(slotStart, match.StartTime, loc)
	if err != nil || !expected.Truncate(time.Minute).Equal(slotStart.Truncate(time.Minute)) {
		return nil, httpx.Validation("slot_start does not match slot template")
	}
	buyerSlot := BuyerSlot{DurationMin: match.DurationMin, Capacity: match.Capacity}
	dur := effectiveDuration(buyerSlot, match)
	cap := effectiveCapacity(buyerSlot, match)

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	booked, err := s.countBookingsTx(ctx, tx, params.ContractID, slotStart)
	if err != nil {
		return nil, err
	}
	if booked >= cap {
		return nil, httpx.Validation("slot is full")
	}

	var leadID int64
	var lead *leads.Lead
	if params.LeadID != 0 {
		leadID = params.LeadID
		lead, err = s.leads.GetByID(ctx, tx, leadID)
		if err != nil {
			return nil, err
		}
		if lead.PublisherID != p.AccountID && lead.OwnerAccountID != p.AccountID {
			return nil, httpx.NotFound("lead not found")
		}
	} else {
		if strings.TrimSpace(params.FirstName) == "" || strings.TrimSpace(params.LastName) == "" {
			return nil, httpx.Validation("first_name and last_name are required")
		}
		if strings.TrimSpace(params.Phone) == "" && strings.TrimSpace(params.Email) == "" {
			return nil, httpx.Validation("phone or email is required")
		}
		leadID, err = s.createLeadForBooking(ctx, tx, p, params)
		if err != nil {
			return nil, err
		}
		lead, err = s.leads.GetByID(ctx, tx, leadID)
		if err != nil {
			return nil, err
		}
	}

	if err := s.leads.SetActionAt(ctx, tx, leadID, &slotStart); err != nil {
		return nil, err
	}
	if _, err := tx.Exec(ctx, `DELETE FROM lead_appointment_bookings WHERE lead_id=$1`, leadID); err != nil {
		return nil, err
	}

	var bookingID int64
	err = tx.QueryRow(ctx,
		`INSERT INTO lead_appointment_bookings(
		   contract_id, lead_id, buyer_slot_id, slot_start, duration_min, booked_by_user_id, delivery_mode)
		 VALUES ($1,$2,$3,$4,$5,$6,$7) RETURNING id`,
		params.ContractID, leadID, params.BuyerSlotID, slotStart, dur, p.UserID, params.DeliveryMode).Scan(&bookingID)
	if err != nil {
		return nil, err
	}

	deps := leads.RouteApplyDeps{Repo: s.leads, Accounts: s.accounts, Notif: s.notif}
	var emails []notifications.EmailJob

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
	} else if params.DeliveryMode == "publisher_pipeline" && lead.Status == "review" && lead.OwnerAccountID == p.AccountID {
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

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	s.notif.SendEmails(emails)
	return s.getBookingByID(ctx, bookingID)
}

func (s *Service) countBookingsTx(ctx context.Context, tx pgx.Tx, contractID int64, slotStart time.Time) (int, error) {
	var n int
	err := tx.QueryRow(ctx,
		`SELECT COUNT(*)::int FROM lead_appointment_bookings
		 WHERE contract_id=$1 AND slot_start=$2`, contractID, slotStart).Scan(&n)
	return n, err
}

func (s *Service) createLeadForBooking(ctx context.Context, tx pgx.Tx, p *auth.Principal, params BookParams) (int64, error) {
	raw, _ := json.Marshal(map[string]string{
		"first_name": params.FirstName,
		"last_name":  params.LastName,
		"phone":      params.Phone,
		"email":      params.Email,
		"source":     params.Source,
	})
	source := strings.TrimSpace(params.Source)
	leadID, _, err := s.leads.InsertLead(ctx, tx, p.AccountID, p.AccountID, source, raw)
	if err != nil {
		return 0, err
	}
	for field, val := range map[string]string{
		"first_name": params.FirstName,
		"last_name":  params.LastName,
		"phone":      params.Phone,
		"email":      params.Email,
		"source":     source,
	} {
		if strings.TrimSpace(val) == "" {
			continue
		}
		if err := s.leads.SetBuiltinField(ctx, tx, leadID, field, val); err != nil {
			return 0, err
		}
	}
	if err := s.leads.LogLeadCreated(ctx, tx, leadID, p.AccountID, leads.ActorFromPrincipal(p), source); err != nil {
		return 0, err
	}
	return leadID, nil
}

func (s *Service) getBookingByID(ctx context.Context, id int64) (*BookingRow, error) {
	rows, err := s.listBookingsQuery(ctx, `b.id=$1`, []any{id}, "b.created_at DESC", 0)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, httpx.NotFound("booking not found")
	}
	return &rows[0], nil
}
