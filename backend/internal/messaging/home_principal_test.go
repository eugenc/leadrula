package messaging

import (
	"context"
	"testing"

	"github.com/echayko/leadrula/backend/internal/accounts"
	"github.com/echayko/leadrula/backend/internal/auth"
)

func TestHomePrincipal_platformOriginSwitchUsesActiveAccount(t *testing.T) {
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

	var buyerID int64
	var buyerPub string
	err = pool.QueryRow(ctx,
		`SELECT id, public_id::text FROM accounts WHERE type='buyer' AND deleted_at IS NULL LIMIT 1`).
		Scan(&buyerID, &buyerPub)
	if err != nil {
		t.Skip("no buyer account in database")
	}

	var userID int64
	var userPub string
	err = pool.QueryRow(ctx,
		`SELECT id, public_id::text FROM users WHERE account_id=$1 AND role='admin' AND is_active LIMIT 1`, platformID).
		Scan(&userID, &userPub)
	if err != nil {
		t.Skip("no platform admin user")
	}

	svc := NewService(pool, NewHub(), nil, nil, accounts.NewRepository(pool))

	switched := &auth.Principal{
		UserID:          userID,
		UserPublicID:    userPub,
		AccountID:       buyerID,
		AccountPublicID: buyerPub,
		AccountType:     "buyer",
		Role:            "admin",
		SwitchedFrom:    platformPub,
	}

	hp, err := svc.homePrincipal(ctx, switched)
	if err != nil {
		t.Fatalf("homePrincipal: %v", err)
	}
	if hp.AccountID != buyerID {
		t.Fatalf("AccountID = %d, want active buyer %d", hp.AccountID, buyerID)
	}
	if hp.AccountType != "buyer" {
		t.Fatalf("AccountType = %q, want buyer", hp.AccountType)
	}
}

func TestHomePrincipal_publisherOriginSwitchUsesHomeAccount(t *testing.T) {
	pool := connectMessagingPool(t)
	ctx := context.Background()

	var publisherID int64
	var publisherPub string
	err := pool.QueryRow(ctx,
		`SELECT id, public_id::text FROM accounts WHERE type='publisher' AND deleted_at IS NULL LIMIT 1`).
		Scan(&publisherID, &publisherPub)
	if err != nil {
		t.Skip("no publisher account in database")
	}

	var buyerID int64
	var buyerPub string
	err = pool.QueryRow(ctx,
		`SELECT id, public_id::text FROM accounts WHERE type='buyer' AND deleted_at IS NULL AND id <> $1 LIMIT 1`, publisherID).
		Scan(&buyerID, &buyerPub)
	if err != nil {
		t.Skip("no buyer account in database")
	}

	var userID int64
	var userPub string
	err = pool.QueryRow(ctx,
		`SELECT id, public_id::text FROM users WHERE account_id=$1 AND role='admin' AND is_active LIMIT 1`, publisherID).
		Scan(&userID, &userPub)
	if err != nil {
		t.Skip("no publisher admin user")
	}

	svc := NewService(pool, NewHub(), nil, nil, accounts.NewRepository(pool))

	switched := &auth.Principal{
		UserID:          userID,
		UserPublicID:    userPub,
		AccountID:       buyerID,
		AccountPublicID: buyerPub,
		AccountType:     "buyer",
		Role:            "admin",
		SwitchedFrom:    publisherPub,
	}

	hp, err := svc.homePrincipal(ctx, switched)
	if err != nil {
		t.Fatalf("homePrincipal: %v", err)
	}
	if hp.AccountID != publisherID {
		t.Fatalf("AccountID = %d, want origin publisher %d", hp.AccountID, publisherID)
	}
	if hp.AccountType != "publisher" {
		t.Fatalf("AccountType = %q, want publisher", hp.AccountType)
	}
}
