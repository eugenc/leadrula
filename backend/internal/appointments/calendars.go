package appointments

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/echayko/leadrula/backend/internal/database"
	"github.com/echayko/leadrula/backend/pkg/httpx"
	"github.com/jackc/pgx/v5"
)

type BookingCalendar struct {
	ID         int64           `json:"id"`
	AccountID  int64           `json:"account_id"`
	Name       string          `json:"name"`
	Schedule   json.RawMessage `json:"schedule"`
	Timezone   string          `json:"timezone"`
	BufferMin  int             `json:"buffer_min"`
	Configured bool            `json:"configured"`
	SlotCount  int             `json:"slot_count"`
	UpdatedAt  time.Time       `json:"updated_at"`
}

type PutCalendarParams struct {
	Name      string
	Schedule  json.RawMessage
	Timezone  string
	BufferMin int
}

type CreateCalendarParams struct {
	Name     string
	Timezone string
}

func (s *Service) ListBookingCalendars(ctx context.Context, buyerID int64) ([]BookingCalendar, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT c.id, c.account_id, c.name, c.schedule, c.timezone, c.buffer_min, c.updated_at,
		        (SELECT COUNT(*)::int FROM buyer_appointment_slots sl
		         WHERE sl.calendar_id = c.id AND sl.disabled_at IS NULL) AS slot_count
		 FROM buyer_booking_calendars c
		 WHERE c.account_id = $1
		 ORDER BY c.name, c.id`, buyerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []BookingCalendar
	for rows.Next() {
		cal, err := scanBookingCalendarRow(rows)
		if err != nil {
			return nil, err
		}
		ok, err := s.calendarConfigured(ctx, cal.ID)
		if err != nil {
			return nil, err
		}
		cal.Configured = ok
		out = append(out, cal)
	}
	return out, rows.Err()
}

func (s *Service) GetBookingCalendar(ctx context.Context, buyerID, calendarID int64) (*BookingCalendar, error) {
	cal, err := s.loadCalendar(ctx, buyerID, calendarID)
	if err != nil {
		return nil, err
	}
	ok, err := s.calendarConfigured(ctx, calendarID)
	if err != nil {
		return nil, err
	}
	cal.Configured = ok
	return cal, nil
}

func (s *Service) CreateBookingCalendar(ctx context.Context, buyerID int64, p CreateCalendarParams) (*BookingCalendar, error) {
	name := p.Name
	if name == "" {
		return nil, httpx.Validation("name is required")
	}
	tz := p.Timezone
	if tz == "" {
		var err error
		tz, err = s.getAccountTimezone(ctx, buyerID)
		if err != nil {
			return nil, err
		}
	}
	if _, err := time.LoadLocation(tz); err != nil {
		return nil, httpx.Validation("invalid timezone")
	}
	var id int64
	err := s.pool.QueryRow(ctx,
		`INSERT INTO buyer_booking_calendars(account_id, name, schedule, timezone, buffer_min, updated_at)
		 VALUES ($1, $2, '{}', $3, 0, now())
		 RETURNING id`, buyerID, name, tz).Scan(&id)
	if err != nil {
		return nil, err
	}
	return s.GetBookingCalendar(ctx, buyerID, id)
}

func (s *Service) PutBookingCalendar(ctx context.Context, buyerID, calendarID int64, p PutCalendarParams) (*BookingCalendar, error) {
	if _, err := s.loadCalendar(ctx, buyerID, calendarID); err != nil {
		return nil, err
	}
	if p.BufferMin < 0 || p.BufferMin > maxBufferMin {
		return nil, httpx.Validation("buffer_min must be between 0 and 60")
	}
	tz := p.Timezone
	if tz != "" {
		if _, err := time.LoadLocation(tz); err != nil {
			return nil, httpx.Validation("invalid timezone")
		}
	}
	sched := p.Schedule
	if len(sched) == 0 {
		sched = json.RawMessage(`{}`)
	}
	name := p.Name
	_, err := s.pool.Exec(ctx,
		`UPDATE buyer_booking_calendars SET
		   name = COALESCE(NULLIF($3, ''), name),
		   schedule = $4,
		   timezone = COALESCE(NULLIF($5, ''), timezone),
		   buffer_min = $6,
		   updated_at = now()
		 WHERE id = $1 AND account_id = $2`,
		calendarID, buyerID, name, sched, tz, p.BufferMin)
	if err != nil {
		return nil, err
	}
	return s.GetBookingCalendar(ctx, buyerID, calendarID)
}

func (s *Service) loadCalendar(ctx context.Context, buyerID, calendarID int64) (*BookingCalendar, error) {
	row := s.pool.QueryRow(ctx,
		`SELECT c.id, c.account_id, c.name, c.schedule, c.timezone, c.buffer_min, c.updated_at,
		        (SELECT COUNT(*)::int FROM buyer_appointment_slots sl
		         WHERE sl.calendar_id = c.id AND sl.disabled_at IS NULL)
		 FROM buyer_booking_calendars c
		 WHERE c.id = $1 AND c.account_id = $2`, calendarID, buyerID)
	cal, err := scanBookingCalendarRow(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, httpx.NotFound("calendar not found")
	}
	if err != nil {
		return nil, err
	}
	return &cal, nil
}

func (s *Service) loadCalendarByID(ctx context.Context, calendarID int64) (*BookingCalendar, error) {
	row := s.pool.QueryRow(ctx,
		`SELECT c.id, c.account_id, c.name, c.schedule, c.timezone, c.buffer_min, c.updated_at,
		        (SELECT COUNT(*)::int FROM buyer_appointment_slots sl
		         WHERE sl.calendar_id = c.id AND sl.disabled_at IS NULL)
		 FROM buyer_booking_calendars c WHERE c.id = $1`, calendarID)
	cal, err := scanBookingCalendarRow(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, httpx.NotFound("calendar not found")
	}
	if err != nil {
		return nil, err
	}
	return &cal, nil
}

type calendarRowScanner interface {
	Scan(dest ...any) error
}

func scanBookingCalendarRow(row calendarRowScanner) (BookingCalendar, error) {
	var cal BookingCalendar
	err := row.Scan(&cal.ID, &cal.AccountID, &cal.Name, &cal.Schedule, &cal.Timezone, &cal.BufferMin, &cal.UpdatedAt, &cal.SlotCount)
	return cal, err
}

func (s *Service) calendarConfigured(ctx context.Context, calendarID int64) (bool, error) {
	var ok bool
	err := s.pool.QueryRow(ctx,
		`SELECT EXISTS(
			SELECT 1 FROM buyer_booking_calendars c
			WHERE c.id = $1 AND c.schedule::text NOT IN ('{}', 'null')
		) AND EXISTS(
			SELECT 1 FROM buyer_appointment_slots s
			WHERE s.calendar_id = $1 AND s.disabled_at IS NULL
		)`, calendarID).Scan(&ok)
	return ok, err
}

func (s *Service) contractCalendarID(ctx context.Context, contractID int64) (int64, error) {
	var calID *int64
	err := s.pool.QueryRow(ctx,
		`SELECT appointment_calendar_id FROM contracts WHERE id = $1`, contractID).Scan(&calID)
	if err != nil {
		return 0, err
	}
	if calID == nil || *calID == 0 {
		return 0, httpx.Validation("contract has no appointment calendar")
	}
	return *calID, nil
}

func (s *Service) SetContractAppointmentCalendar(ctx context.Context, buyerID, contractID, calendarID int64) error {
	if _, err := s.loadCalendar(ctx, buyerID, calendarID); err != nil {
		return err
	}
	tag, err := s.pool.Exec(ctx,
		`UPDATE contracts SET appointment_calendar_id = $3
		 WHERE id = $1 AND buyer_id = $2 AND lead_type = 'Appointment' AND deleted_at IS NULL`,
		contractID, buyerID, calendarID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return httpx.NotFound("contract not found")
	}
	return nil
}

func (s *Service) firstCalendarID(ctx context.Context, buyerID int64) (int64, error) {
	var id int64
	err := s.pool.QueryRow(ctx,
		`SELECT id FROM buyer_booking_calendars WHERE account_id=$1 ORDER BY id LIMIT 1`, buyerID).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, nil
	}
	return id, err
}

func (s *Service) defaultCalendarID(ctx context.Context, buyerID int64) (int64, error) {
	id, err := s.firstCalendarID(ctx, buyerID)
	if err != nil {
		return 0, err
	}
	if id != 0 {
		return id, nil
	}
	cal, err := s.CreateBookingCalendar(ctx, buyerID, CreateCalendarParams{Name: "Default"})
	if err != nil {
		return 0, err
	}
	return cal.ID, nil
}

func (s *Service) ListCalendarSlots(ctx context.Context, buyerID, calendarID int64) ([]BuyerSlot, error) {
	if _, err := s.loadCalendar(ctx, buyerID, calendarID); err != nil {
		return nil, err
	}
	return s.listSlotsForCalendar(ctx, calendarID)
}

func (s *Service) listSlotsForCalendar(ctx context.Context, calendarID int64) ([]BuyerSlot, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT id, account_id, calendar_id, weekday, start_time::text, duration_min, capacity, disabled_at
		 FROM buyer_appointment_slots WHERE calendar_id=$1 ORDER BY weekday, start_time`, calendarID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanBuyerSlots(rows)
}

func scanBuyerSlots(rows pgx.Rows) ([]BuyerSlot, error) {
	var out []BuyerSlot
	for rows.Next() {
		var sl BuyerSlot
		if err := rows.Scan(&sl.ID, &sl.AccountID, &sl.CalendarID, &sl.Weekday, &sl.StartTime, &sl.DurationMin, &sl.Capacity, &sl.DisabledAt); err != nil {
			return nil, err
		}
		out = append(out, sl)
	}
	return out, rows.Err()
}

func (s *Service) findSlotAtTime(ctx context.Context, calendarID int64, weekday int, startTime string) (*BuyerSlot, error) {
	var sl BuyerSlot
	err := s.pool.QueryRow(ctx,
		`SELECT id, account_id, calendar_id, weekday, start_time::text, duration_min, capacity, disabled_at
		 FROM buyer_appointment_slots
		 WHERE calendar_id=$1 AND weekday=$2 AND start_time=$3::time`,
		calendarID, weekday, startTime).Scan(
		&sl.ID, &sl.AccountID, &sl.CalendarID, &sl.Weekday, &sl.StartTime, &sl.DurationMin, &sl.Capacity, &sl.DisabledAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &sl, nil
}

func (s *Service) CreateCalendarSlot(ctx context.Context, buyerID, calendarID int64, p CreateSlotParams) (*BuyerSlot, error) {
	cal, err := s.loadCalendar(ctx, buyerID, calendarID)
	if err != nil {
		return nil, err
	}
	if p.DurationMin < minDurationMin || p.DurationMin > maxDurationMin {
		return nil, httpx.Validation("duration_min must be between 15 and 180")
	}
	if p.Capacity < minCapacity || p.Capacity > maxCapacity {
		return nil, httpx.Validation("capacity must be between 1 and 20")
	}
	if p.Weekday < 0 || p.Weekday > 6 {
		return nil, httpx.Validation("weekday must be 0-6")
	}
	sched := WeeklySchedule{}
	if len(cal.Schedule) > 0 {
		_ = json.Unmarshal(cal.Schedule, &sched)
	}
	loc := loadLocation(cal.Timezone)
	ref := timeNowWeekdayRef(p.Weekday, loc)
	slotStart, err := combineDateAndTime(ref, p.StartTime, loc)
	if err != nil {
		return nil, httpx.Validation(err.Error())
	}
	if !slotInsideWorkingHours(sched, time.Weekday(p.Weekday), slotStart, p.DurationMin) {
		return nil, httpx.Validation("slot must fall within working hours")
	}
	if err := s.validateSlotOverlapCalendar(ctx, calendarID, p.Weekday, slotStart, p.DurationMin, cal.BufferMin, 0); err != nil {
		return nil, err
	}
	existing, err := s.findSlotAtTime(ctx, calendarID, p.Weekday, p.StartTime)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		if existing.DisabledAt != nil {
			reenable := false
			return s.patchSlotRecord(ctx, buyerID, calendarID, existing, PatchSlotParams{
				DurationMin: &p.DurationMin,
				Capacity:    &p.Capacity,
				Disabled:    &reenable,
			})
		}
		return nil, httpx.Conflict("slot already exists at this time")
	}
	var id int64
	err = s.pool.QueryRow(ctx,
		`INSERT INTO buyer_appointment_slots(account_id, calendar_id, weekday, start_time, duration_min, capacity)
		 VALUES ($1,$2,$3,$4::time,$5,$6)
		 RETURNING id`, buyerID, calendarID, p.Weekday, p.StartTime, p.DurationMin, p.Capacity).Scan(&id)
	if err != nil {
		if database.IsUniqueViolation(err) {
			return nil, httpx.Conflict("slot already exists at this time")
		}
		return nil, err
	}
	if err := s.syncNewSlotToContracts(ctx, buyerID, calendarID, id); err != nil {
		return nil, err
	}
	slots, err := s.listSlotsForCalendar(ctx, calendarID)
	if err != nil {
		return nil, err
	}
	for _, sl := range slots {
		if sl.ID == id {
			return &sl, nil
		}
	}
	return nil, httpx.NotFound("slot not found")
}

func (s *Service) PatchCalendarSlot(ctx context.Context, buyerID, calendarID, slotID int64, p PatchSlotParams) (*BuyerSlot, error) {
	if _, err := s.loadCalendar(ctx, buyerID, calendarID); err != nil {
		return nil, err
	}
	slots, err := s.listSlotsForCalendar(ctx, calendarID)
	if err != nil {
		return nil, err
	}
	var cur *BuyerSlot
	for i := range slots {
		if slots[i].ID == slotID {
			cur = &slots[i]
			break
		}
	}
	if cur == nil {
		return nil, httpx.NotFound("slot not found")
	}
	return s.patchSlotRecord(ctx, buyerID, calendarID, cur, p)
}

func (s *Service) patchSlotRecord(ctx context.Context, buyerID, calendarID int64, cur *BuyerSlot, p PatchSlotParams) (*BuyerSlot, error) {
	dur := cur.DurationMin
	if p.DurationMin != nil {
		dur = *p.DurationMin
	}
	cap := cur.Capacity
	if p.Capacity != nil {
		cap = *p.Capacity
	}
	start := cur.StartTime
	if p.StartTime != nil {
		start = *p.StartTime
	}
	if dur < minDurationMin || dur > maxDurationMin {
		return nil, httpx.Validation("duration_min must be between 15 and 180")
	}
	if cap < minCapacity || cap > maxCapacity {
		return nil, httpx.Validation("capacity must be between 1 and 20")
	}
	cal, err := s.loadCalendarByID(ctx, calendarID)
	if err != nil {
		return nil, err
	}
	if p.StartTime != nil || p.DurationMin != nil {
		sched := WeeklySchedule{}
		_ = json.Unmarshal(cal.Schedule, &sched)
		loc := loadLocation(cal.Timezone)
		ref := timeNowWeekdayRef(cur.Weekday, loc)
		slotStart, err := combineDateAndTime(ref, start, loc)
		if err != nil {
			return nil, httpx.Validation(err.Error())
		}
		if !slotInsideWorkingHours(sched, time.Weekday(cur.Weekday), slotStart, dur) {
			return nil, httpx.Validation("slot must fall within working hours")
		}
		if err := s.validateSlotOverlapCalendar(ctx, calendarID, cur.Weekday, slotStart, dur, cal.BufferMin, cur.ID); err != nil {
			return nil, err
		}
	}
	var execErr error
	if p.Disabled != nil {
		if *p.Disabled {
			_, execErr = s.pool.Exec(ctx,
				`UPDATE buyer_appointment_slots SET
				   start_time = COALESCE($3::time, start_time),
				   duration_min = $4, capacity = $5, disabled_at = now(), updated_at = now()
				 WHERE id=$1 AND account_id=$2 AND calendar_id=$6`,
				cur.ID, buyerID, nullStrPtr(p.StartTime), dur, cap, calendarID)
		} else {
			_, execErr = s.pool.Exec(ctx,
				`UPDATE buyer_appointment_slots SET
				   start_time = COALESCE($3::time, start_time),
				   duration_min = $4, capacity = $5, disabled_at = NULL, updated_at = now()
				 WHERE id=$1 AND account_id=$2 AND calendar_id=$6`,
				cur.ID, buyerID, nullStrPtr(p.StartTime), dur, cap, calendarID)
		}
	} else {
		_, execErr = s.pool.Exec(ctx,
			`UPDATE buyer_appointment_slots SET
			   start_time = COALESCE($3::time, start_time),
			   duration_min = $4, capacity = $5, updated_at = now()
			 WHERE id=$1 AND account_id=$2 AND calendar_id=$6`,
			cur.ID, buyerID, nullStrPtr(p.StartTime), dur, cap, calendarID)
	}
	if execErr != nil {
		return nil, execErr
	}
	slots, err := s.listSlotsForCalendar(ctx, calendarID)
	if err != nil {
		return nil, err
	}
	for _, sl := range slots {
		if sl.ID == cur.ID {
			return &sl, nil
		}
	}
	return nil, httpx.NotFound("slot not found")
}

func (s *Service) CopyCalendarSlots(ctx context.Context, buyerID, calendarID int64, fromWeekday int, toWeekdays []int) ([]BuyerSlot, error) {
	if _, err := s.loadCalendar(ctx, buyerID, calendarID); err != nil {
		return nil, err
	}
	src, err := s.listSlotsForCalendar(ctx, calendarID)
	if err != nil {
		return nil, err
	}
	for _, d := range toWeekdays {
		if d < 0 || d > 6 {
			return nil, httpx.Validation("invalid weekday")
		}
		if d == fromWeekday {
			continue
		}
		for _, sl := range src {
			if sl.Weekday != fromWeekday || sl.DisabledAt != nil {
				continue
			}
			_, err := s.CreateCalendarSlot(ctx, buyerID, calendarID, CreateSlotParams{
				Weekday:     d,
				StartTime:   sl.StartTime,
				DurationMin: sl.DurationMin,
				Capacity:    sl.Capacity,
			})
			if err != nil {
				if database.IsUniqueViolation(err) {
					continue
				}
				return nil, err
			}
		}
	}
	return s.listSlotsForCalendar(ctx, calendarID)
}

func (s *Service) validateSlotOverlapCalendar(ctx context.Context, calendarID int64, weekday int, start time.Time, duration, buffer int, excludeID int64) error {
	slots, err := s.listSlotsForCalendar(ctx, calendarID)
	if err != nil {
		return err
	}
	cal, err := s.loadCalendarByID(ctx, calendarID)
	if err != nil {
		return err
	}
	candidate := timeInterval{start: start, duration: duration, buffer: buffer}
	var existing []timeInterval
	loc := loadLocation(cal.Timezone)
	for _, sl := range slots {
		if sl.ID == excludeID || sl.DisabledAt != nil || sl.Weekday != weekday {
			continue
		}
		st, err := combineDateAndTime(timeNowWeekdayRef(weekday, loc), sl.StartTime, loc)
		if err != nil {
			continue
		}
		existing = append(existing, timeInterval{start: st, duration: sl.DurationMin, buffer: buffer})
	}
	if err := validateNoOverlap(existing, candidate); err != nil {
		return httpx.Validation(err.Error())
	}
	return nil
}

func (s *Service) ListBuyerCalendarMarkers(ctx context.Context, buyerID, calendarID int64, fromStr, toStr string) ([]CalendarDayMarker, error) {
	cal, err := s.loadCalendar(ctx, buyerID, calendarID)
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
		hasBookable, err := s.calendarDayHasBookable(ctx, calendarID, d, loc)
		if err != nil {
			return nil, err
		}
		hasBookings, err := s.calendarDayHasBookings(ctx, buyerID, calendarID, d, loc)
		if err != nil {
			return nil, err
		}
		if hasBookable || hasBookings {
			out = append(out, CalendarDayMarker{Date: dateS, HasBookable: hasBookable, HasBookings: hasBookings})
		}
	}
	return out, nil
}

func (s *Service) calendarDayHasBookable(ctx context.Context, calendarID int64, date time.Time, loc *time.Location) (bool, error) {
	cal, err := s.loadCalendarByID(ctx, calendarID)
	if err != nil {
		return false, err
	}
	sched := WeeklySchedule{}
	_ = json.Unmarshal(cal.Schedule, &sched)
	weekday := int(date.Weekday())
	w, ok := sched.dayWindow(time.Weekday(weekday))
	if !ok {
		return false, nil
	}
	slots, err := s.listSlotsForCalendar(ctx, calendarID)
	if err != nil {
		return false, err
	}
	now := time.Now()
	for _, sl := range slots {
		if sl.DisabledAt != nil || sl.Weekday != weekday {
			continue
		}
		slotStart, err := combineDateAndTime(date, sl.StartTime, loc)
		if err != nil {
			continue
		}
		if !slotInsideWorkingHours(sched, time.Weekday(weekday), slotStart, sl.DurationMin) {
			continue
		}
		if !bookingWindowOK(slotStart, now) {
			continue
		}
		_ = w
		return true, nil
	}
	return false, nil
}

func (s *Service) calendarDayHasBookings(ctx context.Context, buyerID, calendarID int64, date time.Time, loc *time.Location) (bool, error) {
	start := time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, loc)
	end := start.AddDate(0, 0, 1)
	var ok bool
	err := s.pool.QueryRow(ctx,
		`SELECT EXISTS(
			SELECT 1 FROM lead_appointment_bookings b
			JOIN buyer_appointment_slots sl ON sl.id = b.buyer_slot_id
			JOIN contracts c ON c.id = b.contract_id
			WHERE c.buyer_id = $1 AND sl.calendar_id = $2
			  AND b.slot_start >= $3 AND b.slot_start < $4)`,
		buyerID, calendarID, start, end).Scan(&ok)
	return ok, err
}

func (s *Service) ListBuyerBookingsForCalendar(ctx context.Context, buyerID, calendarID int64) ([]BookingRow, error) {
	if _, err := s.loadCalendar(ctx, buyerID, calendarID); err != nil {
		return nil, err
	}
	return s.listBookingsQuery(ctx,
		`l.owner_account_id = $1 AND sl.calendar_id = $2`, []any{buyerID, calendarID}, "b.created_at DESC", 0)
}
