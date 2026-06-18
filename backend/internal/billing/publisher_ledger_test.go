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

func TestListPublisherTransactions_legacySaleFromDebitWithoutEarning(t *testing.T) {
	billingSvc, _ := connectPublisherLedgerTest(t)
	ctx := context.Background()

	var publisherID, leadID int64
	var amount float64
	err := billingSvc.pool.QueryRow(ctx,
		`SELECT c.publisher_id, t.lead_id, ABS(t.amount::float8)
		 FROM transactions t
		 JOIN contracts c ON c.id = t.contract_id AND c.deleted_at IS NULL
		 WHERE t.type = 'debit' AND t.amount < 0 AND t.lead_id IS NOT NULL
		   AND NOT EXISTS (
		     SELECT 1 FROM compensation_earnings ce
		     JOIN contract_compensations cc ON cc.id = ce.compensation_id
		     WHERE ce.lead_id = t.lead_id AND cc.contract_id = t.contract_id
		       AND ce.kind = 'distribute'
		   )
		 LIMIT 1`).Scan(&publisherID, &leadID, &amount)
	if err != nil {
		t.Skip("no legacy distribute debit without earning")
	}

	txns, err := billingSvc.ListPublisherTransactions(ctx, publisherID, 0, "")
	if err != nil {
		t.Fatal(err)
	}
	var legacy *Transaction
	for i := range txns {
		if txns[i].LedgerSource == "legacy" && txns[i].Category == "Sale" &&
			txns[i].LeadID != nil && *txns[i].LeadID == leadID {
			legacy = &txns[i]
			break
		}
	}
	if legacy == nil {
		t.Fatal("expected legacy Sale row from historical debit")
	}
	if legacy.Amount != amount {
		t.Fatalf("legacy sale amount = %v, want %v", legacy.Amount, amount)
	}
	if legacy.Amount <= 0 {
		t.Fatalf("legacy sale amount = %v, want positive", legacy.Amount)
	}
}

func TestListPublisherTransactions_noDuplicateWhenEarningExists(t *testing.T) {
	billingSvc, _ := connectPublisherLedgerTest(t)
	ctx := context.Background()

	var publisherID, leadID int64
	err := billingSvc.pool.QueryRow(ctx,
		`SELECT c.publisher_id, ce.lead_id
		 FROM compensation_earnings ce
		 JOIN contract_compensations cc ON cc.id = ce.compensation_id
		 JOIN contracts c ON c.id = cc.contract_id
		 JOIN transactions t ON t.lead_id = ce.lead_id AND t.contract_id = cc.contract_id
		   AND t.type = 'debit' AND t.amount < 0
		 WHERE ce.kind = 'distribute' AND ce.amount > 0
		 LIMIT 1`).Scan(&publisherID, &leadID)
	if err != nil {
		t.Skip("no lead with both earning and debit")
	}

	txns, err := billingSvc.ListPublisherTransactions(ctx, publisherID, 0, "")
	if err != nil {
		t.Fatal(err)
	}
	saleCount := 0
	for _, txn := range txns {
		if txn.Category != "Sale" {
			continue
		}
		if txn.LeadID == nil || *txn.LeadID != leadID {
			continue
		}
		saleCount++
		if txn.LedgerSource == "legacy" {
			t.Fatal("expected earning row only, got legacy duplicate")
		}
	}
	if saleCount != 1 {
		t.Fatalf("sale row count for lead = %d, want 1", saleCount)
	}
}

func TestListPublisherTransactions_returnsRowsForPublisherWithDebits(t *testing.T) {
	billingSvc, _ := connectPublisherLedgerTest(t)
	ctx := context.Background()

	var publisherID int64
	var debitCount int
	err := billingSvc.pool.QueryRow(ctx,
		`SELECT c.publisher_id, COUNT(*)::int
		 FROM transactions t
		 JOIN contracts c ON c.id = t.contract_id AND c.deleted_at IS NULL
		 WHERE t.type = 'debit' AND t.amount < 0 AND t.lead_id IS NOT NULL
		 GROUP BY c.publisher_id
		 ORDER BY COUNT(*) DESC
		 LIMIT 1`).Scan(&publisherID, &debitCount)
	if err != nil {
		t.Skip("no historical distribute debits in database")
	}
	if debitCount == 0 {
		t.Skip("no debits")
	}

	txns, err := billingSvc.ListPublisherTransactions(ctx, publisherID, 0, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(txns) == 0 {
		t.Fatalf("expected transactions for publisher with %d debits, got empty list", debitCount)
	}
	hasSale := false
	for _, txn := range txns {
		if txn.Category == "Sale" && txn.Amount > 0 {
			hasSale = true
			break
		}
	}
	if !hasSale {
		t.Fatal("expected at least one positive Sale row")
	}
}
