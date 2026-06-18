package billing

import (
	"context"
	"testing"

	"github.com/echayko/leadrula/backend/internal/config"
	"github.com/echayko/leadrula/backend/internal/contracts"
	"github.com/echayko/leadrula/backend/internal/database"
)

func connectPublisherLedgerTest(t *testing.T) (*Service, *contracts.Service) {
	t.Helper()
	cfg := config.Load()
	ctx := context.Background()
	pool, err := database.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(func() { pool.Close() })
	return NewService(pool, nil, nil, nil, nil, ""), contracts.NewService(pool)
}

func TestListPublisherTransactions_usesEarningsForSales(t *testing.T) {
	billingSvc, _ := connectPublisherLedgerTest(t)
	ctx := context.Background()

	var publisherID, leadID int64
	var amount float64
	err := billingSvc.pool.QueryRow(ctx,
		`SELECT c.publisher_id, ce.lead_id, ce.amount::float8
		 FROM compensation_earnings ce
		 JOIN contract_compensations cc ON cc.id = ce.compensation_id
		 JOIN contracts c ON c.id = cc.contract_id
		 WHERE ce.kind = 'distribute' AND ce.amount > 0
		 LIMIT 1`).Scan(&publisherID, &leadID, &amount)
	if err != nil {
		t.Skip("no distribute earning in database")
	}

	txns, err := billingSvc.ListPublisherTransactions(ctx, publisherID, 0, "")
	if err != nil {
		t.Fatal(err)
	}

	var sale *Transaction
	for i := range txns {
		if txns[i].LedgerSource == "earning" && txns[i].Category == "Sale" && txns[i].LeadID != nil && *txns[i].LeadID == leadID {
			sale = &txns[i]
			break
		}
	}
	if sale == nil {
		t.Fatal("expected Sale earning row for distributed lead")
	}
	if sale.Amount <= 0 {
		t.Fatalf("sale amount = %v, want positive publisher sign", sale.Amount)
	}
	if sale.Amount != amount {
		t.Fatalf("sale amount = %v, want %v", sale.Amount, amount)
	}
	if sale.BalanceAfter != nil {
		t.Fatal("expected nil balance_after on earning row")
	}

	for _, txn := range txns {
		if txn.LedgerSource != "transaction" {
			continue
		}
		if txn.Category == "Sale" {
			t.Fatalf("unexpected buyer sale debit row id=%d in publisher transactions", txn.ID)
		}
		if txn.Type == "credit" || txn.Type == "dispute_credit" {
			t.Fatalf("unexpected buyer return/refund row id=%d in publisher transactions", txn.ID)
		}
	}
}

func TestListPublisherTransactions_returnIsNegative(t *testing.T) {
	billingSvc, _ := connectPublisherLedgerTest(t)
	ctx := context.Background()

	var publisherID int64
	err := billingSvc.pool.QueryRow(ctx,
		`SELECT c.publisher_id
		 FROM compensation_earnings ce
		 JOIN contract_compensations cc ON cc.id = ce.compensation_id
		 JOIN contracts c ON c.id = cc.contract_id
		 WHERE ce.kind = 'return' AND ce.amount < 0
		 LIMIT 1`).Scan(&publisherID)
	if err != nil {
		t.Skip("no return earning in database")
	}

	txns, err := billingSvc.ListPublisherTransactions(ctx, publisherID, 0, "")
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, txn := range txns {
		if txn.LedgerSource == "earning" && txn.Category == "Return" {
			if txn.Amount >= 0 {
				t.Fatalf("return amount = %v, want negative", txn.Amount)
			}
			found = true
			break
		}
	}
	if !found {
		t.Fatal("expected Return earning row")
	}
}

func TestListPublisherTransactions_excludesPayoutClears(t *testing.T) {
	billingSvc, contractsSvc := connectPublisherLedgerTest(t)
	ctx := context.Background()

	var publisherID int64
	err := billingSvc.pool.QueryRow(ctx,
		`SELECT c.publisher_id
		 FROM compensation_payout_clears pc
		 JOIN contract_compensations cc ON cc.id = pc.compensation_id
		 JOIN contracts c ON c.id = cc.contract_id
		 LIMIT 1`).Scan(&publisherID)
	if err != nil {
		t.Skip("no payout clear in database")
	}

	txns, err := billingSvc.ListPublisherTransactions(ctx, publisherID, 0, "")
	if err != nil {
		t.Fatal(err)
	}
	for _, txn := range txns {
		if txn.Category == "Payout" {
			t.Fatalf("unexpected payout row id=%d in transactions list", txn.ID)
		}
	}

	ledger, err := contractsSvc.ListPayoutLedger(ctx, publisherID)
	if err != nil {
		t.Fatal(err)
	}
	if len(ledger) == 0 {
		t.Fatal("expected payout ledger rows")
	}
	if ledger[0].Amount >= 0 {
		t.Fatalf("payout amount = %v, want negative", ledger[0].Amount)
	}
}
