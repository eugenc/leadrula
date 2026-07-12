package messaging

import (
	"context"
	"testing"

	"github.com/echayko/leadrula/backend/internal/accounts"
	"github.com/echayko/leadrula/backend/internal/auth"
	"github.com/echayko/leadrula/backend/internal/config"
	"github.com/echayko/leadrula/backend/internal/database"
	"github.com/jackc/pgx/v5/pgxpool"
)

func connectMessagingPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	cfg := config.Load()
	ctx := context.Background()
	pool, err := database.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(func() { pool.Close() })
	return pool
}

func TestCreateDirect_platformToPublisher(t *testing.T) {
	pool := connectMessagingPool(t)
	ctx := context.Background()

	var platformID int64
	var platformPub string
	err := pool.QueryRow(ctx,
		`SELECT id, public_id::text FROM accounts WHERE type='platform' AND deleted_at IS NULL LIMIT 1`).
		Scan(&platformID, &platformPub)
	if err != nil {
		t.Skip("no platform account in database")
	}

	var publisherPub string
	err = pool.QueryRow(ctx,
		`SELECT public_id::text FROM accounts WHERE type='publisher' AND deleted_at IS NULL LIMIT 1`).
		Scan(&publisherPub)
	if err != nil {
		t.Skip("no publisher account in database")
	}

	var userID int64
	err = pool.QueryRow(ctx,
		`SELECT id FROM users WHERE account_id=$1 AND role='admin' AND is_active LIMIT 1`, platformID).
		Scan(&userID)
	if err != nil {
		t.Skip("no platform admin user")
	}

	p := &auth.Principal{
		UserID:          userID,
		AccountID:       platformID,
		AccountPublicID: platformPub,
		AccountType:     "platform",
		Role:            "admin",
		FullAccess:      true,
	}

	svc := NewService(pool, NewHub(), nil, nil, accounts.NewRepository(pool))

	th, err := svc.CreateDirect(ctx, p, DirectRequest{
		RecipientAccountID: publisherPub,
		Context:            "general",
	})
	if err != nil {
		t.Fatalf("CreateDirect: %v", err)
	}
	if th.ID == "" {
		t.Fatal("expected non-empty thread id")
	}
	if th.Status != "active" {
		t.Fatalf("status = %q, want active", th.Status)
	}

	threads, err := svc.ListThreads(ctx, p, false, "")
	if err != nil {
		t.Fatalf("ListThreads: %v", err)
	}
	found := false
	for _, row := range threads {
		if row.ID == th.ID {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("thread %s missing from platform inbox", th.ID)
	}
}
