package intake

import (
	"context"
	"errors"
	"testing"

	"github.com/echayko/leadrula/backend/internal/config"
	"github.com/echayko/leadrula/backend/internal/database"
	"github.com/echayko/leadrula/backend/pkg/httpx"
)

func TestGetIntegrationDelivery_notFound(t *testing.T) {
	cfg := config.Load()
	ctx := context.Background()
	pool, err := database.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(func() { pool.Close() })

	svc := &Service{pool: pool}
	_, err = svc.GetIntegrationDelivery(ctx, 999999999, 999999999)
	if err == nil {
		t.Fatal("expected not found")
	}
	var appErr *httpx.AppError
	if !errors.As(err, &appErr) || appErr.Code != httpx.CodeNotFound {
		t.Fatalf("expected not found, got %v", err)
	}
}

func TestGetIntegrationDelivery_attemptsOrdered(t *testing.T) {
	cfg := config.Load()
	ctx := context.Background()
	pool, err := database.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(func() { pool.Close() })

	var accountID, deliveryID int64
	err = pool.QueryRow(ctx,
		`SELECT c.account_id, q.id
		 FROM integration_delivery_queue q
		 JOIN integration_connections c ON c.id = q.connection_id
		 WHERE q.webhook_trigger_id IS NULL
		   AND EXISTS (SELECT 1 FROM integration_delivery_logs l WHERE l.queue_item_id = q.id)
		 ORDER BY q.created_at DESC
		 LIMIT 1`).Scan(&accountID, &deliveryID)
	if err != nil {
		t.Skip("no integration delivery with attempt logs in database")
	}

	svc := &Service{pool: pool}
	detail, err := svc.GetIntegrationDelivery(ctx, accountID, deliveryID)
	if err != nil {
		t.Fatalf("GetIntegrationDelivery: %v", err)
	}
	if detail.ID != deliveryID {
		t.Fatalf("id = %d, want %d", detail.ID, deliveryID)
	}
	if len(detail.Attempts) == 0 {
		t.Fatal("expected at least one attempt")
	}
	for i, a := range detail.Attempts {
		want := i + 1
		if a.AttemptNumber != want {
			t.Fatalf("attempt[%d].number = %d, want %d", i, a.AttemptNumber, want)
		}
	}
}
