package billing

import (
	"context"
	"testing"

	"github.com/echayko/leadrula/backend/internal/config"
	"github.com/echayko/leadrula/backend/internal/database"
	"github.com/jackc/pgx/v5"
)

func connectBillingTestDB(t *testing.T) *Service {
	t.Helper()
	cfg := config.Load()
	ctx := context.Background()
	pool, err := database.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(func() { pool.Close() })
	return NewService(pool, nil, nil, nil, nil, "")
}

func TestCreateCompensationPayoutInvoice_createsLinkedInvoice(t *testing.T) {
	svc := connectBillingTestDB(t)
	ctx := context.Background()

	var clearID int64
	err := svc.pool.QueryRow(ctx,
		`SELECT pc.id
		 FROM compensation_payout_clears pc
		 JOIN contract_compensations cc ON cc.id = pc.compensation_id
		 WHERE cc.kind IN ('rev_share', 'profit_share')
		   AND pc.invoice_id IS NULL
		   AND pc.amount > 0
		 LIMIT 1`).Scan(&clearID)
	if err != nil {
		t.Skip("no rev/profit share payout clear without invoice")
	}

	tx, err := svc.pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)

	if err := svc.CreateCompensationPayoutInvoice(ctx, clearID); err != nil {
		t.Fatal(err)
	}

	var invoiceID int64
	var kind, status string
	err = tx.QueryRow(ctx,
		`SELECT pc.invoice_id, i.kind, i.status::text
		 FROM compensation_payout_clears pc
		 JOIN invoices i ON i.id = pc.invoice_id
		 WHERE pc.id = $1`,
		clearID).Scan(&invoiceID, &kind, &status)
	if err != nil {
		t.Fatal(err)
	}
	if invoiceID == 0 {
		t.Fatal("expected invoice linked to clear")
	}
	if kind != InvoiceKindCompensationPayout {
		t.Fatalf("kind = %q, want compensation_payout", kind)
	}
	if status != "open" {
		t.Fatalf("status = %q, want open", status)
	}

	var transferStatus string
	if err := tx.QueryRow(ctx,
		`SELECT stripe_transfer_status FROM compensation_payout_clears WHERE id = $1`,
		clearID).Scan(&transferStatus); err != nil {
		t.Fatal(err)
	}
	if transferStatus != "skipped" {
		t.Fatalf("transfer status = %q, want skipped", transferStatus)
	}
}

func TestSettleCompensationPayoutInvoice_doesNotCreditBalance(t *testing.T) {
	svc := connectBillingTestDB(t)
	ctx := context.Background()

	var invID, buyerID int64
	var publicID string
	var amount float64
	err := svc.pool.QueryRow(ctx,
		`SELECT i.id, i.buyer_id, i.public_id::text, i.amount::float8
		 FROM invoices i
		 WHERE i.kind = $1 AND i.status = 'open'
		 LIMIT 1`,
		InvoiceKindCompensationPayout).Scan(&invID, &buyerID, &publicID, &amount)
	if err != nil {
		t.Skip("no open compensation payout invoice")
	}

	tx, err := svc.pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)

	var balanceBefore float64
	if err := tx.QueryRow(ctx,
		`SELECT COALESCE(balance::float8, 0) FROM buyer_balances WHERE buyer_id = $1`,
		buyerID).Scan(&balanceBefore); err != nil && err != pgx.ErrNoRows {
		t.Fatal(err)
	}

	if err := svc.settleInvoiceTx(ctx, tx, publicID, amount, "bank_transfer", nil, nil, nil, nil); err != nil {
		t.Fatal(err)
	}

	var balanceAfter float64
	if err := tx.QueryRow(ctx,
		`SELECT COALESCE(balance::float8, 0) FROM buyer_balances WHERE buyer_id = $1`,
		buyerID).Scan(&balanceAfter); err != nil && err != pgx.ErrNoRows {
		t.Fatal(err)
	}
	if balanceAfter != balanceBefore {
		t.Fatalf("balance changed from %v to %v", balanceBefore, balanceAfter)
	}

	var txnType string
	if err := tx.QueryRow(ctx,
		`SELECT type::text FROM transactions WHERE buyer_id = $1 ORDER BY id DESC LIMIT 1`,
		buyerID).Scan(&txnType); err != nil {
		t.Fatal(err)
	}
	if txnType != "compensation_payout" {
		t.Fatalf("txn type = %q, want compensation_payout", txnType)
	}
}

func TestExecuteMarketplacePayoutTransfers_skipsRevShare(t *testing.T) {
	svc := connectBillingTestDB(t)
	ctx := context.Background()

	var publisherID, clearID int64
	err := svc.pool.QueryRow(ctx,
		`SELECT c.publisher_id, pc.id
		 FROM compensation_payout_clears pc
		 JOIN contract_compensations cc ON cc.id = pc.compensation_id
		 JOIN contracts c ON c.id = cc.contract_id
		 WHERE cc.kind IN ('rev_share', 'profit_share')
		   AND pc.stripe_transfer_status = 'pending'
		 LIMIT 1`).Scan(&publisherID, &clearID)
	if err != nil {
		t.Skip("no pending rev/profit share payout clear")
	}

	tx, err := svc.pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx,
		`UPDATE compensation_payout_clears SET stripe_transfer_status = 'pending' WHERE id = $1`,
		clearID); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}

	if err := svc.ExecuteMarketplacePayoutTransfers(ctx, publisherID); err != nil {
		t.Fatal(err)
	}

	var status string
	if err := svc.pool.QueryRow(ctx,
		`SELECT stripe_transfer_status FROM compensation_payout_clears WHERE id = $1`,
		clearID).Scan(&status); err != nil {
		t.Fatal(err)
	}
	if status != "skipped" {
		t.Fatalf("status = %q, want skipped", status)
	}
}
