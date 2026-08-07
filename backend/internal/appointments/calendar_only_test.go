package appointments

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/echayko/leadrula/backend/db"
	"github.com/echayko/leadrula/backend/internal/database"
	"github.com/jackc/pgx/v5/pgxpool"
)

var migrateOnce sync.Once

func ensureTestMigrations(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	migrateOnce.Do(func() {
		if err := database.Migrate(context.Background(), pool, db.Migrations, db.Dir); err != nil {
			t.Fatalf("migrate: %v", err)
		}
	})
}

func TestListFreeSlotsByPublisherCalendar(t *testing.T) {
	pool := connectAppointmentsTestDB(t)
	ensureTestMigrations(t, pool)
	ctx := context.Background()

	var publisherID, calendarID int64
	var weekday int
	err := pool.QueryRow(ctx,
		`SELECT pc.account_id, pc.id, sl.weekday
		 FROM publisher_booking_calendars pc
		 JOIN publisher_appointment_slots sl ON sl.calendar_id = pc.id AND sl.disabled_at IS NULL
		 WHERE pc.schedule::text NOT IN ('{}', 'null')
		 LIMIT 1`).Scan(&publisherID, &calendarID, &weekday)
	if err != nil {
		t.Skip("no configured publisher booking calendar fixture")
	}

	svc := NewService(pool, nil, nil, nil)
	date := nextWeekdayDate(weekday)
	slots, err := svc.ListFreeSlotsByPublisherCalendar(ctx, publisherID, calendarID, date)
	if err != nil {
		t.Fatalf("ListFreeSlotsByPublisherCalendar: %v", err)
	}
	if len(slots) == 0 {
		t.Fatalf("expected free slots for publisher calendar %d on %s", calendarID, date)
	}
}

func TestDayInfoPublisherCalendar_workingHours(t *testing.T) {
	pool := connectAppointmentsTestDB(t)
	ensureTestMigrations(t, pool)
	ctx := context.Background()

	var publisherID, calendarID int64
	var weekday int
	var scheduleJSON []byte
	err := pool.QueryRow(ctx,
		`SELECT pc.account_id, pc.id, sl.weekday, pc.schedule
		 FROM publisher_booking_calendars pc
		 JOIN publisher_appointment_slots sl ON sl.calendar_id = pc.id AND sl.disabled_at IS NULL
		 WHERE pc.schedule::text NOT IN ('{}', 'null')
		 LIMIT 1`).Scan(&publisherID, &calendarID, &weekday, &scheduleJSON)
	if err != nil {
		t.Skip("no configured publisher booking calendar fixture")
	}

	svc := NewService(pool, nil, nil, nil)
	date := nextWeekdayDate(weekday)
	freeSlots, err := svc.ListFreeSlotsByPublisherCalendar(ctx, publisherID, calendarID, date)
	if err != nil {
		t.Fatalf("ListFreeSlotsByPublisherCalendar: %v", err)
	}
	_, _, workingHours, err := svc.dayInfoPublisherCalendar(ctx, publisherID, calendarID, date)
	if err != nil {
		t.Fatalf("dayInfoPublisherCalendar: %v", err)
	}
	if len(freeSlots) > 0 && workingHours == nil {
		t.Fatal("expected working_hours when free slots exist")
	}
	if workingHours == nil {
		return
	}
	sched := parseWeeklySchedule(scheduleJSON)
	expected, ok := sched.dayWindow(time.Weekday(weekday))
	if !ok {
		t.Fatalf("expected schedule window for weekday %d", weekday)
	}
	if workingHours.Start != expected.Start || workingHours.End != expected.End {
		t.Fatalf("working_hours = %+v, want start=%s end=%s", workingHours, expected.Start, expected.End)
	}
}

func TestListFreeSlotsByBuyerCalendar(t *testing.T) {
	pool := connectAppointmentsTestDB(t)
	ensureTestMigrations(t, pool)
	ctx := context.Background()

	var buyerID, calendarID int64
	var weekday int
	err := pool.QueryRow(ctx,
		`SELECT bc.account_id, bc.id, sl.weekday
		 FROM buyer_booking_calendars bc
		 JOIN buyer_appointment_slots sl ON sl.calendar_id = bc.id AND sl.disabled_at IS NULL
		 WHERE bc.schedule::text NOT IN ('{}', 'null')
		 LIMIT 1`).Scan(&buyerID, &calendarID, &weekday)
	if err != nil {
		t.Skip("no configured buyer booking calendar fixture")
	}

	svc := NewService(pool, nil, nil, nil)
	date := nextWeekdayDate(weekday)
	slots, err := svc.ListFreeSlotsByBuyerCalendar(ctx, buyerID, calendarID, date)
	if err != nil {
		t.Fatalf("ListFreeSlotsByBuyerCalendar: %v", err)
	}
	if len(slots) == 0 {
		t.Fatalf("expected free slots for buyer calendar %d on %s", calendarID, date)
	}
}
