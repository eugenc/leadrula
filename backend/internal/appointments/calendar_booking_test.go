package appointments

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/echayko/leadrula/backend/pkg/httpx"
)

func TestOwnCalendarConfigured_publisherCalendarWithoutSource(t *testing.T) {
	pool := connectAppointmentsTestDB(t)
	ctx := context.Background()

	var contractID int64
	err := pool.QueryRow(ctx,
		`SELECT c.id
		 FROM contracts c
		 JOIN publisher_booking_calendars pc ON pc.id = c.publisher_appointment_calendar_id
		 WHERE c.lead_type = 'Appointment' AND c.status = 'active' AND c.deleted_at IS NULL
		   AND c.publisher_appointment_calendar_id IS NOT NULL
		   AND pc.schedule::text NOT IN ('{}', 'null')
		   AND EXISTS(
		     SELECT 1 FROM publisher_appointment_slots sl
		     WHERE sl.calendar_id = pc.id AND sl.disabled_at IS NULL
		   )
		 LIMIT 1`).Scan(&contractID)
	if err != nil {
		t.Skip("no publisher-configured appointment contract fixture")
	}

	svc := NewService(pool, nil, nil, nil)
	ok, err := svc.ownCalendarConfigured(ctx, contractID, false)
	if err != nil {
		t.Fatalf("ownCalendarConfigured: %v", err)
	}
	if !ok {
		t.Fatal("expected publisher own calendar configured without appointment_calendar_source")
	}
}

func TestCrossCalendarConfigured_blockedUntilAccepted(t *testing.T) {
	pool := connectAppointmentsTestDB(t)
	ctx := context.Background()

	var contractID int64
	err := pool.QueryRow(ctx,
		`SELECT c.id
		 FROM contracts c
		 JOIN buyer_booking_calendars bc ON bc.id = c.appointment_calendar_id
		 WHERE c.lead_type = 'Appointment' AND c.status = 'draft' AND c.deleted_at IS NULL
		   AND c.appointment_calendar_id IS NOT NULL
		   AND bc.schedule::text NOT IN ('{}', 'null')
		   AND EXISTS(
		     SELECT 1 FROM buyer_appointment_slots sl
		     WHERE sl.calendar_id = bc.id AND sl.disabled_at IS NULL
		   )
		 LIMIT 1`).Scan(&contractID)
	if err != nil {
		t.Skip("no draft appointment contract with buyer calendar fixture")
	}

	svc := NewService(pool, nil, nil, nil)
	ok, err := svc.crossCalendarConfigured(ctx, contractID, false)
	if err != nil {
		t.Fatalf("crossCalendarConfigured: %v", err)
	}
	if ok {
		t.Fatal("expected cross calendar not configured for draft contract")
	}

	_, err = svc.resolveCrossBookingCalendar(ctx, contractID, false)
	if err == nil {
		t.Fatal("expected error resolving cross calendar on draft contract")
	}
	var appErr *httpx.AppError
	if !errors.As(err, &appErr) || appErr.Code != httpx.CodeValidation {
		t.Fatalf("expected validation error, got %v", err)
	}
}

func TestListFreeSlots_ownPublisherCalendar(t *testing.T) {
	pool := connectAppointmentsTestDB(t)
	ctx := context.Background()

	var publisherID, contractID int64
	err := pool.QueryRow(ctx,
		`SELECT c.publisher_id, c.id
		 FROM contracts c
		 JOIN publisher_booking_calendars pc ON pc.id = c.publisher_appointment_calendar_id
		 WHERE c.lead_type = 'Appointment' AND c.status = 'active' AND c.deleted_at IS NULL
		   AND c.publisher_appointment_calendar_id IS NOT NULL
		   AND pc.schedule::text NOT IN ('{}', 'null')
		   AND EXISTS(
		     SELECT 1 FROM publisher_appointment_slots sl
		     WHERE sl.calendar_id = pc.id AND sl.disabled_at IS NULL
		   )
		 LIMIT 1`).Scan(&publisherID, &contractID)
	if err != nil {
		t.Skip("no publisher own-calendar appointment contract fixture")
	}

	var weekday int
	err = pool.QueryRow(ctx,
		`SELECT sl.weekday
		 FROM publisher_appointment_slots sl
		 JOIN contracts c ON c.publisher_appointment_calendar_id = sl.calendar_id
		 WHERE c.id = $1 AND sl.disabled_at IS NULL
		 LIMIT 1`, contractID).Scan(&weekday)
	if err != nil {
		t.Skip("no publisher slots for contract")
	}

	svc := NewService(pool, nil, nil, nil)
	date := nextWeekdayDate(weekday)
	slots, err := svc.ListFreeSlots(ctx, publisherID, contractID, date, bookingTargetOwn)
	if err != nil {
		t.Fatalf("ListFreeSlots own: %v", err)
	}
	if len(slots) == 0 {
		t.Fatalf("expected own-booking free slots for contract %d on %s", contractID, date)
	}
}

func nextWeekdayDate(weekday int) string {
	now := time.Now().UTC()
	diff := weekday - int(now.Weekday())
	if diff <= 0 {
		diff += 7
	}
	return now.AddDate(0, 0, diff).Format("2006-01-02")
}
