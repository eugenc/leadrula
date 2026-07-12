package messaging

import (
	"context"
	"testing"

	"github.com/echayko/leadrula/backend/internal/accounts"
	"github.com/echayko/leadrula/backend/internal/auth"
	"github.com/echayko/leadrula/backend/internal/config"
	"github.com/echayko/leadrula/backend/internal/database"
)

func connectBroadcastPool(t *testing.T) *Service {
	t.Helper()
	cfg := config.Load()
	ctx := context.Background()
	pool, err := database.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(func() { pool.Close() })
	return NewService(pool, NewHub(), nil, nil, accounts.NewRepository(pool))
}

func TestCreateBroadcast_selectiveRecipients(t *testing.T) {
	svc := connectBroadcastPool(t)
	ctx := context.Background()

	var publisherID int64
	var publisherPub string
	err := svc.pool.QueryRow(ctx,
		`SELECT id, public_id::text FROM accounts WHERE type='publisher' AND deleted_at IS NULL LIMIT 1`).
		Scan(&publisherID, &publisherPub)
	if err != nil {
		t.Skip("no publisher account")
	}

	var userID int64
	err = svc.pool.QueryRow(ctx,
		`SELECT id FROM users WHERE account_id=$1 AND role='admin' AND is_active LIMIT 1`, publisherID).
		Scan(&userID)
	if err != nil {
		t.Skip("no publisher admin")
	}

	p := &auth.Principal{
		UserID:          userID,
		AccountID:       publisherID,
		AccountPublicID: publisherPub,
		AccountType:     "publisher",
		Role:            "admin",
		FullAccess:      true,
	}

	recipients, err := svc.ListBroadcastRecipients(ctx, p)
	if err != nil {
		t.Fatalf("ListBroadcastRecipients: %v", err)
	}
	if len(recipients) == 0 {
		t.Skip("no broadcast recipients for publisher")
	}

	pick := recipients[0].ID
	job, err := svc.CreateBroadcast(ctx, p, "test broadcast", []string{pick})
	if err != nil {
		t.Fatalf("CreateBroadcast: %v", err)
	}
	if job.TotalCount != 1 {
		t.Fatalf("total_count = %d, want 1", job.TotalCount)
	}
	if job.ID == "" {
		t.Fatal("expected job id")
	}

	_, err = svc.CreateBroadcast(ctx, p, "bad", []string{"00000000-0000-0000-0000-000000000000"})
	if err == nil {
		t.Fatal("expected error for invalid recipient")
	}
}
