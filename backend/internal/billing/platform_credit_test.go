package billing

import (
	"context"
	"testing"
)

func TestPlatformCreditBuyer_topup(t *testing.T) {
	pool := connectBillingPool(t)
	ctx := context.Background()

	var buyerID int64
	err := pool.QueryRow(ctx, `SELECT id FROM accounts WHERE type='buyer' AND deleted_at IS NULL LIMIT 1`).Scan(&buyerID)
	if err != nil {
		t.Skip("no buyer in database")
	}

	svc := NewService(pool, nil, nil, nil, nil, "")
	balBefore, err := svc.GetBalance(ctx, buyerID)
	if err != nil {
		t.Fatal(err)
	}

	const amount = 12.34
	txn, err := svc.Topup(ctx, buyerID, amount, "Platform admin credit: test")
	if err != nil {
		t.Fatal(err)
	}
	if txn.Type != "credit" {
		t.Fatalf("type = %q, want credit", txn.Type)
	}
	if txn.Amount != amount {
		t.Fatalf("amount = %v, want %v", txn.Amount, amount)
	}

	balAfter, err := svc.GetBalance(ctx, buyerID)
	if err != nil {
		t.Fatal(err)
	}
	if balAfter != balBefore+amount {
		t.Fatalf("balance = %v, want %v", balAfter, balBefore+amount)
	}

	var ledgerCount int
	if err := pool.QueryRow(ctx,
		`SELECT count(*) FROM transactions WHERE buyer_id=$1 AND type='credit' AND amount=$2 AND description=$3`,
		buyerID, amount, "Platform admin credit: test").Scan(&ledgerCount); err != nil {
		t.Fatal(err)
	}
	if ledgerCount != 1 {
		t.Fatalf("ledger rows = %d, want 1", ledgerCount)
	}
}
