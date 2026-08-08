package intake

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/echayko/leadrula/backend/internal/config"
	"github.com/echayko/leadrula/backend/internal/database"
	"github.com/echayko/leadrula/backend/internal/routing"
)

func TestIngestFromSource_rejectsCallSource(t *testing.T) {
	cfg := config.Load()
	ctx := context.Background()
	pool, err := database.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		t.Skipf("connect: %v", err)
	}
	t.Cleanup(func() { pool.Close() })

	routeSvc := routing.NewService(pool)
	svc := &Service{pool: pool}

	var publisherID int64
	if err := pool.QueryRow(ctx,
		`SELECT id FROM accounts WHERE account_type = 'publisher' ORDER BY id LIMIT 1`).Scan(&publisherID); err != nil {
		t.Skip("no publisher account")
	}

	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	connID := int64(0)
	tracking := "+1555" + suffix[len(suffix)-7:]
	sid := "PN" + suffix
	src, err := routeSvc.CreateSource(ctx, publisherID, "Call reject "+suffix, "call-reject-"+suffix, "call", nil, &routing.CallSourceParams{
		IntegrationConnectionID: &connID,
		TrackingNumber:          &tracking,
		TwilioSID:               &sid,
	}, nil)
	if err != nil {
		t.Skipf("CreateSource call: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM routing_sources WHERE id=$1`, src.ID)
	})

	_, err = svc.IngestFromSource(ctx, publisherID, src.Slug, map[string]any{"phone": "+15551234567"})
	if err == nil {
		t.Fatal("expected error for call source ingest")
	}
}
