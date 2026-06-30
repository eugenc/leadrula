package appointments

import (
	"context"
	"testing"
	"time"
)

func TestListFreeSlots_configuredContract(t *testing.T) {
	pool := connectAppointmentsTestDB(t)
	ctx := context.Background()

	var publisherID, contractID int64
	err := pool.QueryRow(ctx,
		`SELECT c.publisher_id, c.id
		 FROM contracts c
		 JOIN buyer_booking_calendars bc ON bc.id = c.appointment_calendar_id
		 WHERE c.lead_type = 'Appointment' AND c.status = 'active' AND c.deleted_at IS NULL
		   AND c.appointment_calendar_id IS NOT NULL
		   AND bc.schedule::text NOT IN ('{}', 'null')
		   AND EXISTS(
		     SELECT 1 FROM buyer_appointment_slots sl
		     WHERE sl.calendar_id = bc.id AND sl.disabled_at IS NULL
		   )
		 LIMIT 1`).Scan(&publisherID, &contractID)
	if err != nil {
		t.Skip("no configured appointment contract fixture in database")
	}

	var weekday int
	err = pool.QueryRow(ctx,
		`SELECT sl.weekday
		 FROM buyer_appointment_slots sl
		 JOIN contracts c ON c.appointment_calendar_id = sl.calendar_id
		 WHERE c.id = $1 AND sl.disabled_at IS NULL
		 LIMIT 1`, contractID).Scan(&weekday)
	if err != nil {
		t.Skip("no slots for contract")
	}

	loc := time.UTC
	now := time.Now().In(loc)
	diff := weekday - int(now.Weekday())
	if diff <= 0 {
		diff += 7
	}
	date := now.AddDate(0, 0, diff).Format("2006-01-02")

	svc := NewService(pool, nil, nil, nil)
	slots, err := svc.ListFreeSlots(ctx, publisherID, contractID, date)
	if err != nil {
		t.Fatalf("ListFreeSlots: %v", err)
	}
	if len(slots) == 0 {
		t.Fatalf("expected free slots for contract %d on %s (weekday %d)", contractID, date, weekday)
	}
}

func TestCountSlotOccupancy_routeLeadWithActionAt(t *testing.T) {
	pool := connectAppointmentsTestDB(t)
	ctx := context.Background()

	var buyerSlotID int64
	var slotStart time.Time
	err := pool.QueryRow(ctx,
		`SELECT sl.id, l.action_at
		 FROM leads l
		 JOIN contracts c ON c.id = l.contract_id
		 JOIN buyer_appointment_slots sl ON sl.calendar_id = c.appointment_calendar_id
		 WHERE c.lead_type = 'Appointment'
		   AND l.action_at IS NOT NULL
		   AND l.deleted_at IS NULL
		   AND NOT EXISTS (SELECT 1 FROM lead_appointment_bookings b WHERE b.lead_id = l.id)
		   AND sl.weekday = EXTRACT(DOW FROM l.action_at AT TIME ZONE COALESCE(
		     (SELECT timezone FROM buyer_booking_calendars WHERE id = sl.calendar_id), 'UTC'
		   ))::int
		   AND sl.start_time = (l.action_at AT TIME ZONE COALESCE(
		     (SELECT timezone FROM buyer_booking_calendars WHERE id = sl.calendar_id), 'UTC'
		   ))::time
		 LIMIT 1`).Scan(&buyerSlotID, &slotStart)
	if err != nil {
		t.Skip("no route-delivered appointment lead with action_at matching a slot template")
	}

	svc := NewService(pool, nil, nil, nil)
	n, err := svc.countSlotOccupancy(ctx, buyerSlotID, slotStart)
	if err != nil {
		t.Fatalf("countSlotOccupancy: %v", err)
	}
	if n < 1 {
		t.Fatalf("expected occupancy >= 1 for slot %d at %v, got %d", buyerSlotID, slotStart, n)
	}
}

func TestListFreeSlots_excludesRouteOccupiedSlot(t *testing.T) {
	pool := connectAppointmentsTestDB(t)
	ctx := context.Background()

	var publisherID, contractID, buyerSlotID int64
	var slotStart time.Time
	err := pool.QueryRow(ctx,
		`SELECT c.publisher_id, c.id, sl.id, l.action_at
		 FROM leads l
		 JOIN contracts c ON c.id = l.contract_id
		 JOIN buyer_appointment_slots sl ON sl.calendar_id = c.appointment_calendar_id
		 JOIN buyer_booking_calendars bc ON bc.id = sl.calendar_id
		 JOIN contract_appointment_slots cs ON cs.contract_id = c.id AND cs.buyer_slot_id = sl.id
		 WHERE c.lead_type = 'Appointment' AND c.status = 'active' AND c.deleted_at IS NULL
		   AND l.action_at IS NOT NULL AND l.deleted_at IS NULL
		   AND NOT EXISTS (SELECT 1 FROM lead_appointment_bookings b WHERE b.lead_id = l.id)
		   AND cs.enabled = true AND sl.disabled_at IS NULL
		   AND sl.weekday = EXTRACT(DOW FROM l.action_at AT TIME ZONE bc.timezone)::int
		   AND sl.start_time = (l.action_at AT TIME ZONE bc.timezone)::time
		   AND sl.capacity = 1
		 LIMIT 1`).Scan(&publisherID, &contractID, &buyerSlotID, &slotStart)
	if err != nil {
		t.Skip("no route-delivered lead occupying a capacity-1 slot")
	}

	loc, err := time.LoadLocation("America/New_York")
	if err != nil {
		loc = time.UTC
	}
	date := slotStart.In(loc).Format("2006-01-02")

	svc := NewService(pool, nil, nil, nil)
	slots, err := svc.ListFreeSlots(ctx, publisherID, contractID, date)
	if err != nil {
		t.Fatalf("ListFreeSlots: %v", err)
	}
	for _, s := range slots {
		if s.BuyerSlotID == buyerSlotID && s.SlotStart.Equal(slotStart) {
			t.Fatalf("occupied slot %d at %v should not appear in free slots", buyerSlotID, slotStart)
		}
	}
}
