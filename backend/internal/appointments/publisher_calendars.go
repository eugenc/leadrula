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

func (s *Service) ListPublisherBookingCalendars(ctx context.Context, publisherID int64) ([]BookingCalendar, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT c.id, c.account_id, c.name, c.schedule, c.timezone, c.buffer_min, COALESCE(c.location, ''), c.updated_at,
		        (SELECT COUNT(*)::int FROM publisher_appointment_slots sl
		         WHERE sl.calendar_id = c.id AND sl.disabled_at IS NULL) AS slot_count
		 FROM publisher_booking_calendars c
		 WHERE c.account_id = $1
		 ORDER BY c.name, c.id`, publisherID)
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
		ok, err := s.publisherCalendarConfigured(ctx, cal.ID)
		if err != nil {
			return nil, err
		}
		cal.Configured = ok
		out = append(out, cal)
	}
	return out, rows.Err()
}

func (s *Service) GetPublisherBookingCalendar(ctx context.Context, publisherID, calendarID int64) (*BookingCalendar, error) {
	cal, err := s.loadPublisherCalendar(ctx, publisherID, calendarID)
	if err != nil {
		return nil, err
	}
	ok, err := s.publisherCalendarConfigured(ctx, calendarID)
	if err != nil {
		return nil, err
	}
	cal.Configured = ok
	return cal, nil
}

func (s *Service) CreatePublisherBookingCalendar(ctx context.Context, publisherID int64, p CreateCalendarParams) (*BookingCalendar, error) {
	name := p.Name
	if name == "" {
		return nil, httpx.Validation("name is required")
	}
	tz := p.Timezone
	if tz == "" {
		var err error
		tz, err = s.getAccountTimezone(ctx, publisherID)
		if err != nil {
			return nil, err
		}
	}
	if _, err := time.LoadLocation(tz); err != nil {
		return nil, httpx.Validation("invalid timezone")
	}
	var id int64
	err := s.pool.QueryRow(ctx,
		`INSERT INTO publisher_booking_calendars(account_id, name, schedule, timezone, buffer_min, updated_at)
		 VALUES ($1, $2, '{}', $3, 0, now())
		 RETURNING id`, publisherID, name, tz).Scan(&id)
	if err != nil {
		return nil, err
	}
	return s.GetPublisherBookingCalendar(ctx, publisherID, id)
}

func (s *Service) PutPublisherBookingCalendar(ctx context.Context, publisherID, calendarID int64, p PutCalendarParams) (*BookingCalendar, error) {
	if _, err := s.loadPublisherCalendar(ctx, publisherID, calendarID); err != nil {
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
		`UPDATE publisher_booking_calendars SET
		   name = COALESCE(NULLIF($3, ''), name),
		   schedule = $4,
		   timezone = COALESCE(NULLIF($5, ''), timezone),
		   buffer_min = $6,
		   location = COALESCE(NULLIF(TRIM($7), ''), location),
		   updated_at = now()
		 WHERE id = $1 AND account_id = $2`,
		calendarID, publisherID, name, sched, tz, p.BufferMin, p.Location)
	if err != nil {
		return nil, err
	}
	return s.GetPublisherBookingCalendar(ctx, publisherID, calendarID)
}

func (s *Service) DeletePublisherBookingCalendar(ctx context.Context, publisherID, calendarID int64) error {
	if _, err := s.loadPublisherCalendar(ctx, publisherID, calendarID); err != nil {
		return err
	}
	if msg, err := s.calendarDeleteBlocked(ctx, publisherID, calendarID, calendarSourcePublisher); err != nil {
		return err
	} else if msg != "" {
		return httpx.BusinessRule(msg)
	}
	ct, err := s.pool.Exec(ctx,
		`DELETE FROM publisher_booking_calendars WHERE id=$1 AND account_id=$2`, calendarID, publisherID)
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return httpx.NotFound("calendar not found")
	}
	return nil
}

func (s *Service) loadPublisherCalendar(ctx context.Context, publisherID, calendarID int64) (*BookingCalendar, error) {
	row := s.pool.QueryRow(ctx,
		`SELECT c.id, c.account_id, c.name, c.schedule, c.timezone, c.buffer_min, COALESCE(c.location, ''), c.updated_at,
		        (SELECT COUNT(*)::int FROM publisher_appointment_slots sl
		         WHERE sl.calendar_id = c.id AND sl.disabled_at IS NULL)
		 FROM publisher_booking_calendars c
		 WHERE c.id = $1 AND c.account_id = $2`, calendarID, publisherID)
	cal, err := scanBookingCalendarRow(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, httpx.NotFound("calendar not found")
	}
	if err != nil {
		return nil, err
	}
	return &cal, nil
}

func (s *Service) loadPublisherCalendarByID(ctx context.Context, calendarID int64) (*BookingCalendar, error) {
	row := s.pool.QueryRow(ctx,
		`SELECT c.id, c.account_id, c.name, c.schedule, c.timezone, c.buffer_min, COALESCE(c.location, ''), c.updated_at,
		        (SELECT COUNT(*)::int FROM publisher_appointment_slots sl
		         WHERE sl.calendar_id = c.id AND sl.disabled_at IS NULL)
		 FROM publisher_booking_calendars c WHERE c.id = $1`, calendarID)
	cal, err := scanBookingCalendarRow(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, httpx.NotFound("calendar not found")
	}
	if err != nil {
		return nil, err
	}
	return &cal, nil
}

func (s *Service) publisherCalendarConfigured(ctx context.Context, calendarID int64) (bool, error) {
	var ok bool
	err := s.pool.QueryRow(ctx,
		`SELECT EXISTS(
			SELECT 1 FROM publisher_booking_calendars c
			WHERE c.id = $1 AND c.schedule::text NOT IN ('{}', 'null')
		) AND EXISTS(
			SELECT 1 FROM publisher_appointment_slots s
			WHERE s.calendar_id = $1 AND s.disabled_at IS NULL
		)`, calendarID).Scan(&ok)
	return ok, err
}

func (s *Service) ListPublisherCalendarSlots(ctx context.Context, publisherID, calendarID int64) ([]PublisherSlot, error) {
	if _, err := s.loadPublisherCalendar(ctx, publisherID, calendarID); err != nil {
		return nil, err
	}
	return s.listPublisherSlotsForCalendar(ctx, calendarID)
}

func (s *Service) SetContractPublisherAppointmentCalendar(ctx context.Context, publisherID, contractID, calendarID int64) error {
	if _, err := s.loadPublisherCalendar(ctx, publisherID, calendarID); err != nil {
		return err
	}
	tag, err := s.pool.Exec(ctx,
		`UPDATE contracts SET publisher_appointment_calendar_id = $3
		 WHERE id = $1 AND publisher_id = $2 AND lead_type = 'Appointment' AND deleted_at IS NULL`,
		contractID, publisherID, calendarID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return httpx.NotFound("contract not found")
	}
	return s.ensureContractPublisherSlots(ctx, contractID)
}

func (s *Service) listPublisherSlotsForCalendar(ctx context.Context, calendarID int64) ([]PublisherSlot, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT id, account_id, calendar_id, weekday, start_time::text, duration_min, capacity, disabled_at
		 FROM publisher_appointment_slots WHERE calendar_id=$1 ORDER BY weekday, start_time`, calendarID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanPublisherSlots(rows)
}

func scanPublisherSlots(rows pgx.Rows) ([]PublisherSlot, error) {
	var out []PublisherSlot
	for rows.Next() {
		var sl PublisherSlot
		if err := rows.Scan(&sl.ID, &sl.AccountID, &sl.CalendarID, &sl.Weekday, &sl.StartTime, &sl.DurationMin, &sl.Capacity, &sl.DisabledAt); err != nil {
			return nil, err
		}
		out = append(out, sl)
	}
	return out, rows.Err()
}

func (s *Service) findPublisherSlotAtTime(ctx context.Context, calendarID int64, weekday int, startTime string) (*PublisherSlot, error) {
	var sl PublisherSlot
	err := s.pool.QueryRow(ctx,
		`SELECT id, account_id, calendar_id, weekday, start_time::text, duration_min, capacity, disabled_at
		 FROM publisher_appointment_slots
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

func (s *Service) CreatePublisherCalendarSlot(ctx context.Context, publisherID, calendarID int64, p CreateSlotParams) (*PublisherSlot, error) {
	cal, err := s.loadPublisherCalendar(ctx, publisherID, calendarID)
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
	if err := s.validatePublisherSlotOverlapCalendar(ctx, calendarID, p.Weekday, slotStart, p.DurationMin, cal.BufferMin, 0); err != nil {
		return nil, err
	}
	existing, err := s.findPublisherSlotAtTime(ctx, calendarID, p.Weekday, p.StartTime)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		if existing.DisabledAt != nil {
			reenable := false
			return s.patchPublisherSlotRecord(ctx, publisherID, calendarID, existing, PatchSlotParams{
				DurationMin: &p.DurationMin,
				Capacity:    &p.Capacity,
				Disabled:    &reenable,
			})
		}
		return nil, httpx.Conflict("slot already exists at this time")
	}
	var id int64
	err = s.pool.QueryRow(ctx,
		`INSERT INTO publisher_appointment_slots(account_id, calendar_id, weekday, start_time, duration_min, capacity)
		 VALUES ($1,$2,$3,$4::time,$5,$6)
		 RETURNING id`, publisherID, calendarID, p.Weekday, p.StartTime, p.DurationMin, p.Capacity).Scan(&id)
	if err != nil {
		if database.IsUniqueViolation(err) {
			return nil, httpx.Conflict("slot already exists at this time")
		}
		return nil, err
	}
	if err := s.syncNewPublisherSlotToContracts(ctx, publisherID, calendarID, id); err != nil {
		return nil, err
	}
	slots, err := s.listPublisherSlotsForCalendar(ctx, calendarID)
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

func (s *Service) PatchPublisherCalendarSlot(ctx context.Context, publisherID, calendarID, slotID int64, p PatchSlotParams) (*PublisherSlot, error) {
	if _, err := s.loadPublisherCalendar(ctx, publisherID, calendarID); err != nil {
		return nil, err
	}
	slots, err := s.listPublisherSlotsForCalendar(ctx, calendarID)
	if err != nil {
		return nil, err
	}
	var cur *PublisherSlot
	for i := range slots {
		if slots[i].ID == slotID {
			cur = &slots[i]
			break
		}
	}
	if cur == nil {
		return nil, httpx.NotFound("slot not found")
	}
	return s.patchPublisherSlotRecord(ctx, publisherID, calendarID, cur, p)
}

func (s *Service) patchPublisherSlotRecord(ctx context.Context, publisherID, calendarID int64, cur *PublisherSlot, p PatchSlotParams) (*PublisherSlot, error) {
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
	cal, err := s.loadPublisherCalendarByID(ctx, calendarID)
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
		if err := s.validatePublisherSlotOverlapCalendar(ctx, calendarID, cur.Weekday, slotStart, dur, cal.BufferMin, cur.ID); err != nil {
			return nil, err
		}
	}
	var execErr error
	if p.Disabled != nil {
		if *p.Disabled {
			_, execErr = s.pool.Exec(ctx,
				`UPDATE publisher_appointment_slots SET
				   start_time = COALESCE($3::time, start_time),
				   duration_min = $4, capacity = $5, disabled_at = now(), updated_at = now()
				 WHERE id=$1 AND account_id=$2 AND calendar_id=$6`,
				cur.ID, publisherID, nullStrPtr(p.StartTime), dur, cap, calendarID)
		} else {
			_, execErr = s.pool.Exec(ctx,
				`UPDATE publisher_appointment_slots SET
				   start_time = COALESCE($3::time, start_time),
				   duration_min = $4, capacity = $5, disabled_at = NULL, updated_at = now()
				 WHERE id=$1 AND account_id=$2 AND calendar_id=$6`,
				cur.ID, publisherID, nullStrPtr(p.StartTime), dur, cap, calendarID)
		}
	} else {
		_, execErr = s.pool.Exec(ctx,
			`UPDATE publisher_appointment_slots SET
			   start_time = COALESCE($3::time, start_time),
			   duration_min = $4, capacity = $5, updated_at = now()
			 WHERE id=$1 AND account_id=$2 AND calendar_id=$6`,
			cur.ID, publisherID, nullStrPtr(p.StartTime), dur, cap, calendarID)
	}
	if execErr != nil {
		return nil, execErr
	}
	slots, err := s.listPublisherSlotsForCalendar(ctx, calendarID)
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

func (s *Service) CopyPublisherCalendarSlots(ctx context.Context, publisherID, calendarID int64, fromWeekday int, toWeekdays []int) ([]PublisherSlot, error) {
	if _, err := s.loadPublisherCalendar(ctx, publisherID, calendarID); err != nil {
		return nil, err
	}
	src, err := s.listPublisherSlotsForCalendar(ctx, calendarID)
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
			_, err := s.CreatePublisherCalendarSlot(ctx, publisherID, calendarID, CreateSlotParams{
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
	return s.listPublisherSlotsForCalendar(ctx, calendarID)
}

func (s *Service) validatePublisherSlotOverlapCalendar(ctx context.Context, calendarID int64, weekday int, start time.Time, duration, buffer int, excludeID int64) error {
	slots, err := s.listPublisherSlotsForCalendar(ctx, calendarID)
	if err != nil {
		return err
	}
	cal, err := s.loadPublisherCalendarByID(ctx, calendarID)
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

func (s *Service) ListPublisherCalendarMarkers(ctx context.Context, publisherID, calendarID int64, fromStr, toStr string) ([]CalendarDayMarker, error) {
	cal, err := s.loadPublisherCalendar(ctx, publisherID, calendarID)
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
		hasBookable, err := s.publisherCalendarDayHasBookable(ctx, calendarID, d, loc)
		if err != nil {
			return nil, err
		}
		hasBookings, err := s.publisherCalendarDayHasBookings(ctx, publisherID, calendarID, d, loc)
		if err != nil {
			return nil, err
		}
		if hasBookable || hasBookings {
			out = append(out, CalendarDayMarker{Date: dateS, HasBookable: hasBookable, HasBookings: hasBookings})
		}
	}
	return out, nil
}

func (s *Service) publisherCalendarDayHasBookable(ctx context.Context, calendarID int64, date time.Time, loc *time.Location) (bool, error) {
	cal, err := s.loadPublisherCalendarByID(ctx, calendarID)
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
	slots, err := s.listPublisherSlotsForCalendar(ctx, calendarID)
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

func (s *Service) publisherCalendarDayHasBookings(ctx context.Context, publisherID, calendarID int64, date time.Time, loc *time.Location) (bool, error) {
	start := time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, loc)
	end := start.AddDate(0, 0, 1)
	var ok bool
	err := s.pool.QueryRow(ctx,
		`SELECT EXISTS(
			SELECT 1 FROM lead_appointment_bookings b
			JOIN publisher_appointment_slots sl ON sl.id = b.publisher_slot_id
			WHERE b.publisher_calendar_id = $2
			  AND b.slot_start >= $3 AND b.slot_start < $4
			UNION ALL
			SELECT 1 FROM lead_appointment_bookings b
			JOIN publisher_appointment_slots sl ON sl.id = b.publisher_slot_id
			JOIN contracts c ON c.id = b.contract_id
			WHERE c.publisher_id = $1 AND sl.calendar_id = $2
			  AND b.slot_start >= $3 AND b.slot_start < $4)`,
		publisherID, calendarID, start, end).Scan(&ok)
	return ok, err
}

