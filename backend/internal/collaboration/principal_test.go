package collaboration

import (
	"context"
	"errors"
	"testing"

	"github.com/echayko/leadrula/backend/internal/auth"
	"github.com/echayko/leadrula/backend/internal/config"
	"github.com/echayko/leadrula/backend/internal/database"
	"github.com/echayko/leadrula/backend/internal/permissions"
)

func connectPrincipalTestDB(t *testing.T) *Service {
	t.Helper()
	cfg := config.Load()
	ctx := context.Background()
	pool, err := database.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(func() { pool.Close() })
	return NewService(NewRepository(pool), nil, nil)
}

func TestResolvePrincipal_loadsRoleLeadScope(t *testing.T) {
	svc := connectPrincipalTestDB(t)
	ctx := context.Background()

	var userPublicID, role string
	err := svc.repo.pool.QueryRow(ctx,
		`SELECT u.public_id, u.role
		 FROM users u
		 JOIN accounts a ON a.id = u.account_id
		 WHERE a.type = 'publisher' AND u.is_active AND a.deleted_at IS NULL
		   AND u.role IN ('admin', 'user')
		 LIMIT 1`).Scan(&userPublicID, &role)
	if err != nil {
		t.Skip("no active publisher user fixture")
	}

	p, err := svc.ResolvePrincipal(ctx, &auth.Claims{Subject: userPublicID})
	if err != nil {
		t.Fatal(err)
	}
	wantScope := permissions.PresetForRole(role, "publisher").LeadScope
	if p.LeadScope() != wantScope {
		t.Fatalf("LeadScope() = %q, want %q for role %q", p.LeadScope(), wantScope, role)
	}
	if p.Perms.LeadScope != wantScope {
		t.Fatalf("Perms.LeadScope = %q, want %q", p.Perms.LeadScope, wantScope)
	}
}

func TestResolvePrincipal_unknownUser(t *testing.T) {
	svc := connectPrincipalTestDB(t)
	ctx := context.Background()

	_, err := svc.ResolvePrincipal(ctx, &auth.Claims{Subject: "00000000-0000-0000-0000-000000000000"})
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}
