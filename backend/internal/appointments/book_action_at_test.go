package appointments

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/echayko/leadrula/backend/internal/accounts"
	"github.com/echayko/leadrula/backend/internal/auth"
	"github.com/echayko/leadrula/backend/internal/leads"
	"github.com/echayko/leadrula/backend/internal/notifications"
)

func TestBook_setsLeadActionAt(t *testing.T) {
	pool := connectAppointmentsTestDB(t)
	ctx := context.Background()

	var publisherID, contractID, userID int64
	err := pool.QueryRow(ctx,
		`SELECT c.publisher_id, c.id,
		        (SELECT u.id FROM users u WHERE u.account_id = c.publisher_id ORDER BY u.id LIMIT 1)
		 FROM contracts c
		 JOIN buyer_booking_calendars bc ON bc.id = c.appointment_calendar_id
		 WHERE c.lead_type = 'Appointment' AND c.status = 'active' AND c.deleted_at IS NULL
		   AND c.appointment_calendar_id IS NOT NULL
		   AND bc.schedule::text NOT IN ('{}', 'null')
		   AND EXISTS(
		     SELECT 1 FROM buyer_appointment_slots sl
		     WHERE sl.calendar_id = bc.id AND sl.disabled_at IS NULL
		   )
		 LIMIT 1`).Scan(&publisherID, &contractID, &userID)
	if err != nil {
		t.Skip("no configured appointment contract fixture in database")
	}

	leadsRepo := leads.NewRepository(pool)
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	raw, _ := json.Marshal(map[string]string{
		"first_name": "ActionAt",
		"last_name":  "BookTest",
		"phone":      "5550009999",
	})
	leadID, _, err := leadsRepo.InsertLead(ctx, tx, publisherID, publisherID, "book_action_at_test", raw)
	if err != nil {
		tx.Rollback(ctx)
		t.Fatalf("InsertLead: %v", err)
	}
	for field, val := range map[string]string{
		"first_name": "ActionAt",
		"last_name":  "BookTest",
		"phone":      "5550009999",
	} {
		if err := leadsRepo.SetBuiltinField(ctx, tx, leadID, field, val); err != nil {
			tx.Rollback(ctx)
			t.Fatalf("SetBuiltinField: %v", err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatalf("commit lead: %v", err)
	}
	if _, err := pool.Exec(ctx, `UPDATE leads SET status='distributed' WHERE id=$1`, leadID); err != nil {
		t.Fatalf("mark lead distributed: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, `DELETE FROM lead_appointment_bookings WHERE lead_id=$1`, leadID)
		_, _ = pool.Exec(ctx, `DELETE FROM lead_change_log WHERE lead_id=$1`, leadID)
		_, _ = pool.Exec(ctx, `DELETE FROM leads WHERE id=$1`, leadID)
	})

	svc := NewService(pool, leadsRepo, accounts.NewRepository(pool), notifications.NewService(pool, nil, nil, ""))

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

	loc := time.UTC
	now := time.Now().In(loc)
	diff := weekday - int(now.Weekday())
	if diff <= 0 {
		diff += 7
	}
	date := now.AddDate(0, 0, diff).Format("2006-01-02")

	freeSlots, err := svc.ListFreeSlots(ctx, publisherID, contractID, date)
	if err != nil {
		t.Fatalf("ListFreeSlots: %v", err)
	}
	if len(freeSlots) == 0 {
		t.Skip("no free slots on " + date)
	}
	slot := freeSlots[0]

	p := &auth.Principal{
		UserID:      userID,
		AccountID:   publisherID,
		AccountType: "publisher",
		Role:        "admin",
	}
	_, err = svc.Book(ctx, p, BookParams{
		ContractID:   contractID,
		BuyerSlotID:  slot.BuyerSlotID,
		SlotStart:    slot.SlotStart,
		DeliveryMode: "contract",
		LeadID:       leadID,
	})
	if err != nil {
		t.Fatalf("Book: %v", err)
	}

	var actionAt time.Time
	err = pool.QueryRow(ctx, `SELECT action_at FROM leads WHERE id=$1`, leadID).Scan(&actionAt)
	if err != nil {
		t.Fatalf("load action_at: %v", err)
	}
	if !actionAt.Truncate(time.Minute).Equal(slot.SlotStart.Truncate(time.Minute)) {
		t.Fatalf("action_at = %v, want %v", actionAt, slot.SlotStart)
	}

	var logCount int
	err = pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM lead_change_log
		 WHERE lead_id=$1 AND change_kind='action_at' AND field_name='Action Date & Time'`,
		leadID).Scan(&logCount)
	if err != nil {
		t.Fatalf("count change log: %v", err)
	}
	if logCount < 1 {
		t.Fatal("expected action_at change log entry")
	}
}
