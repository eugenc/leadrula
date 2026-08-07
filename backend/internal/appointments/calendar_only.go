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

const publisherCalendarSlotOccupancySQL = `
SELECT COUNT(*)::int FROM lead_appointment_bookings b
WHERE b.publisher_calendar_id = $1 AND b.publisher_slot_id = $2 AND b.slot_start = $3`

const buyerCalendarSlotOccupancySQL = `
SELECT COUNT(*)::int FROM lead_appointment_bookings b
WHERE b.buyer_calendar_id = $1 AND b.buyer_slot_id = $2 AND b.slot_start = $3`

func (s *Service) ListFreeSlotsByPublisherCalendar(ctx context.Context, publisherID, calendarID int64, dateStr string) ([]FreeSlot, error) {
	cal, err := s.loadPublisherCalendar(ctx, publisherID, calendarID)
	if err != nil {
		return nil, err
	}
	ok, err := s.publisherCalendarConfigured(ctx, calendarID)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, httpx.Validation("appointment calendar is not configured")
	}
	loc := loadLocation(cal.Timezone)
	date, err := parseDateParam(dateStr, loc)
	if err != nil {
		return nil, httpx.Validation(err.Error())
	}
	slots, err := s.listPublisherCalendarSlotsDirect(ctx, publisherID, calendarID)
	if err != nil {
		return nil, err
	}
	return s.buildPublisherFreeSlotsForCalendar(ctx, calendarID, slots, date, loc)
}

func (s *Service) ListFreeSlotsByBuyerCalendar(ctx context.Context, buyerID, calendarID int64, dateStr string) ([]FreeSlot, error) {
	cal, err := s.loadCalendar(ctx, buyerID, calendarID)
	if err != nil {
		return nil, err
	}
	ok, err := s.calendarConfigured(ctx, calendarID)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, httpx.Validation("appointment calendar is not configured")
	}
	loc := loadLocation(cal.Timezone)
	date, err := parseDateParam(dateStr, loc)
	if err != nil {
		return nil, httpx.Validation(err.Error())
	}
	slots, err := s.listBuyerCalendarSlotsDirect(ctx, buyerID, calendarID)
	if err != nil {
		return nil, err
	}
	return s.buildBuyerFreeSlotsForCalendar(ctx, calendarID, slots, date, loc)
}

func (s *Service) listPublisherCalendarSlotsDirect(ctx context.Context, publisherID, calendarID int64) ([]ContractPublisherSlot, error) {
	if _, err := s.loadPublisherCalendar(ctx, publisherID, calendarID); err != nil {
		return nil, err
	}
	rows, err := s.pool.Query(ctx,
		`SELECT sl.id, sl.weekday, sl.start_time::text, sl.duration_min, sl.capacity,
		        true, NULL::int, NULL::int, sl.disabled_at IS NOT NULL
		 FROM publisher_appointment_slots sl
		 WHERE sl.calendar_id = $1
		 ORDER BY sl.weekday, sl.start_time`, calendarID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanContractPublisherSlots(rows)
}

func (s *Service) listBuyerCalendarSlotsDirect(ctx context.Context, buyerID, calendarID int64) ([]ContractSlot, error) {
	if _, err := s.loadCalendar(ctx, buyerID, calendarID); err != nil {
		return nil, err
	}
	rows, err := s.pool.Query(ctx,
		`SELECT sl.id, sl.weekday, sl.start_time::text, sl.duration_min, sl.capacity,
		        true, NULL::int, NULL::int, sl.disabled_at IS NOT NULL
		 FROM buyer_appointment_slots sl
		 WHERE sl.calendar_id = $1
		 ORDER BY sl.weekday, sl.start_time`, calendarID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanContractSlots(rows)
}

func (s *Service) buildPublisherFreeSlotsForCalendar(ctx context.Context, calendarID int64, contractSlots []ContractPublisherSlot, date time.Time, loc *time.Location) ([]FreeSlot, error) {
	now := time.Now()
	var out []FreeSlot
	weekday := int(date.Weekday())
	for _, cs := range contractSlots {
		if !cs.Enabled || cs.Disabled || cs.Weekday != weekday {
			continue
		}
		slot := PublisherSlot{
			ID: cs.PublisherSlotID, Weekday: cs.Weekday, StartTime: cs.StartTime,
			DurationMin: cs.DurationMin, Capacity: cs.Capacity,
		}
		dur := effectivePublisherDuration(slot, &cs)
		cap := effectivePublisherCapacity(slot, &cs)
		slotStart, err := combineDateAndTime(date, cs.StartTime, loc)
		if err != nil {
			continue
		}
		if !bookingWindowOK(slotStart, now) {
			continue
		}
		booked, err := s.countPublisherCalendarSlotOccupancy(ctx, calendarID, cs.PublisherSlotID, slotStart)
		if err != nil {
			return nil, err
		}
		remaining := cap - booked
		if remaining <= 0 {
			continue
		}
		out = append(out, FreeSlot{
			PublisherSlotID:   cs.PublisherSlotID,
			SlotStart:         slotStart,
			DurationMin:       dur,
			Capacity:          cap,
			RemainingCapacity: remaining,
		})
	}
	return out, nil
}

func (s *Service) buildBuyerFreeSlotsForCalendar(ctx context.Context, calendarID int64, contractSlots []ContractSlot, date time.Time, loc *time.Location) ([]FreeSlot, error) {
	now := time.Now()
	var out []FreeSlot
	weekday := int(date.Weekday())
	for _, cs := range contractSlots {
		if !cs.Enabled || cs.Disabled || cs.Weekday != weekday {
			continue
		}
		slot := BuyerSlot{
			ID: cs.BuyerSlotID, Weekday: cs.Weekday, StartTime: cs.StartTime,
			DurationMin: cs.DurationMin, Capacity: cs.Capacity,
		}
		dur := effectiveDuration(slot, &cs)
		cap := effectiveCapacity(slot, &cs)
		slotStart, err := combineDateAndTime(date, cs.StartTime, loc)
		if err != nil {
			continue
		}
		if !bookingWindowOK(slotStart, now) {
			continue
		}
		booked, err := s.countBuyerCalendarSlotOccupancy(ctx, calendarID, cs.BuyerSlotID, slotStart)
		if err != nil {
			return nil, err
		}
		remaining := cap - booked
		if remaining <= 0 {
			continue
		}
		out = append(out, FreeSlot{
			BuyerSlotID:       cs.BuyerSlotID,
			SlotStart:         slotStart,
			DurationMin:       dur,
			Capacity:          cap,
			RemainingCapacity: remaining,
		})
	}
	return out, nil
}

func (s *Service) countPublisherCalendarSlotOccupancy(ctx context.Context, calendarID, publisherSlotID int64, slotStart time.Time) (int, error) {
	var n int
	err := s.pool.QueryRow(ctx, publisherCalendarSlotOccupancySQL, calendarID, publisherSlotID, slotStart).Scan(&n)
	return n, err
}

func (s *Service) countPublisherCalendarSlotOccupancyTx(ctx context.Context, tx pgx.Tx, calendarID, publisherSlotID int64, slotStart time.Time) (int, error) {
	var n int
	err := tx.QueryRow(ctx, publisherCalendarSlotOccupancySQL, calendarID, publisherSlotID, slotStart).Scan(&n)
	return n, err
}

func (s *Service) countBuyerCalendarSlotOccupancy(ctx context.Context, calendarID, buyerSlotID int64, slotStart time.Time) (int, error) {
	var n int
	err := s.pool.QueryRow(ctx, buyerCalendarSlotOccupancySQL, calendarID, buyerSlotID, slotStart).Scan(&n)
	return n, err
}

func (s *Service) countBuyerCalendarSlotOccupancyTx(ctx context.Context, tx pgx.Tx, calendarID, buyerSlotID int64, slotStart time.Time) (int, error) {
	var n int
	err := tx.QueryRow(ctx, buyerCalendarSlotOccupancySQL, calendarID, buyerSlotID, slotStart).Scan(&n)
	return n, err
}

func (s *Service) bookPublisherCalendar(ctx context.Context, p *auth.Principal, params BookParams) (*BookingRow, error) {
	if params.DeliveryMode != "publisher_pipeline" {
		return nil, httpx.Validation("calendar-only booking requires delivery_mode publisher_pipeline")
	}
	if params.PublisherPipelineID == 0 || params.PublisherStageID == 0 {
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
		var err error
		dur, err = validateCustomDurationMin(params.DurationMin)
		if err != nil {
			return nil, err
		}
	} else {
		var err error
		dur, cap, err = s.validatePublisherCalendarBookingSlot(ctx, p, params, slotStart, loc)
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
		booked, err := s.countPublisherCalendarSlotOccupancyTx(ctx, tx, params.CalendarID, params.PublisherSlotID, slotStart)
		if err != nil {
			return nil, err
		}
		if booked >= cap {
			return nil, httpx.Validation("slot is full")
		}
	}

	leadID, lead, err := s.resolveBookingLead(ctx, tx, p, params, false)
	if err != nil {
		return nil, err
	}

	if _, err := tx.Exec(ctx, `DELETE FROM lead_appointment_bookings WHERE lead_id=$1`, leadID); err != nil {
		return nil, err
	}

	var bookingID int64
	publisherSlotID := any(params.PublisherSlotID)
	if params.CustomTime {
		publisherSlotID = nil
	}
	err = tx.QueryRow(ctx,
		`INSERT INTO lead_appointment_bookings(
		   publisher_calendar_id, lead_id, publisher_slot_id, slot_start, duration_min, booked_by_user_id, delivery_mode,
		   external_event_id, external_provider_slug, custom_time)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10) RETURNING id`,
		params.CalendarID, leadID, publisherSlotID, slotStart, dur, p.UserID, params.DeliveryMode,
		nullStr(params.ExternalEventID), nullStr(externalProvider(params)), params.CustomTime).Scan(&bookingID)
	if err != nil {
		return nil, err
	}

	var emails []notifications.EmailJob
	if lead.Status == "review" && lead.OwnerAccountID == p.AccountID {
		if err := s.leads.PlaceInPipeline(ctx, tx, leadID, p.AccountID, params.PublisherPipelineID, params.PublisherStageID, nil); err != nil {
			return nil, err
		}
		if err := s.leads.LogPipelinePlacement(ctx, tx, leadID, leads.ActorFromPrincipal(p), params.PublisherPipelineID, params.PublisherStageID); err != nil {
			return nil, err
		}
	}

	emails, err = s.finalizeBooking(ctx, tx, p, leadID, lead, slotStart, p.AccountID, 0, emails)
	if err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	s.notif.SendEmails(emails)
	return s.getBookingByID(ctx, bookingID)
}

func (s *Service) bookBuyerCalendar(ctx context.Context, p *auth.Principal, params BookParams) (*BookingRow, error) {
	cal, err := s.loadCalendar(ctx, p.AccountID, params.CalendarID)
	if err != nil {
		return nil, err
	}
	ok, err := s.calendarConfigured(ctx, params.CalendarID)
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
		var err error
		dur, err = validateCustomDurationMin(params.DurationMin)
		if err != nil {
			return nil, err
		}
	} else {
		var err error
		dur, cap, err = s.validateBuyerCalendarBookingSlot(ctx, p, params, slotStart, loc)
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
		booked, err := s.countBuyerCalendarSlotOccupancyTx(ctx, tx, params.CalendarID, params.BuyerSlotID, slotStart)
		if err != nil {
			return nil, err
		}
		if booked >= cap {
			return nil, httpx.Validation("slot is full")
		}
	}

	leadID, lead, err := s.resolveBookingLead(ctx, tx, p, params, true)
	if err != nil {
		return nil, err
	}

	if _, err := tx.Exec(ctx, `DELETE FROM lead_appointment_bookings WHERE lead_id=$1`, leadID); err != nil {
		return nil, err
	}

	var bookingID int64
	buyerSlotID := any(params.BuyerSlotID)
	if params.CustomTime {
		buyerSlotID = nil
	}
	err = tx.QueryRow(ctx,
		`INSERT INTO lead_appointment_bookings(
		   buyer_calendar_id, lead_id, buyer_slot_id, slot_start, duration_min, booked_by_user_id, delivery_mode,
		   external_event_id, external_provider_slug, custom_time)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10) RETURNING id`,
		params.CalendarID, leadID, buyerSlotID, slotStart, dur, p.UserID, "contract",
		nullStr(params.ExternalEventID), nullStr(externalProvider(params)), params.CustomTime).Scan(&bookingID)
	if err != nil {
		return nil, err
	}

	emails, err := s.finalizeBooking(ctx, tx, p, leadID, lead, slotStart, p.AccountID, 0, nil)
	if err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	s.notif.SendEmails(emails)
	return s.getBookingByID(ctx, bookingID)
}

func (s *Service) validatePublisherCalendarBookingSlot(ctx context.Context, p *auth.Principal, params BookParams, slotStart time.Time, loc *time.Location) (int, int, error) {
	if params.PublisherSlotID == 0 {
		return 0, 0, httpx.Validation("publisher_slot_id is required")
	}
	slots, err := s.listPublisherCalendarSlotsDirect(ctx, p.AccountID, params.CalendarID)
	if err != nil {
		return 0, 0, err
	}
	var match *ContractPublisherSlot
	for i := range slots {
		cs := &slots[i]
		if cs.PublisherSlotID == params.PublisherSlotID && cs.Enabled && !cs.Disabled {
			match = cs
			break
		}
	}
	if match == nil {
		return 0, 0, httpx.Validation("slot not available for this calendar")
	}
	expected, err := combineDateAndTime(slotStart, match.StartTime, loc)
	if err != nil || !expected.Truncate(time.Minute).Equal(slotStart.Truncate(time.Minute)) {
		return 0, 0, httpx.Validation("slot_start does not match slot template")
	}
	pubSlot := PublisherSlot{DurationMin: match.DurationMin, Capacity: match.Capacity}
	return effectivePublisherDuration(pubSlot, match), effectivePublisherCapacity(pubSlot, match), nil
}

func (s *Service) validateBuyerCalendarBookingSlot(ctx context.Context, p *auth.Principal, params BookParams, slotStart time.Time, loc *time.Location) (int, int, error) {
	if params.BuyerSlotID == 0 {
		return 0, 0, httpx.Validation("buyer_slot_id is required")
	}
	slots, err := s.listBuyerCalendarSlotsDirect(ctx, p.AccountID, params.CalendarID)
	if err != nil {
		return 0, 0, err
	}
	var match *ContractSlot
	for i := range slots {
		cs := &slots[i]
		if cs.BuyerSlotID == params.BuyerSlotID && cs.Enabled && !cs.Disabled {
			match = cs
			break
		}
	}
	if match == nil {
		return 0, 0, httpx.Validation("slot not available for this calendar")
	}
	expected, err := combineDateAndTime(slotStart, match.StartTime, loc)
	if err != nil || !expected.Truncate(time.Minute).Equal(slotStart.Truncate(time.Minute)) {
		return 0, 0, httpx.Validation("slot_start does not match slot template")
	}
	buyerSlot := BuyerSlot{DurationMin: match.DurationMin, Capacity: match.Capacity}
	return effectiveDuration(buyerSlot, match), effectiveCapacity(buyerSlot, match), nil
}

func (s *Service) resolveBookingLead(ctx context.Context, tx pgx.Tx, p *auth.Principal, params BookParams, asBuyer bool) (int64, *leads.Lead, error) {
	if params.LeadID != 0 {
		lead, err := s.leads.GetByID(ctx, tx, params.LeadID)
		if err != nil {
			return 0, nil, err
		}
		if asBuyer {
			if lead.OwnerAccountID != p.AccountID {
				return 0, nil, httpx.NotFound("lead not found")
			}
		} else if lead.PublisherID != p.AccountID && lead.OwnerAccountID != p.AccountID {
			return 0, nil, httpx.NotFound("lead not found")
		}
		return params.LeadID, lead, nil
	}
	if strings.TrimSpace(params.FirstName) == "" || strings.TrimSpace(params.LastName) == "" {
		return 0, nil, httpx.Validation("first_name and last_name are required")
	}
	if strings.TrimSpace(params.Phone) == "" && strings.TrimSpace(params.Email) == "" {
		return 0, nil, httpx.Validation("phone or email is required")
	}
	leadID, err := s.createLeadForBooking(ctx, tx, p, params)
	if err != nil {
		return 0, nil, err
	}
	lead, err := s.leads.GetByID(ctx, tx, leadID)
	if err != nil {
		return 0, nil, err
	}
	return leadID, lead, nil
}

func (s *Service) finalizeBooking(ctx context.Context, tx pgx.Tx, p *auth.Principal, leadID int64, lead *leads.Lead, slotStart time.Time, notifyAccountID, contractID int64, emails []notifications.EmailJob) ([]notifications.EmailJob, error) {
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
	adminIDs, err := s.accounts.AdminUserIDs(ctx, tx, notifyAccountID)
	if err != nil {
		return nil, err
	}
	payload := map[string]any{
		"lead_id":    leadID,
		"slot_start": slotStart.Format(time.RFC3339),
		"lead_name":  leadName,
	}
	if contractID > 0 {
		payload["contract_id"] = contractID
	}
	notifyEmails, err := s.notif.Deliver(ctx, tx, notifications.DeliverParams{
		AccountID: notifyAccountID,
		UserIDs:   adminIDs,
		EventType: "new_appointment",
		Payload:   payload,
	})
	if err != nil {
		return nil, err
	}
	return append(emails, notifyEmails...), nil
}

func externalProvider(params BookParams) string {
	extProvider := strings.TrimSpace(params.ExternalProvider)
	if extProvider == "" && strings.TrimSpace(params.ExternalEventID) != "" {
		return "voiceuni"
	}
	return extProvider
}
