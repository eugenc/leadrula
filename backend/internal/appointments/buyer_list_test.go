package appointments

import (
	"context"
	"testing"

	"github.com/echayko/leadrula/backend/internal/config"
	"github.com/echayko/leadrula/backend/internal/database"
	"github.com/jackc/pgx/v5/pgxpool"
)

func connectAppointmentsTestDB(t *testing.T) *pgxpool.Pool {
	t.Helper()
	cfg := config.Load()
	pool, err := database.Connect(context.Background(), cfg.DatabaseURL)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(func() { pool.Close() })
	return pool
}

func TestListBuyerBookings_routeAppointmentLead(t *testing.T) {
	pool := connectAppointmentsTestDB(t)
	ctx := context.Background()

	var buyerID int64
	err := pool.QueryRow(ctx,
		`SELECT l.owner_account_id
		 FROM leads l
		 JOIN contracts c ON c.id = l.contract_id AND c.lead_type = 'Appointment'
		 WHERE l.deleted_at IS NULL
		   AND l.status IN ('distributed', 'closed', 'review')
		   AND l.contract_id IS NOT NULL
		   AND NOT EXISTS (SELECT 1 FROM lead_appointment_bookings b WHERE b.lead_id = l.id)
		 LIMIT 1`).Scan(&buyerID)
	if err != nil {
		t.Skip("no route-delivered appointment lead fixture in database")
	}

	svc := NewService(pool, nil, nil, nil)
	res, err := svc.ListBuyerBookings(ctx, BuyerListParams{
		BuyerID:           buyerID,
		Page:              1,
		Limit:             50,
		AppointmentPreset: "all",
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Total == 0 {
		t.Fatal("expected at least one appointment row for buyer with route-delivered lead")
	}
	foundRoute := false
	for _, row := range res.Items {
		if row.IsRoute {
			foundRoute = true
			if row.LeadID == 0 {
				t.Fatal("route row missing lead_id")
			}
			if row.BookedAt.IsZero() {
				t.Fatal("route row missing booked_at")
			}
			break
		}
	}
	if !foundRoute {
		t.Fatal("expected a route-delivered row in results")
	}
}

func TestListBuyerBookings_presetExcludesNullAppointment(t *testing.T) {
	pool := connectAppointmentsTestDB(t)
	ctx := context.Background()

	var buyerID int64
	_ = pool.QueryRow(ctx, `SELECT id FROM accounts WHERE type = 'buyer' LIMIT 1`).Scan(&buyerID)
	if buyerID == 0 {
		t.Skip("no buyer account")
	}

	svc := NewService(pool, nil, nil, nil)
	allRes, err := svc.ListBuyerBookings(ctx, BuyerListParams{
		BuyerID: buyerID, Page: 1, Limit: 200, AppointmentPreset: "all",
	})
	if err != nil {
		t.Fatal(err)
	}
	weekRes, err := svc.ListBuyerBookings(ctx, BuyerListParams{
		BuyerID: buyerID, Page: 1, Limit: 200, AppointmentPreset: "this_week",
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range weekRes.Items {
		if row.AppointmentAt == nil {
			t.Fatal("this_week preset should exclude rows without appointment_at")
		}
	}
	if weekRes.Total > allRes.Total {
		t.Fatalf("week total %d > all total %d", weekRes.Total, allRes.Total)
	}
}
