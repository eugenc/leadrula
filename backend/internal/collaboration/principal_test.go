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
	tokens := auth.NewTokenManager(cfg.JWTAccessSecret, cfg.JWTRefreshSecret, cfg.AccessTokenTTL, cfg.RefreshTokenTTL)
	return NewService(NewRepository(pool), nil, tokens)
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

func TestResolvePrincipal_impersonation_platformAdmin(t *testing.T) {
	svc := connectPrincipalTestDB(t)
	ctx := context.Background()

	var platformUserPublicID, pubPublicID, buyerPublicID string
	var collabVersion int64
	err := svc.repo.pool.QueryRow(ctx,
		`SELECT u.public_id, pub.public_id, buyer.public_id, bc.version
		 FROM buyer_collaborations bc
		 JOIN accounts pub ON pub.id = bc.publisher_id AND pub.deleted_at IS NULL
		 JOIN accounts buyer ON buyer.id = bc.buyer_id AND buyer.deleted_at IS NULL
		 JOIN users u
		   ON u.account_id = (SELECT a.id FROM accounts a WHERE a.type = 'platform' AND a.deleted_at IS NULL LIMIT 1)
		 WHERE bc.status = 'active'
		   AND u.role = 'admin' AND u.is_active
		 LIMIT 1`).Scan(&platformUserPublicID, &pubPublicID, &buyerPublicID, &collabVersion)
	if err != nil {
		t.Skip("no platform admin + active collaboration fixture")
	}

	p, err := svc.ResolvePrincipal(ctx, &auth.Claims{
		Subject:          platformUserPublicID,
		AccountID:        buyerPublicID,
		Impersonating:    true,
		ImpersonatorAcct: pubPublicID,
		CollabVersion:    collabVersion,
	})
	if err != nil {
		t.Fatal(err)
	}
	if p.AccountType != "buyer" {
		t.Fatalf("AccountType = %q, want buyer", p.AccountType)
	}
	if p.Impersonator == nil || p.Impersonator.AccountType != "publisher" {
		t.Fatalf("Impersonator = %+v, want publisher impersonator", p.Impersonator)
	}
	if p.Impersonator.AccountPublicID != pubPublicID {
		t.Fatalf("Impersonator.AccountPublicID = %q, want %q", p.Impersonator.AccountPublicID, pubPublicID)
	}
	if pubID, ok := p.OversightPublisherID(); !ok || pubID != p.Impersonator.AccountID {
		t.Fatalf("OversightPublisherID() = (%d, %v), want impersonator account id", pubID, ok)
	}
}

func TestResolvePrincipal_impersonation_publisherAdmin(t *testing.T) {
	svc := connectPrincipalTestDB(t)
	ctx := context.Background()

	var userPublicID, pubPublicID, buyerPublicID string
	var collabVersion int64
	err := svc.repo.pool.QueryRow(ctx,
		`SELECT u.public_id, pub.public_id, buyer.public_id, bc.version
		 FROM buyer_collaborations bc
		 JOIN accounts pub ON pub.id = bc.publisher_id AND pub.deleted_at IS NULL
		 JOIN accounts buyer ON buyer.id = bc.buyer_id AND buyer.deleted_at IS NULL
		 JOIN users u ON u.account_id = pub.id
		 WHERE bc.status = 'active'
		   AND u.role = 'admin' AND u.is_active
		 LIMIT 1`).Scan(&userPublicID, &pubPublicID, &buyerPublicID, &collabVersion)
	if err != nil {
		t.Skip("no publisher admin + active collaboration fixture")
	}

	p, err := svc.ResolvePrincipal(ctx, &auth.Claims{
		Subject:          userPublicID,
		AccountID:        buyerPublicID,
		Impersonating:    true,
		ImpersonatorAcct: pubPublicID,
		CollabVersion:    collabVersion,
	})
	if err != nil {
		t.Fatal(err)
	}
	if p.AccountType != "buyer" || p.Impersonator == nil {
		t.Fatalf("principal = %+v, want buyer with impersonator", p)
	}
}

func TestResolvePrincipal_impersonation_nonAdminRejected(t *testing.T) {
	svc := connectPrincipalTestDB(t)
	ctx := context.Background()

	var userPublicID, pubPublicID, buyerPublicID string
	var collabVersion int64
	err := svc.repo.pool.QueryRow(ctx,
		`SELECT u.public_id, pub.public_id, buyer.public_id, bc.version
		 FROM buyer_collaborations bc
		 JOIN accounts pub ON pub.id = bc.publisher_id AND pub.deleted_at IS NULL
		 JOIN accounts buyer ON buyer.id = bc.buyer_id AND buyer.deleted_at IS NULL
		 JOIN users u
		   ON u.account_id = (SELECT a.id FROM accounts a WHERE a.type = 'platform' AND a.deleted_at IS NULL LIMIT 1)
		 WHERE bc.status = 'active'
		   AND u.role = 'user' AND u.is_active
		 LIMIT 1`).Scan(&userPublicID, &pubPublicID, &buyerPublicID, &collabVersion)
	if err != nil {
		t.Skip("no platform non-admin + active collaboration fixture")
	}

	_, err = svc.ResolvePrincipal(ctx, &auth.Claims{
		Subject:          userPublicID,
		AccountID:        buyerPublicID,
		Impersonating:    true,
		ImpersonatorAcct: pubPublicID,
		CollabVersion:    collabVersion,
	})
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

func TestStartImpersonation_thenResolvePrincipal_switchedPublisher(t *testing.T) {
	svc := connectPrincipalTestDB(t)
	ctx := context.Background()

	var platformUserPublicID, pubPublicID, buyerPublicID string
	var platformUserID, pubID int64
	err := svc.repo.pool.QueryRow(ctx,
		`SELECT u.public_id, u.id, pub.public_id, pub.id, buyer.public_id
		 FROM buyer_collaborations bc
		 JOIN accounts pub ON pub.id = bc.publisher_id AND pub.deleted_at IS NULL
		 JOIN accounts buyer ON buyer.id = bc.buyer_id AND buyer.deleted_at IS NULL
		 JOIN users u
		   ON u.account_id = (SELECT a.id FROM accounts a WHERE a.type = 'platform' AND a.deleted_at IS NULL LIMIT 1)
		 WHERE bc.status = 'active'
		   AND u.role = 'admin' AND u.is_active
		 LIMIT 1`).Scan(&platformUserPublicID, &platformUserID, &pubPublicID, &pubID, &buyerPublicID)
	if err != nil {
		t.Skip("no platform admin + active collaboration fixture")
	}

	// Platform admin switched into publisher (same principal StartImpersonation sees).
	switchedPublisher := &auth.Principal{
		UserID:          platformUserID,
		UserPublicID:    platformUserPublicID,
		AccountID:       pubID,
		AccountPublicID: pubPublicID,
		AccountType:     "publisher",
		Role:            "admin",
		FullAccess:      true,
		Perms:           permissions.FullAccess("publisher"),
	}

	res, err := svc.StartImpersonation(ctx, switchedPublisher, buyerPublicID)
	if err != nil {
		t.Fatal(err)
	}
	claims, err := svc.tokens.ParseAccess(res.Access)
	if err != nil {
		t.Fatal(err)
	}
	p, err := svc.ResolvePrincipal(ctx, claims)
	if err != nil {
		t.Fatal(err)
	}
	if p.AccountType != "buyer" || p.Impersonator == nil || p.Impersonator.AccountType != "publisher" {
		t.Fatalf("principal = %+v, want buyer with publisher impersonator", p)
	}
}
