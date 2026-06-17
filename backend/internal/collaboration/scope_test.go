package collaboration

import (
	"context"
	"testing"

	"github.com/echayko/leadrula/backend/internal/auth"
	"github.com/echayko/leadrula/backend/internal/config"
	"github.com/echayko/leadrula/backend/internal/database"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestAppendLeadScope_noOversight(t *testing.T) {
	p := &auth.Principal{AccountID: 10, AccountType: "buyer"}
	base := "l.owner_account_id = $1 AND l.deleted_at IS NULL"
	where, args := AppendLeadScope(p, base, []any{int64(10)})
	if where != base {
		t.Fatalf("where = %q, want unchanged", where)
	}
	if len(args) != 1 {
		t.Fatalf("args len = %d, want 1", len(args))
	}
}

func TestAppendLeadScope_switchedFromPublisher(t *testing.T) {
	p := &auth.Principal{AccountID: 20, AccountType: "buyer", SwitchedFromPublisherID: 5}
	where, args := AppendLeadScope(p, "l.owner_account_id = $1 AND l.deleted_at IS NULL", []any{int64(20)})
	if len(args) != 2 || args[1] != int64(5) {
		t.Fatalf("args = %v, want buyer 20 and publisher 5", args)
	}
	if where == "l.owner_account_id = $1 AND l.deleted_at IS NULL" {
		t.Fatal("expected publisher scope SQL to be appended")
	}
}

func TestAppendLeadScope_impersonation(t *testing.T) {
	imp := &auth.Principal{AccountID: 5, AccountType: "publisher"}
	p := &auth.Principal{AccountID: 20, AccountType: "buyer", Impersonator: imp}
	where, args := AppendLeadScope(p, "l.owner_account_id = $1 AND l.deleted_at IS NULL", []any{int64(20)})
	if len(args) != 2 || args[1] != int64(5) {
		t.Fatalf("args = %v, want buyer 20 and publisher 5", args)
	}
	if where == "l.owner_account_id = $1 AND l.deleted_at IS NULL" {
		t.Fatal("expected publisher scope SQL to be appended")
	}
}

func TestOversightPublisherID(t *testing.T) {
	imp := &auth.Principal{AccountID: 7, AccountType: "publisher"}
	p := &auth.Principal{Impersonator: imp}
	if id, ok := p.OversightPublisherID(); !ok || id != 7 {
		t.Fatalf("impersonation: got (%d, %v)", id, ok)
	}
	p2 := &auth.Principal{SwitchedFromPublisherID: 9}
	if id, ok := p2.OversightPublisherID(); !ok || id != 9 {
		t.Fatalf("switch: got (%d, %v)", id, ok)
	}
	if _, ok := (&auth.Principal{}).OversightPublisherID(); ok {
		t.Fatal("expected no oversight for plain buyer principal")
	}
}

func connectScopeTestDB(t *testing.T) *pgxpool.Pool {
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

func TestLeadContractAllowed_directContract(t *testing.T) {
	pool := connectScopeTestDB(t)
	ctx := context.Background()

	var contractID, pubID, buyerID int64
	err := pool.QueryRow(ctx,
		`SELECT c.id, c.publisher_id, c.buyer_id
		 FROM contracts c
		 WHERE c.status = 'active' AND c.buyer_id IS NOT NULL AND c.deleted_at IS NULL
		 LIMIT 1`).Scan(&contractID, &pubID, &buyerID)
	if err != nil {
		t.Skip("no active direct contract fixture")
	}
	if !LeadContractAllowed(ctx, pool, contractID, pubID, buyerID) {
		t.Fatal("expected direct contract to be allowed")
	}
	if LeadContractAllowed(ctx, pool, contractID, pubID+999999, buyerID) {
		t.Fatal("expected wrong publisher to be denied")
	}
}

func TestLeadContractAllowed_participationContract(t *testing.T) {
	pool := connectScopeTestDB(t)
	ctx := context.Background()

	var contractID, pubID, buyerID int64
	err := pool.QueryRow(ctx,
		`SELECT c.id, c.publisher_id, cp.buyer_id
		 FROM contracts c
		 JOIN contract_participations cp ON cp.contract_id = c.id
		 WHERE c.status = 'active' AND c.buyer_id IS NULL AND c.deleted_at IS NULL
		   AND cp.status = 'active'
		 LIMIT 1`).Scan(&contractID, &pubID, &buyerID)
	if err != nil {
		t.Skip("no active participation contract fixture")
	}
	if !LeadContractAllowed(ctx, pool, contractID, pubID, buyerID) {
		t.Fatal("expected participation contract to be allowed")
	}
}
