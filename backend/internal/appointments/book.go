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
	CalendarID          int64
	BuyerSlotID         int64
	PublisherSlotID     int64
	SlotStart           time.Time
	DurationMin         int
	CustomTime          bool
	DeliveryMode        string
	LeadID              int64
	FirstName           string
	LastName            string
	Phone               string
	Email               string
	Source              string
	PublisherPipelineID int64
	PublisherStageID    int64
	ExternalEventID     string
	ExternalProvider    string
	BookingTarget       string
}

func (s *Service) Book(ctx context.Context, p *auth.Principal, params BookParams) (*BookingRow, error) {
	if params.CalendarID != 0 && params.ContractID != 0 {
		return nil, httpx.Validation("provide contract_id or calendar_id, not both")
	}
	if params.CalendarID != 0 {
		return s.bookPublisherCalendar(ctx, p, params)
	}
	return s.bookAppointment(ctx, p, params, false)
}

func (s *Service) BookAsBuyer(ctx context.Context, p *auth.Principal, params BookParams) (*BookingRow, error) {
	if params.CalendarID != 0 && params.ContractID != 0 {
		return nil, httpx.Validation("provide contract_id or calendar_id, not both")
	}
	if params.CalendarID != 0 {
		return s.bookBuyerCalendar(ctx, p, params)
	}
	params.DeliveryMode = "contract"
	return s.bookAppointment(ctx, p, params, true)
}

func (s *Service) bookAppointment(ctx context.Context, p *auth.Principal, params BookParams, asBuyer bool) (*BookingRow, error) {
	if params.ContractID == 0 {
		return nil, httpx.Validation("contract_id is required")
	}
	if params.DeliveryMode != "contract" && params.DeliveryMode != "publisher_pipeline" {
		return nil, httpx.Validation("delivery_mode must be contract or publisher_pipeline")
	}
	if params.DeliveryMode == "publisher_pipeline" && (params.PublisherPipelineID == 0 || params.PublisherStageID == 0) {
		return nil, httpx.Validation("publisher pipeline and stage required")
	}
	var buyerID int64
	var err error
	if asBuyer {
		if err := s.contractOwnedByBuyer(ctx, p.AccountID, params.ContractID); err != nil {
			return nil, err
		}
		buyerID = p.AccountID
	} else {
		buyerID, err = s.contractBuyerID(ctx, p.AccountID, params.ContractID)
		if err != nil {
			return nil, err
		}
	}
	target := parseBookingTarget(params.BookingTarget)
	ok, err := s.bookingCalendarConfigured(ctx, params.ContractID, asBuyer, target)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, httpx.Validation("appointment calendar is not configured")
	}
	active, err := s.resolveBookingCalendar(ctx, params.ContractID, asBuyer, target)
	if err != nil {
		return nil, err
	}
	var calTimezone string
	switch active.Source {
	case calendarSourceBuyer:
		cal, err := s.loadCalendarByID(ctx, active.CalendarID)
		if err != nil {
			return nil, err
		}
		calTimezone = cal.Timezone
	case calendarSourcePublisher:
		cal, err := s.loadPublisherCalendarByID(ctx, active.CalendarID)
		if err != nil {
			return nil, err
		}
		calTimezone = cal.Timezone
	default:
		return nil, httpx.Validation("appointment calendar is not configured")
	}
	loc := loadLocation(calTimezone)
	slotStart := params.SlotStart.In(loc)
	if !bookingWindowOK(slotStart, time.Now()) {
		return nil, httpx.Validation("slot is outside booking window")
	}

	var dur int
	var cap int
	if params.CustomTime {
		var err error
		dur, err = validateCustomDurationMin(params.DurationMin)
		if err != nil {
			return nil, err
		}
	} else {
		var err error
		switch active.Source {
		case calendarSourceBuyer:
			dur, cap, err = s.validateBuyerBookingSlot(ctx, p, params, asBuyer, slotStart, loc)
		case calendarSourcePublisher:
			dur, cap, err = s.validatePublisherBookingSlot(ctx, p, params, asBuyer, slotStart, loc)
		default:
			return nil, httpx.Validation("appointment calendar is not configured")
		}
		if err != nil {
			return nil, err
		}
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	if !params.CustomTime {
		var booked int
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

	var leadID int64
	var lead *leads.Lead
	if params.LeadID != 0 {
		leadID = params.LeadID
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

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	s.notif.SendEmails(emails)
	return s.getBookingByID(ctx, bookingID)
}

func (s *Service) validateBuyerBookingSlot(ctx context.Context, p *auth.Principal, params BookParams, asBuyer bool, slotStart time.Time, loc *time.Location) (int, int, error) {
	if params.BuyerSlotID == 0 {
		return 0, 0, httpx.Validation("buyer_slot_id is required")
	}
	target := parseBookingTarget(params.BookingTarget)
	var contractSlots []ContractSlot
	var err error
	switch target {
	case bookingTargetActive:
		contractSlots, err = s.listActiveContractBuyerSlots(ctx, params.ContractID)
	case bookingTargetCross:
		if asBuyer {
			return 0, 0, httpx.Validation("buyer cross-booking uses publisher calendar")
		}
		contractSlots, err = s.ListContractSlots(ctx, p.AccountID, params.ContractID)
	default:
		contractSlots, err = s.listOwnBuyerCalendarSlots(ctx, p.AccountID, params.ContractID, asBuyer)
	}
	if err != nil {
		return 0, 0, err
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
		return 0, 0, httpx.Validation("slot not available for this contract")
	}
	expected, err := combineDateAndTime(slotStart, match.StartTime, loc)
	if err != nil || !expected.Truncate(time.Minute).Equal(slotStart.Truncate(time.Minute)) {
		return 0, 0, httpx.Validation("slot_start does not match slot template")
	}
	buyerSlot := BuyerSlot{DurationMin: match.DurationMin, Capacity: match.Capacity}
	return effectiveDuration(buyerSlot, match), effectiveCapacity(buyerSlot, match), nil
}

func (s *Service) validatePublisherBookingSlot(ctx context.Context, p *auth.Principal, params BookParams, asBuyer bool, slotStart time.Time, loc *time.Location) (int, int, error) {
	if params.PublisherSlotID == 0 {
		return 0, 0, httpx.Validation("publisher_slot_id is required")
	}
	target := parseBookingTarget(params.BookingTarget)
	var contractSlots []ContractPublisherSlot
	var err error
	switch target {
	case bookingTargetActive:
		contractSlots, err = s.listActiveContractPublisherSlots(ctx, params.ContractID)
	case bookingTargetCross:
		if asBuyer {
			contractSlots, err = s.ListContractPublisherSlotsForBuyer(ctx, p.AccountID, params.ContractID)
		} else {
			return 0, 0, httpx.Validation("publisher cross-booking uses buyer calendar")
		}
	default:
		contractSlots, err = s.listOwnPublisherCalendarSlots(ctx, p.AccountID, params.ContractID, asBuyer)
	}
	if err != nil {
		return 0, 0, err
	}
	var match *ContractPublisherSlot
	for i := range contractSlots {
		cs := &contractSlots[i]
		if cs.PublisherSlotID == params.PublisherSlotID && cs.Enabled && !cs.Disabled {
			match = cs
			break
		}
	}
	if match == nil {
		return 0, 0, httpx.Validation("slot not available for this contract")
	}
	expected, err := combineDateAndTime(slotStart, match.StartTime, loc)
	if err != nil || !expected.Truncate(time.Minute).Equal(slotStart.Truncate(time.Minute)) {
		return 0, 0, httpx.Validation("slot_start does not match slot template")
	}
	pubSlot := PublisherSlot{DurationMin: match.DurationMin, Capacity: match.Capacity}
	return effectivePublisherDuration(pubSlot, match), effectivePublisherCapacity(pubSlot, match), nil
}

func (s *Service) countBuyerSlotOccupancyTx(ctx context.Context, tx pgx.Tx, buyerSlotID int64, slotStart time.Time) (int, error) {
	var n int
	err := tx.QueryRow(ctx, buyerSlotOccupancySQL, buyerSlotID, slotStart).Scan(&n)
	return n, err
}

func (s *Service) countPublisherSlotOccupancyTx(ctx context.Context, tx pgx.Tx, contractID, publisherSlotID int64, slotStart time.Time) (int, error) {
	var n int
	err := tx.QueryRow(ctx, publisherSlotOccupancySQL, contractID, publisherSlotID, slotStart).Scan(&n)
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

func (s *Service) createLeadForBuyerBooking(ctx context.Context, tx pgx.Tx, p *auth.Principal, contractID int64, params BookParams) (int64, error) {
	var publisherID int64
	if err := tx.QueryRow(ctx, `SELECT publisher_id FROM contracts WHERE id=$1`, contractID).Scan(&publisherID); err != nil {
		return 0, err
	}
	raw, _ := json.Marshal(map[string]string{
		"first_name": params.FirstName,
		"last_name":  params.LastName,
		"phone":      params.Phone,
		"email":      params.Email,
		"source":     params.Source,
	})
	source := strings.TrimSpace(params.Source)
	leadID, _, err := s.leads.InsertLead(ctx, tx, p.AccountID, publisherID, source, raw)
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

func nullStr(s string) any {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	return s
}

func validateCustomDurationMin(dur int) (int, error) {
	if dur < 15 || dur > 240 {
		return 0, httpx.Validation("duration_min must be between 15 and 240")
	}
	return dur, nil
}
