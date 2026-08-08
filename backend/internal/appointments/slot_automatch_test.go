package appointments

import (
	"context"
	"testing"
	"time"

	"github.com/echayko/leadrula/backend/internal/config"
	"github.com/echayko/leadrula/backend/internal/database"
)

func TestResolveSlotFromStart_buyerCalendar(t *testing.T) {
	cfg := config.Load()
	ctx := context.Background()
	pool, err := database.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		t.Skipf("connect: %v", err)
	}
	t.Cleanup(func() { pool.Close() })

	var contractID, publisherID int64
	err = pool.QueryRow(ctx,
		`SELECT c.id, c.publisher_id
		 FROM contracts c
		 JOIN buyer_booking_calendars bc ON bc.id = c.appointment_calendar_id
		 WHERE c.lead_type = 'Appointment' AND c.status = 'active' AND c.deleted_at IS NULL
		   AND c.appointment_calendar_source = 'buyer'
		   AND bc.schedule::text NOT IN ('{}', 'null')
		   AND EXISTS(SELECT 1 FROM buyer_appointment_slots sl WHERE sl.calendar_id = bc.id AND sl.disabled_at IS NULL)
		 LIMIT 1`).Scan(&contractID, &publisherID)
	if err != nil {
		t.Skip("no configured appointment contract fixture")
	}

	svc := NewService(pool, nil, nil, nil)

	var weekday int
	err = pool.QueryRow(ctx,
		`SELECT sl.weekday
		 FROM buyer_appointment_slots sl
		 JOIN contracts c ON c.appointment_calendar_id = sl.calendar_id
		 JOIN contract_appointment_slots cs ON cs.contract_id = c.id AND cs.buyer_slot_id = sl.id
		 WHERE c.id = $1 AND cs.enabled = true AND sl.disabled_at IS NULL
		 LIMIT 1`, contractID).Scan(&weekday)
	if err != nil {
		t.Skip("no enabled contract slot")
	}

	now := time.Now().UTC()
	diff := weekday - int(now.Weekday())
	if diff <= 0 {
		diff += 7
	}
	date := now.AddDate(0, 0, diff)

	freeSlots, err := svc.ListFreeSlots(ctx, publisherID, contractID, date.Format("2006-01-02"), bookingTargetActive)
	if err != nil || len(freeSlots) == 0 {
		t.Skip("no free slots for auto-match test")
	}
	slot := freeSlots[0]

	params, err := svc.ResolveSlotFromStart(ctx, contractID, slot.SlotStart)
	if err != nil {
		t.Fatalf("ResolveSlotFromStart: %v", err)
	}
	if params.BuyerSlotID == 0 {
		t.Fatal("expected buyer_slot_id")
	}
	if !params.SlotStart.Truncate(time.Minute).Equal(slot.SlotStart.Truncate(time.Minute)) {
		t.Fatalf("slot_start = %v, want %v", params.SlotStart, slot.SlotStart)
	}
}

func TestResolveSlotFromPublisherCalendar(t *testing.T) {
	cfg := config.Load()
	ctx := context.Background()
	pool, err := database.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		t.Skipf("connect: %v", err)
	}
	t.Cleanup(func() { pool.Close() })

	var publisherID, calendarID int64
	err = pool.QueryRow(ctx,
		`SELECT c.account_id, c.id
		 FROM publisher_booking_calendars c
		 WHERE c.schedule::text NOT IN ('{}', 'null')
		   AND EXISTS(SELECT 1 FROM publisher_appointment_slots sl WHERE sl.calendar_id = c.id AND sl.disabled_at IS NULL)
		 LIMIT 1`).Scan(&publisherID, &calendarID)
	if err != nil {
		t.Skip("no configured publisher calendar fixture")
	}

	svc := NewService(pool, nil, nil, nil)

	var weekday int
	err = pool.QueryRow(ctx,
		`SELECT sl.weekday FROM publisher_appointment_slots sl
		 WHERE sl.calendar_id = $1 AND sl.disabled_at IS NULL LIMIT 1`, calendarID).Scan(&weekday)
	if err != nil {
		t.Skip("no publisher calendar slots")
	}

	now := time.Now().UTC()
	diff := weekday - int(now.Weekday())
	if diff <= 0 {
		diff += 7
	}
	date := now.AddDate(0, 0, diff)

	freeSlots, err := svc.ListFreeSlotsByPublisherCalendar(ctx, publisherID, calendarID, date.Format("2006-01-02"))
	if err != nil || len(freeSlots) == 0 {
		t.Skip("no free publisher calendar slots")
	}
	slot := freeSlots[0]

	params, err := svc.ResolveSlotFromPublisherCalendar(ctx, publisherID, calendarID, slot.SlotStart)
	if err != nil {
		t.Fatalf("ResolveSlotFromPublisherCalendar: %v", err)
	}
	if params.CalendarID != calendarID {
		t.Fatalf("calendar_id = %d, want %d", params.CalendarID, calendarID)
	}
	if params.PublisherSlotID == 0 {
		t.Fatal("expected publisher_slot_id")
	}
}
