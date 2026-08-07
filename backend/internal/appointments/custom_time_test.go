package appointments

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/echayko/leadrula/backend/internal/accounts"
	"github.com/echayko/leadrula/backend/internal/auth"
	"github.com/echayko/leadrula/backend/internal/leads"
	"github.com/echayko/leadrula/backend/internal/notifications"
	"github.com/echayko/leadrula/backend/pkg/httpx"
)

func TestBookBuyerCalendar_customTime(t *testing.T) {
	pool := connectAppointmentsTestDB(t)
	ensureTestMigrations(t, pool)
	ctx := context.Background()

	var buyerID, calendarID, userID int64
	var tz string
	err := pool.QueryRow(ctx,
		`SELECT bc.account_id, bc.id, bc.timezone,
		        (SELECT u.id FROM users u WHERE u.account_id = bc.account_id ORDER BY u.id LIMIT 1)
		 FROM buyer_booking_calendars bc
		 WHERE bc.schedule::text NOT IN ('{}', 'null')
		   AND EXISTS(
		     SELECT 1 FROM buyer_appointment_slots sl
		     WHERE sl.calendar_id = bc.id AND sl.disabled_at IS NULL
		   )
		 LIMIT 1`).Scan(&buyerID, &calendarID, &tz, &userID)
	if err != nil {
		t.Skip("no configured buyer booking calendar fixture")
	}

	leadsRepo := leads.NewRepository(pool)
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	raw, _ := json.Marshal(map[string]string{
		"first_name": "Custom",
		"last_name":  "TimeTest",
		"phone":      "5550008888",
	})
	leadID, _, err := leadsRepo.InsertLead(ctx, tx, buyerID, buyerID, "custom_time_test", raw)
	if err != nil {
		tx.Rollback(ctx)
		t.Fatalf("InsertLead: %v", err)
	}
	for field, val := range map[string]string{
		"first_name": "Custom",
		"last_name":  "TimeTest",
		"phone":      "5550008888",
	} {
		if err := leadsRepo.SetBuiltinField(ctx, tx, leadID, field, val); err != nil {
			tx.Rollback(ctx)
			t.Fatalf("SetBuiltinField: %v", err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatalf("commit lead: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, `DELETE FROM lead_appointment_bookings WHERE lead_id=$1`, leadID)
		_, _ = pool.Exec(ctx, `DELETE FROM lead_change_log WHERE lead_id=$1`, leadID)
		_, _ = pool.Exec(ctx, `DELETE FROM leads WHERE id=$1`, leadID)
	})

	svc := NewService(pool, leadsRepo, accounts.NewRepository(pool), notifications.NewService(pool, nil, nil, ""))
	loc := loadLocation(tz)
	slotStart := time.Now().In(loc).AddDate(0, 0, 3)
	slotStart = time.Date(slotStart.Year(), slotStart.Month(), slotStart.Day(), 14, 37, 0, 0, loc)

	p := &auth.Principal{
		UserID:      userID,
		AccountID:   buyerID,
		AccountType: "buyer",
		Role:        "admin",
	}
	row, err := svc.BookAsBuyer(ctx, p, BookParams{
		CalendarID:  calendarID,
		SlotStart:   slotStart,
		DurationMin: 45,
		CustomTime:  true,
		LeadID:      leadID,
	})
	if err != nil {
		t.Fatalf("BookAsBuyer custom_time: %v", err)
	}
	if row.ID == 0 {
		t.Fatal("expected booking id")
	}
	if row.DurationMin != 45 {
		t.Fatalf("duration_min = %d, want 45", row.DurationMin)
	}

	var custom bool
	err = pool.QueryRow(ctx, `SELECT custom_time FROM lead_appointment_bookings WHERE id=$1`, row.ID).Scan(&custom)
	if err != nil {
		t.Fatalf("load custom_time: %v", err)
	}
	if !custom {
		t.Fatal("expected custom_time=true on booking row")
	}
}

func TestBookBuyerCalendar_rejectsMismatchedTemplateSlot(t *testing.T) {
	pool := connectAppointmentsTestDB(t)
	ensureTestMigrations(t, pool)
	ctx := context.Background()

	var buyerID, calendarID, buyerSlotID, userID int64
	var tz string
	err := pool.QueryRow(ctx,
		`SELECT bc.account_id, bc.id, sl.id, bc.timezone,
		        (SELECT u.id FROM users u WHERE u.account_id = bc.account_id ORDER BY u.id LIMIT 1)
		 FROM buyer_booking_calendars bc
		 JOIN buyer_appointment_slots sl ON sl.calendar_id = bc.id AND sl.disabled_at IS NULL
		 WHERE bc.schedule::text NOT IN ('{}', 'null')
		 LIMIT 1`).Scan(&buyerID, &calendarID, &buyerSlotID, &tz, &userID)
	if err != nil {
		t.Skip("no configured buyer booking calendar fixture")
	}

	leadsRepo := leads.NewRepository(pool)
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	raw, _ := json.Marshal(map[string]string{
		"first_name": "Mismatch",
		"last_name":  "SlotTest",
		"phone":      "5550007777",
	})
	leadID, _, err := leadsRepo.InsertLead(ctx, tx, buyerID, buyerID, "custom_time_mismatch_test", raw)
	if err != nil {
		tx.Rollback(ctx)
		t.Fatalf("InsertLead: %v", err)
	}
	for field, val := range map[string]string{
		"first_name": "Mismatch",
		"last_name":  "SlotTest",
		"phone":      "5550007777",
	} {
		if err := leadsRepo.SetBuiltinField(ctx, tx, leadID, field, val); err != nil {
			tx.Rollback(ctx)
			t.Fatalf("SetBuiltinField: %v", err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatalf("commit lead: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, `DELETE FROM lead_appointment_bookings WHERE lead_id=$1`, leadID)
		_, _ = pool.Exec(ctx, `DELETE FROM lead_change_log WHERE lead_id=$1`, leadID)
		_, _ = pool.Exec(ctx, `DELETE FROM leads WHERE id=$1`, leadID)
	})

	svc := NewService(pool, leadsRepo, accounts.NewRepository(pool), notifications.NewService(pool, nil, nil, ""))
	loc := loadLocation(tz)
	slotStart := time.Now().In(loc).AddDate(0, 0, 4)
	slotStart = time.Date(slotStart.Year(), slotStart.Month(), slotStart.Day(), 10, 15, 0, 0, loc)

	p := &auth.Principal{
		UserID:      userID,
		AccountID:   buyerID,
		AccountType: "buyer",
		Role:        "admin",
	}
	_, err = svc.BookAsBuyer(ctx, p, BookParams{
		CalendarID:  calendarID,
		BuyerSlotID: buyerSlotID,
		SlotStart:   slotStart,
		LeadID:      leadID,
	})
	if err == nil {
		t.Fatal("expected validation error for mismatched slot_start")
	}
	var appErr *httpx.AppError
	if !errors.As(err, &appErr) || appErr.Code != httpx.CodeValidation {
		t.Fatalf("expected validation error, got: %v", err)
	}
}
