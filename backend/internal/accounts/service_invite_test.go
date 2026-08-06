package accounts

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/echayko/leadrula/backend/internal/database"
	"github.com/echayko/leadrula/backend/internal/handlerid"
	"github.com/echayko/leadrula/backend/pkg/httpx"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestInviteStatus(t *testing.T) {
	if inviteStatus(time.Now().Add(time.Hour)) != "pending" {
		t.Fatal("expected pending for future expiry")
	}
	if inviteStatus(time.Now().Add(-time.Hour)) != "expired" {
		t.Fatal("expected expired for past expiry")
	}
}

func TestResendInvite_expiredInvite(t *testing.T) {
	ctx := context.Background()
	pool, err := database.Connect(ctx, "postgres://crm:crm@localhost:5432/crm?sslmode=disable")
	if err != nil {
		t.Skip(err)
	}
	defer pool.Close()

	repo := NewRepository(pool)
	svc := NewService(repo, nil, nil, nil)

	accountID, cleanup := insertTestAccount(t, ctx, pool)
	defer cleanup()

	email := fmt.Sprintf("expired-resend-%d@example.com", time.Now().UnixNano())
	token := randomToken()
	expired := time.Now().Add(-time.Hour)
	inv, err := repo.CreateInvite(ctx, accountID, email, "Expired User", "user", token, expired, nil)
	if err != nil {
		t.Fatalf("create invite: %v", err)
	}

	if err := svc.ResendInvite(ctx, accountID, inv.ID); err != nil {
		t.Fatalf("resend expired invite: %v", err)
	}

	row, err := repo.GetPendingInvite(ctx, accountID, inv.ID)
	if err != nil {
		t.Fatalf("get invite after resend: %v", err)
	}
	if row.Token == token {
		t.Fatal("expected token to rotate on resend")
	}
	if !row.ExpiresAt.After(time.Now()) {
		t.Fatal("expected expiry extended into the future")
	}
}

func TestInvite_reusesExpiredRow(t *testing.T) {
	ctx := context.Background()
	pool, err := database.Connect(ctx, "postgres://crm:crm@localhost:5432/crm?sslmode=disable")
	if err != nil {
		t.Skip(err)
	}
	defer pool.Close()

	repo := NewRepository(pool)
	svc := NewService(repo, nil, nil, nil)

	accountID, cleanup := insertTestAccount(t, ctx, pool)
	defer cleanup()

	email := fmt.Sprintf("expired-reinvite-%d@example.com", time.Now().UnixNano())
	token := randomToken()
	expired := time.Now().Add(-time.Hour)
	existing, err := repo.CreateInvite(ctx, accountID, email, "Old Name", "user", token, expired, nil)
	if err != nil {
		t.Fatalf("create invite: %v", err)
	}

	var before int
	if err := pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM invites WHERE account_id = $1 AND email = $2 AND accepted_at IS NULL`,
		accountID, email,
	).Scan(&before); err != nil {
		t.Fatalf("count invites: %v", err)
	}
	if before != 1 {
		t.Fatalf("expected 1 invite row before re-invite, got %d", before)
	}

	updated, err := svc.Invite(ctx, accountID, email, "New Name", "admin", nil)
	if err != nil {
		t.Fatalf("re-invite expired email: %v", err)
	}
	if updated.ID != existing.ID {
		t.Fatalf("invite id %d, want reused row %d", updated.ID, existing.ID)
	}
	if updated.FullName != "New Name" {
		t.Fatalf("full name %q, want New Name", updated.FullName)
	}
	if updated.Role != "admin" {
		t.Fatalf("role %q, want admin", updated.Role)
	}
	if !updated.ExpiresAt.After(time.Now()) {
		t.Fatal("expected refreshed expiry")
	}

	var after int
	if err := pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM invites WHERE account_id = $1 AND email = $2 AND accepted_at IS NULL`,
		accountID, email,
	).Scan(&after); err != nil {
		t.Fatalf("count invites after: %v", err)
	}
	if after != 1 {
		t.Fatalf("expected 1 invite row after re-invite, got %d", after)
	}
}

func TestAcceptInvite_rejectsExpiredToken(t *testing.T) {
	ctx := context.Background()
	pool, err := database.Connect(ctx, "postgres://crm:crm@localhost:5432/crm?sslmode=disable")
	if err != nil {
		t.Skip(err)
	}
	defer pool.Close()

	repo := NewRepository(pool)
	svc := NewService(repo, nil, nil, nil)

	accountID, cleanup := insertTestAccount(t, ctx, pool)
	defer cleanup()

	email := fmt.Sprintf("expired-accept-%d@example.com", time.Now().UnixNano())
	token := randomToken()
	expired := time.Now().Add(-time.Hour)
	if _, err := repo.CreateInvite(ctx, accountID, email, "Expired User", "user", token, expired, nil); err != nil {
		t.Fatalf("create invite: %v", err)
	}

	_, err = svc.AcceptInvite(ctx, token, "Expired User", "password123")
	if err == nil {
		t.Fatal("expected error accepting expired invite")
	}
	var appErr *httpx.AppError
	if !errors.As(err, &appErr) {
		t.Fatalf("expected AppError, got %v", err)
	}
	if appErr.Message != "invite expired or already used" {
		t.Fatalf("message %q, want invite expired or already used", appErr.Message)
	}
}

func insertTestAccount(t *testing.T, ctx context.Context, pool *pgxpool.Pool) (int64, func()) {
	t.Helper()
	name := fmt.Sprintf("Invite Test %d", time.Now().UnixNano())
	hid := handlerid.Generate("PB")
	var accountID int64
	err := pool.QueryRow(ctx,
		`INSERT INTO accounts(type, name, timezone, handler_id) VALUES ('publisher', $1, 'America/Toronto', $2) RETURNING id`,
		name, hid,
	).Scan(&accountID)
	if err != nil {
		t.Fatalf("insert account: %v", err)
	}
	return accountID, func() {
		_, _ = pool.Exec(ctx, `DELETE FROM accounts WHERE id = $1`, accountID)
	}
}
