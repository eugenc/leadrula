package routing

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestCreateSource_appointmentContractRequiresContract(t *testing.T) {
	pool := connectRoutingTestDB(t)
	ctx := context.Background()
	svc := NewService(pool)

	publisherID := testPublisherID(t, pool, ctx)
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	delivery := "contract"
	_, err := svc.CreateSource(ctx, publisherID, "Appt contract "+suffix, "appt-contract-"+suffix, "appointment", nil, nil, &AppointmentSourceParams{
		DeliveryMode: &delivery,
	})
	if err == nil {
		t.Fatal("expected validation error for missing contract_id")
	}
}

func TestCreateSource_appointmentPublisherRequiresCalendar(t *testing.T) {
	pool := connectRoutingTestDB(t)
	ctx := context.Background()
	svc := NewService(pool)

	publisherID := testPublisherID(t, pool, ctx)
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	delivery := "publisher"
	_, err := svc.CreateSource(ctx, publisherID, "Appt inbox "+suffix, "appt-inbox-"+suffix, "appointment", nil, nil, &AppointmentSourceParams{
		DeliveryMode: &delivery,
	})
	if err == nil {
		t.Fatal("expected validation error for missing calendar_id")
	}
}

func TestCreateSource_appointmentWithConfiguredContract(t *testing.T) {
	pool := connectRoutingTestDB(t)
	ctx := context.Background()
	svc := NewService(pool)

	var publisherID, contractID int64
	err := pool.QueryRow(ctx,
		`SELECT c.publisher_id, c.id
		 FROM contracts c
		 JOIN buyer_booking_calendars bc ON bc.id = c.appointment_calendar_id
		 WHERE c.lead_type = 'Appointment' AND c.status = 'active' AND c.deleted_at IS NULL
		   AND c.appointment_calendar_source = 'buyer'
		   AND bc.schedule::text NOT IN ('{}', 'null')
		   AND EXISTS(SELECT 1 FROM buyer_appointment_slots sl WHERE sl.calendar_id = bc.id AND sl.disabled_at IS NULL)
		 LIMIT 1`).Scan(&publisherID, &contractID)
	if err != nil {
		t.Skip("no configured appointment contract fixture")
	}

	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	delivery := "contract"
	src, err := svc.CreateSource(ctx, publisherID, "Appt source "+suffix, "appt-source-"+suffix, "appointment", nil, nil, &AppointmentSourceParams{
		ContractID:   &contractID,
		DeliveryMode: &delivery,
	})
	if err != nil {
		t.Fatalf("CreateSource: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM routing_sources WHERE id=$1`, src.ID)
	})
	if src.Type != "appointment" || src.ContractID == nil || *src.ContractID != contractID {
		t.Fatalf("unexpected source: %+v", src)
	}
	if src.CalendarID != nil && *src.CalendarID != 0 {
		t.Fatalf("calendar_id should be empty for contract delivery: %+v", src.CalendarID)
	}
}

func TestCreateSource_appointmentPublisherInbox(t *testing.T) {
	pool := connectRoutingTestDB(t)
	ctx := context.Background()
	svc := NewService(pool)

	var publisherID, calendarID int64
	err := pool.QueryRow(ctx,
		`SELECT c.account_id, c.id
		 FROM publisher_booking_calendars c
		 WHERE c.schedule::text NOT IN ('{}', 'null')
		   AND EXISTS(SELECT 1 FROM publisher_appointment_slots sl WHERE sl.calendar_id = c.id AND sl.disabled_at IS NULL)
		 LIMIT 1`).Scan(&publisherID, &calendarID)
	if err != nil {
		t.Skip("no configured publisher calendar fixture")
	}

	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	delivery := "publisher"
	src, err := svc.CreateSource(ctx, publisherID, "Appt inbox "+suffix, "appt-inbox-"+suffix, "appointment", nil, nil, &AppointmentSourceParams{
		CalendarID:   &calendarID,
		DeliveryMode: &delivery,
	})
	if err != nil {
		t.Fatalf("CreateSource: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM routing_sources WHERE id=$1`, src.ID)
	})
	if src.CalendarID == nil || *src.CalendarID != calendarID {
		t.Fatalf("calendar_id = %v, want %d", src.CalendarID, calendarID)
	}
	if src.ContractID != nil && *src.ContractID != 0 {
		t.Fatalf("contract_id should be empty: %+v", src.ContractID)
	}
}

func TestCreateSource_appointmentPublisherPipelineRequiresStage(t *testing.T) {
	pool := connectRoutingTestDB(t)
	ctx := context.Background()
	svc := NewService(pool)

	var publisherID, calendarID int64
	err := pool.QueryRow(ctx,
		`SELECT c.account_id, c.id
		 FROM publisher_booking_calendars c
		 WHERE c.schedule::text NOT IN ('{}', 'null')
		   AND EXISTS(SELECT 1 FROM publisher_appointment_slots sl WHERE sl.calendar_id = c.id AND sl.disabled_at IS NULL)
		 LIMIT 1`).Scan(&publisherID, &calendarID)
	if err != nil {
		t.Skip("no configured publisher calendar fixture")
	}

	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	delivery := "publisher_pipeline"
	_, err = svc.CreateSource(ctx, publisherID, "Appt pipe "+suffix, "appt-pipe-"+suffix, "appointment", nil, nil, &AppointmentSourceParams{
		CalendarID:   &calendarID,
		DeliveryMode: &delivery,
	})
	if err == nil {
		t.Fatal("expected validation error for missing pipeline/stage")
	}
}

func testPublisherID(t *testing.T, pool *pgxpool.Pool, ctx context.Context) int64 {
	t.Helper()
	var publisherID int64
	err := pool.QueryRow(ctx, `SELECT id FROM accounts WHERE account_type = 'publisher' ORDER BY id LIMIT 1`).Scan(&publisherID)
	if err != nil {
		t.Skip("no publisher account")
	}
	return publisherID
}
