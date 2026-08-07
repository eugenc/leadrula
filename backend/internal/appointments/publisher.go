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
		        CASE
		          WHEN pc.id IS NOT NULL THEN COALESCE(pc.timezone, p.timezone, 'UTC')
		          WHEN bc.id IS NOT NULL THEN COALESCE(bc.timezone, b.timezone, 'UTC')
		          ELSE COALESCE(b.timezone, 'UTC')
		        END,
		        COALESCE(pc.location, ''),
		        CASE WHEN pc.id IS NOT NULL THEN 'publisher' WHEN bc.id IS NOT NULL THEN 'buyer' ELSE '' END,
		        (pc.id IS NOT NULL AND pc.schedule::text NOT IN ('{}', 'null')
		         AND EXISTS(SELECT 1 FROM publisher_appointment_slots sl
		                    WHERE sl.calendar_id = pc.id AND sl.disabled_at IS NULL)),
		        (c.status = 'active' AND c.buyer_id IS NOT NULL
		         AND bc.id IS NOT NULL AND bc.schedule::text NOT IN ('{}', 'null')
		         AND EXISTS(SELECT 1 FROM buyer_appointment_slots sl
		                    WHERE sl.calendar_id = bc.id AND sl.disabled_at IS NULL))
		 FROM contracts c
		 JOIN accounts b ON b.id = c.buyer_id
		 JOIN accounts p ON p.id = c.publisher_id
		 LEFT JOIN buyer_booking_calendars bc ON bc.id = c.appointment_calendar_id
		 LEFT JOIN publisher_booking_calendars pc ON pc.id = c.publisher_appointment_calendar_id
		 WHERE c.publisher_id = $1 AND c.lead_type = 'Appointment' AND c.status = 'active'
		   AND c.deleted_at IS NULL AND c.buyer_id IS NOT NULL
		   AND (pc.id IS NOT NULL OR bc.id IS NOT NULL)
		 ORDER BY b.name, c.name`, publisherID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []AppointmentContract
	for rows.Next() {
		var c AppointmentContract
		if err := rows.Scan(&c.ContractID, &c.ContractName, &c.BuyerID, &c.BuyerName,
			&c.Timezone, &c.Location, &c.CalendarSource, &c.OwnConfigured, &c.CounterpartyConfigured); err != nil {
			return nil, err
		}
		c.Configured = c.OwnConfigured || c.CounterpartyConfigured
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
	err := s.pool.QueryRow(ctx,
		`SELECT buyer_id, COALESCE(lead_type,'') FROM contracts
		 WHERE id=$1 AND publisher_id=$2 AND deleted_at IS NULL`,
		contractID, publisherID).Scan(&buyerID, &leadType)
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
	ok, err := s.contractAccepted(ctx, contractID)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, httpx.Validation("contract is not accepted yet")
	}
	active, err := s.resolveCrossBookingCalendar(ctx, contractID, false)
	if err != nil {
		return nil, err
	}
	if active.Source != calendarSourceBuyer {
		return nil, httpx.Validation("contract has no buyer appointment calendar")
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
		 WHERE sl.calendar_id = $2
		 ORDER BY sl.weekday, sl.start_time`, contractID, active.CalendarID)
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

func (s *Service) ListFreeSlots(ctx context.Context, publisherID, contractID int64, dateStr, target string) ([]FreeSlot, error) {
	return s.listFreeSlots(ctx, publisherID, contractID, dateStr, false, target)
}

func (s *Service) listFreeSlots(ctx context.Context, accountID, contractID int64, dateStr string, asBuyer bool, target string) ([]FreeSlot, error) {
	if asBuyer {
		if err := s.contractOwnedByBuyer(ctx, accountID, contractID); err != nil {
			return nil, err
		}
	} else if _, err := s.contractBuyerID(ctx, accountID, contractID); err != nil {
		return nil, err
	}
	ok, err := s.bookingCalendarConfigured(ctx, contractID, asBuyer, target)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, httpx.Validation("appointment calendar is not configured")
	}
	active, err := s.resolveBookingCalendar(ctx, contractID, asBuyer, target)
	if err != nil {
		return nil, err
	}
	switch active.Source {
	case calendarSourceBuyer:
		return s.listFreeBuyerCalendarSlots(ctx, accountID, contractID, dateStr, asBuyer, target, active)
	case calendarSourcePublisher:
		return s.listFreePublisherCalendarSlots(ctx, accountID, contractID, dateStr, asBuyer, target, active)
	default:
		return nil, httpx.Validation("appointment calendar is not configured")
	}
}

func (s *Service) listFreeBuyerCalendarSlots(ctx context.Context, accountID, contractID int64, dateStr string, asBuyer bool, target string, active activeCalendar) ([]FreeSlot, error) {
	cal, err := s.loadCalendarByID(ctx, active.CalendarID)
	if err != nil {
		return nil, err
	}
	loc := loadLocation(cal.Timezone)
	date, err := parseDateParam(dateStr, loc)
	if err != nil {
		return nil, httpx.Validation(err.Error())
	}
	var contractSlots []ContractSlot
	switch target {
	case bookingTargetActive:
		contractSlots, err = s.listActiveContractBuyerSlots(ctx, contractID)
	case bookingTargetCross:
		if asBuyer {
			return nil, httpx.Validation("buyer cross-booking uses publisher calendar")
		}
		contractSlots, err = s.ListContractSlots(ctx, accountID, contractID)
	default:
		contractSlots, err = s.listOwnBuyerCalendarSlots(ctx, accountID, contractID, asBuyer)
	}
	if err != nil {
		return nil, err
	}
	return s.buildBuyerFreeSlots(ctx, contractSlots, date, loc)
}

func (s *Service) listFreePublisherCalendarSlots(ctx context.Context, accountID, contractID int64, dateStr string, asBuyer bool, target string, active activeCalendar) ([]FreeSlot, error) {
	cal, err := s.loadPublisherCalendarByID(ctx, active.CalendarID)
	if err != nil {
		return nil, err
	}
	loc := loadLocation(cal.Timezone)
	date, err := parseDateParam(dateStr, loc)
	if err != nil {
		return nil, httpx.Validation(err.Error())
	}
	var contractSlots []ContractPublisherSlot
	switch target {
	case bookingTargetActive:
		contractSlots, err = s.listActiveContractPublisherSlots(ctx, contractID)
	case bookingTargetCross:
		if asBuyer {
			contractSlots, err = s.ListContractPublisherSlotsForBuyer(ctx, accountID, contractID)
		} else {
			return nil, httpx.Validation("publisher cross-booking uses buyer calendar")
		}
	default:
		contractSlots, err = s.listOwnPublisherCalendarSlots(ctx, accountID, contractID, asBuyer)
	}
	if err != nil {
		return nil, err
	}
	return s.buildPublisherFreeSlots(ctx, contractID, contractSlots, date, loc)
}

func (s *Service) listActiveContractBuyerSlots(ctx context.Context, contractID int64) ([]ContractSlot, error) {
	row, err := s.loadContractCalendarRow(ctx, contractID)
	if err != nil {
		return nil, err
	}
	if row.Source == nil || *row.Source != calendarSourceBuyer {
		return nil, httpx.Validation("contract is not using buyer calendar")
	}
	if row.BuyerCalendarID == nil || *row.BuyerCalendarID == 0 {
		return nil, httpx.Validation("contract has no buyer appointment calendar")
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
		 WHERE sl.calendar_id = $2
		 ORDER BY sl.weekday, sl.start_time`, contractID, *row.BuyerCalendarID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanContractSlots(rows)
}

func (s *Service) listActiveContractPublisherSlots(ctx context.Context, contractID int64) ([]ContractPublisherSlot, error) {
	row, err := s.loadContractCalendarRow(ctx, contractID)
	if err != nil {
		return nil, err
	}
	if row.Source == nil || *row.Source != calendarSourcePublisher {
		return nil, httpx.Validation("contract is not using publisher calendar")
	}
	if row.PublisherCalendarID == nil || *row.PublisherCalendarID == 0 {
		return nil, httpx.Validation("contract has no publisher appointment calendar")
	}
	if err := s.ensureContractPublisherSlots(ctx, contractID); err != nil {
		return nil, err
	}
	rows, err := s.pool.Query(ctx,
		`SELECT sl.id, sl.weekday, sl.start_time::text, sl.duration_min, sl.capacity,
		        COALESCE(cs.enabled, false), cs.duration_min_override, cs.capacity_override,
		        sl.disabled_at IS NOT NULL
		 FROM publisher_appointment_slots sl
		 LEFT JOIN contract_publisher_appointment_slots cs ON cs.publisher_slot_id = sl.id AND cs.contract_id = $1
		 WHERE sl.calendar_id = $2
		 ORDER BY sl.weekday, sl.start_time`, contractID, *row.PublisherCalendarID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanContractPublisherSlots(rows)
}

func (s *Service) listOwnBuyerCalendarSlots(ctx context.Context, accountID, contractID int64, asBuyer bool) ([]ContractSlot, error) {
	var calID int64
	var err error
	if asBuyer {
		err = s.pool.QueryRow(ctx,
			`SELECT appointment_calendar_id FROM contracts
			 WHERE id=$1 AND buyer_id=$2 AND deleted_at IS NULL`,
			contractID, accountID).Scan(&calID)
	} else {
		err = s.pool.QueryRow(ctx,
			`SELECT appointment_calendar_id FROM contracts
			 WHERE id=$1 AND publisher_id=$2 AND deleted_at IS NULL`,
			contractID, accountID).Scan(&calID)
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, httpx.NotFound("contract not found")
	}
	if err != nil {
		return nil, err
	}
	if calID == 0 {
		return nil, httpx.Validation("contract has no buyer appointment calendar")
	}
	rows, err := s.pool.Query(ctx,
		`SELECT sl.id, sl.weekday, sl.start_time::text, sl.duration_min, sl.capacity,
		        true, NULL::int, NULL::int, sl.disabled_at IS NOT NULL
		 FROM buyer_appointment_slots sl
		 WHERE sl.calendar_id = $1
		 ORDER BY sl.weekday, sl.start_time`, calID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanContractSlots(rows)
}


func scanContractSlots(rows pgx.Rows) ([]ContractSlot, error) {
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

func (s *Service) listOwnPublisherCalendarSlots(ctx context.Context, accountID, contractID int64, asBuyer bool) ([]ContractPublisherSlot, error) {
	var calID int64
	var err error
	if asBuyer {
		err = s.pool.QueryRow(ctx,
			`SELECT publisher_appointment_calendar_id FROM contracts
			 WHERE id=$1 AND buyer_id=$2 AND deleted_at IS NULL`,
			contractID, accountID).Scan(&calID)
	} else {
		err = s.pool.QueryRow(ctx,
			`SELECT publisher_appointment_calendar_id FROM contracts
			 WHERE id=$1 AND publisher_id=$2 AND deleted_at IS NULL`,
			contractID, accountID).Scan(&calID)
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, httpx.NotFound("contract not found")
	}
	if err != nil {
		return nil, err
	}
	if calID == 0 {
		return nil, httpx.Validation("contract has no publisher appointment calendar")
	}
	if err := s.ensureContractPublisherSlots(ctx, contractID); err != nil {
		return nil, err
	}
	rows, err := s.pool.Query(ctx,
		`SELECT sl.id, sl.weekday, sl.start_time::text, sl.duration_min, sl.capacity,
		        true, NULL::int, NULL::int, sl.disabled_at IS NOT NULL
		 FROM publisher_appointment_slots sl
		 WHERE sl.calendar_id = $1
		 ORDER BY sl.weekday, sl.start_time`, calID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanContractPublisherSlots(rows)
}


func scanContractPublisherSlots(rows pgx.Rows) ([]ContractPublisherSlot, error) {
	var out []ContractPublisherSlot
	for rows.Next() {
		var cs ContractPublisherSlot
		var enabled bool
		if err := rows.Scan(&cs.PublisherSlotID, &cs.Weekday, &cs.StartTime, &cs.DurationMin, &cs.Capacity,
			&enabled, &cs.DurationMinOverride, &cs.CapacityOverride, &cs.Disabled); err != nil {
			return nil, err
		}
		cs.Enabled = enabled
		out = append(out, cs)
	}
	return out, rows.Err()
}

func (s *Service) ListCalendarMarkers(ctx context.Context, publisherID, contractID int64, fromStr, toStr, target string) ([]CalendarDayMarker, error) {
	return s.listCalendarMarkers(ctx, publisherID, contractID, fromStr, toStr, false, target)
}

func (s *Service) buildBuyerFreeSlots(ctx context.Context, contractSlots []ContractSlot, date time.Time, loc *time.Location) ([]FreeSlot, error) {
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
		booked, err := s.countBuyerSlotOccupancy(ctx, cs.BuyerSlotID, slotStart)
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

func (s *Service) buildPublisherFreeSlots(ctx context.Context, contractID int64, contractSlots []ContractPublisherSlot, date time.Time, loc *time.Location) ([]FreeSlot, error) {
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
		booked, err := s.countPublisherSlotOccupancy(ctx, contractID, cs.PublisherSlotID, slotStart)
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

func (s *Service) countBuyerSlotOccupancy(ctx context.Context, buyerSlotID int64, slotStart time.Time) (int, error) {
	var n int
	err := s.pool.QueryRow(ctx, buyerSlotOccupancySQL, buyerSlotID, slotStart).Scan(&n)
	return n, err
}

func (s *Service) countPublisherSlotOccupancy(ctx context.Context, contractID, publisherSlotID int64, slotStart time.Time) (int, error) {
	var n int
	err := s.pool.QueryRow(ctx, publisherSlotOccupancySQL, contractID, publisherSlotID, slotStart).Scan(&n)
	return n, err
}

const buyerSlotOccupancySQL = `
SELECT COUNT(*)::int FROM (
  SELECT b.lead_id
  FROM lead_appointment_bookings b
  WHERE b.buyer_slot_id = $1 AND b.slot_start = $2
  UNION
  SELECT l.id
  FROM leads l
  JOIN contracts c ON c.id = l.contract_id
  JOIN buyer_appointment_slots sl ON sl.id = $1
  WHERE c.appointment_calendar_id = sl.calendar_id
    AND c.appointment_calendar_source = 'buyer'
    AND l.action_at = $2
    AND l.deleted_at IS NULL
    AND NOT EXISTS (
      SELECT 1 FROM lead_appointment_bookings b2 WHERE b2.lead_id = l.id
    )
) occupied`

const publisherSlotOccupancySQL = `
SELECT COUNT(*)::int FROM (
  SELECT b.lead_id
  FROM lead_appointment_bookings b
  WHERE b.contract_id = $1 AND b.publisher_slot_id = $2 AND b.slot_start = $3
  UNION
  SELECT l.id
  FROM leads l
  JOIN contracts c ON c.id = l.contract_id
  JOIN publisher_appointment_slots sl ON sl.id = $2
  WHERE c.id = $1
    AND c.appointment_calendar_source = 'publisher'
    AND c.publisher_appointment_calendar_id = sl.calendar_id
    AND l.action_at = $3
    AND l.deleted_at IS NULL
    AND NOT EXISTS (
      SELECT 1 FROM lead_appointment_bookings b2 WHERE b2.lead_id = l.id
    )
) occupied`

func (s *Service) listCalendarMarkers(ctx context.Context, accountID, contractID int64, fromStr, toStr string, asBuyer bool, target string) ([]CalendarDayMarker, error) {
	if asBuyer {
		if err := s.contractOwnedByBuyer(ctx, accountID, contractID); err != nil {
			return nil, err
		}
	} else if _, err := s.contractBuyerID(ctx, accountID, contractID); err != nil {
		return nil, err
	}
	ok, err := s.bookingCalendarConfigured(ctx, contractID, asBuyer, target)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, httpx.Validation("appointment calendar is not configured")
	}
	active, err := s.resolveBookingCalendar(ctx, contractID, asBuyer, target)
	if err != nil {
		return nil, err
	}
	var loc *time.Location
	switch active.Source {
	case calendarSourceBuyer:
		cal, err := s.loadCalendarByID(ctx, active.CalendarID)
		if err != nil {
			return nil, err
		}
		loc = loadLocation(cal.Timezone)
	case calendarSourcePublisher:
		cal, err := s.loadPublisherCalendarByID(ctx, active.CalendarID)
		if err != nil {
			return nil, err
		}
		loc = loadLocation(cal.Timezone)
	default:
		return nil, httpx.Validation("appointment calendar is not configured")
	}
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
		var free []FreeSlot
		if asBuyer {
			free, err = s.ListFreeSlotsForBuyer(ctx, accountID, contractID, dateS, target)
		} else {
			free, err = s.ListFreeSlots(ctx, accountID, contractID, dateS, target)
		}
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
