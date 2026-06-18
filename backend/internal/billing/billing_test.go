package billing

import (
	"context"
	"testing"

	"github.com/echayko/leadrula/backend/internal/config"
	"github.com/echayko/leadrula/backend/internal/database"
	"github.com/jackc/pgx/v5/pgxpool"
)

func connectBillingPool(t *testing.T) *pgxpool.Pool {
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

func TestCredit_refundsDistributeDebit(t *testing.T) {
	pool := connectBillingPool(t)
	ctx := context.Background()

	var buyerID, leadID, contractID int64
	err := pool.QueryRow(ctx,
		`SELECT l.owner_account_id, l.id, l.contract_id
		 FROM leads l
		 WHERE l.deleted_at IS NULL AND l.contract_id IS NOT NULL
		 LIMIT 1`).Scan(&buyerID, &leadID, &contractID)
	if err != nil {
		t.Skip("no distributed lead in database")
	}

	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)

	if err := EnsureBalance(ctx, tx, buyerID); err != nil {
		t.Fatal(err)
	}
	var balBefore float64
	if err := tx.QueryRow(ctx,
		`SELECT balance::float8 FROM buyer_balances WHERE buyer_id = $1`, buyerID).Scan(&balBefore); err != nil {
		t.Fatal(err)
	}

	const amount = 12.34
	if err := Debit(ctx, tx, buyerID, amount, leadID, contractID, "test distribute"); err != nil {
		t.Fatal(err)
	}
	got, err := DistributeDebitAmount(ctx, tx, buyerID, leadID, contractID)
	if err != nil {
		t.Fatal(err)
	}
	if got != amount {
		t.Fatalf("DistributeDebitAmount = %v, want %v", got, amount)
	}

	exists, err := ReturnCreditExists(ctx, tx, buyerID, leadID, contractID)
	if err != nil {
		t.Fatal(err)
	}
	if exists {
		t.Fatal("expected no prior return credit in rolled-back test tx")
	}

	if err := Credit(ctx, tx, buyerID, amount, leadID, contractID, "lead returned"); err != nil {
		t.Fatal(err)
	}

	var balAfter float64
	if err := tx.QueryRow(ctx,
		`SELECT balance::float8 FROM buyer_balances WHERE buyer_id = $1`, buyerID).Scan(&balAfter); err != nil {
		t.Fatal(err)
	}
	if balAfter != balBefore {
		t.Fatalf("balance after refund = %v, want %v", balAfter, balBefore)
	}

	exists, err = ReturnCreditExists(ctx, tx, buyerID, leadID, contractID)
	if err != nil {
		t.Fatal(err)
	}
	if !exists {
		t.Fatal("expected return credit row")
	}
}
