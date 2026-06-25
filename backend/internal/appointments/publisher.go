package appointments

import (
	"context"
	"errors"
	"time"

	"github.com/echayko/leadrula/backend/pkg/httpx"
	"github.com/jackc/pgx/v5"
)

func (s *Service) ListPublisherContracts(ctx context.Context, publisherID int64) ([]AppointmentContract, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT c.id, c.name, c.buyer_id, COALESCE(b.name, ''),
		        COALESCE(bc.timezone, b.timezone, 'UTC'),
		        (bc.id IS NOT NULL AND bc.schedule::text NOT IN ('{}', 'null')
		         AND EXISTS(SELECT 1 FROM buyer_appointment_slots sl
		                    WHERE sl.calendar_id = bc.id AND sl.disabled_at IS NULL))
		 FROM contracts c
		 JOIN accounts b ON b.id = c.buyer_id
		 JOIN buyer_booking_calendars bc ON bc.id = c.appointment_calendar_id
		 WHERE c.publisher_id = $1 AND c.lead_type = 'Appointment' AND c.status = 'active'
		   AND c.deleted_at IS NULL AND c.buyer_id IS NOT NULL
		   AND c.appointment_calendar_id IS NOT NULL
		   AND bc.schedule::text NOT IN ('{}', 'null')
		   AND EXISTS(
		     SELECT 1 FROM buyer_appointment_slots sl
		     WHERE sl.calendar_id = bc.id AND sl.disabled_at IS NULL
		   )
		 ORDER BY b.name, c.name`, publisherID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []AppointmentContract
	for rows.Next() {
		var c AppointmentContract
		if err := rows.Scan(&c.ContractID, &c.ContractName, &c.BuyerID, &c.BuyerName, &c.Timezone, &c.Configured); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (s *Service) ensureContractSlots(ctx context.Context, contractID int64) error {
	_, err := s.pool.Exec(ctx,
		`INSERT INTO contract_appointment_slots(contract_id, buyer_slot_id, enabled)
		 SELECT $1, sl.id, true
		 FROM buyer_appointment_slots sl
		 JOIN contracts c ON c.id = $1
		 WHERE sl.calendar_id = c.appointment_calendar_id AND sl.disabled_at IS NULL
		 ON CONFLICT DO NOTHING`, contractID)
	return err
}

func (s *Service) ListContractSlots(ctx context.Context, publisherID, contractID int64) ([]ContractSlot, error) {
	var buyerID int64
	var leadType string
	var calendarID *int64
	err := s.pool.QueryRow(ctx,
		`SELECT buyer_id, COALESCE(lead_type,''), appointment_calendar_id FROM contracts
		 WHERE id=$1 AND publisher_id=$2 AND deleted_at IS NULL`,
		contractID, publisherID).Scan(&buyerID, &leadType, &calendarID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, httpx.NotFound("contract not found")
	}
	if err != nil {
		return nil, err
	}
	if leadType != "Appointment" {
		return nil, httpx.Validation("contract is not appointment type")
	}
	if buyerID == 0 {
		return nil, httpx.Validation("contract has no buyer")
	}
	if calendarID == nil || *calendarID == 0 {
		return nil, httpx.Validation("contract has no appointment calendar")
	}
	if err := s.ensureContractSlots(ctx, contractID); err != nil {
		return nil, err
	}
	rows, err := s.pool.Query(ctx,
		`SELECT sl.id, sl.weekday, sl.start_time::text, sl.duration_min, sl.capacity,
		        COALESCE(cs.enabled, false), cs.duration_min_override, cs.capacity_override,
		        sl.disabled_at IS NOT NULL
		 FROM buyer_appointment_slots sl
		 LEFT JOIN contract_appointment_slots cs ON cs.buyer_slot_id = sl.id AND cs.contract_id = $1
		 WHERE sl.calendar_id = $3
		 ORDER BY sl.weekday, sl.start_time`, contractID, buyerID, *calendarID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ContractSlot
	for rows.Next() {
		var cs ContractSlot
		var enabled bool
		if err := rows.Scan(&cs.BuyerSlotID, &cs.Weekday, &cs.StartTime, &cs.DurationMin, &cs.Capacity,
			&enabled, &cs.DurationMinOverride, &cs.CapacityOverride, &cs.Disabled); err != nil {
			return nil, err
		}
		cs.Enabled = enabled
		out = append(out, cs)
	}
	return out, rows.Err()
}

type PutContractSlotsParams struct {
	Slots []struct {
		BuyerSlotID         int64 `json:"buyer_slot_id"`
		Enabled             bool  `json:"enabled"`
		DurationMinOverride *int  `json:"duration_min_override"`
		CapacityOverride    *int  `json:"capacity_override"`
	} `json:"slots"`
}

func (s *Service) PutContractSlots(ctx context.Context, publisherID, contractID int64, p PutContractSlotsParams) ([]ContractSlot, error) {
	slots, err := s.ListContractSlots(ctx, publisherID, contractID)
	if err != nil {
		return nil, err
	}
	allowed := map[int64]bool{}
	for _, sl := range slots {
		allowed[sl.BuyerSlotID] = true
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	for _, item := range p.Slots {
		if !allowed[item.BuyerSlotID] {
			return nil, httpx.Validation("invalid buyer slot for contract")
		}
		if item.DurationMinOverride != nil && (*item.DurationMinOverride < minDurationMin || *item.DurationMinOverride > maxDurationMin) {
			return nil, httpx.Validation("duration_min_override out of range")
		}
		if item.CapacityOverride != nil && (*item.CapacityOverride < minCapacity || *item.CapacityOverride > maxCapacity) {
			return nil, httpx.Validation("capacity_override out of range")
		}
		_, err := tx.Exec(ctx,
			`INSERT INTO contract_appointment_slots(contract_id, buyer_slot_id, enabled, duration_min_override, capacity_override)
			 VALUES ($1,$2,$3,$4,$5)
			 ON CONFLICT (contract_id, buyer_slot_id) DO UPDATE SET
			   enabled = EXCLUDED.enabled,
			   duration_min_override = EXCLUDED.duration_min_override,
			   capacity_override = EXCLUDED.capacity_override`,
			contractID, item.BuyerSlotID, item.Enabled, item.DurationMinOverride, item.CapacityOverride)
		if err != nil {
			return nil, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return s.ListContractSlots(ctx, publisherID, contractID)
}

func (s *Service) contractBuyerID(ctx context.Context, publisherID, contractID int64) (int64, error) {
	var buyerID int64
	err := s.pool.QueryRow(ctx,
		`SELECT buyer_id FROM contracts
		 WHERE id=$1 AND publisher_id=$2 AND lead_type='Appointment' AND status='active' AND deleted_at IS NULL`,
		contractID, publisherID).Scan(&buyerID)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, httpx.NotFound("appointment contract not found")
	}
	if buyerID == 0 {
		return 0, httpx.Validation("buyer has not configured availability")
	}
	return buyerID, err
}

func (s *Service) ListFreeSlots(ctx context.Context, publisherID, contractID int64, dateStr string) ([]FreeSlot, error) {
	buyerID, err := s.contractBuyerID(ctx, publisherID, contractID)
	if err != nil {
		return nil, err
	}
	ok, err := s.contractCalendarConfigured(ctx, contractID)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, httpx.Validation("buyer has not configured availability")
	}
	calID, err := s.contractCalendarID(ctx, contractID)
	if err != nil {
		return nil, err
	}
	cal, err := s.loadCalendarByID(ctx, calID)
	if err != nil {
		return nil, err
	}
	_ = buyerID
	loc := loadLocation(cal.Timezone)
	date, err := parseDateParam(dateStr, loc)
	if err != nil {
		return nil, httpx.Validation(err.Error())
	}
	contractSlots, err := s.ListContractSlots(ctx, publisherID, contractID)
	if err != nil {
		return nil, err
	}
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
		booked, err := s.countBookings(ctx, contractID, slotStart)
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

func (s *Service) countBookings(ctx context.Context, contractID int64, slotStart time.Time) (int, error) {
	var n int
	err := s.pool.QueryRow(ctx,
		`SELECT COUNT(*)::int FROM lead_appointment_bookings
		 WHERE contract_id=$1 AND slot_start=$2`, contractID, slotStart).Scan(&n)
	return n, err
}

func (s *Service) ListCalendarMarkers(ctx context.Context, publisherID, contractID int64, fromStr, toStr string) ([]CalendarDayMarker, error) {
	if _, err := s.contractBuyerID(ctx, publisherID, contractID); err != nil {
		return nil, err
	}
	calID, err := s.contractCalendarID(ctx, contractID)
	if err != nil {
		return nil, err
	}
	cal, err := s.loadCalendarByID(ctx, calID)
	if err != nil {
		return nil, err
	}
	loc := loadLocation(cal.Timezone)
	from, err := parseDateParam(fromStr, loc)
	if err != nil {
		return nil, httpx.Validation("invalid from date")
	}
	to, err := parseDateParam(toStr, loc)
	if err != nil {
		return nil, httpx.Validation("invalid to date")
	}
	if to.Before(from) {
		return nil, httpx.Validation("to must be after from")
	}
	var out []CalendarDayMarker
	for d := from; !d.After(to); d = d.AddDate(0, 0, 1) {
		dateS := d.Format("2006-01-02")
		free, err := s.ListFreeSlots(ctx, publisherID, contractID, dateS)
		if err != nil {
			return nil, err
		}
		hasBookable := len(free) > 0
		hasBookings, err := s.dateHasBookings(ctx, contractID, d, loc)
		if err != nil {
			return nil, err
		}
		if hasBookable || hasBookings {
			out = append(out, CalendarDayMarker{
				Date:        dateS,
				HasBookable: hasBookable,
				HasBookings: hasBookings,
			})
		}
	}
	return out, nil
}

func (s *Service) dateHasBookings(ctx context.Context, contractID int64, date time.Time, loc *time.Location) (bool, error) {
	start := time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, loc)
	end := start.AddDate(0, 0, 1)
	var ok bool
	err := s.pool.QueryRow(ctx,
		`SELECT EXISTS(
			SELECT 1 FROM lead_appointment_bookings
			WHERE contract_id=$1 AND slot_start >= $2 AND slot_start < $3)`,
		contractID, start, end).Scan(&ok)
	return ok, err
}
