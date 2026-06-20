package billing

// Production verification (run for the publisher account on Billing → Transactions):
//
//   SELECT COUNT(*) FROM transactions t
//   JOIN contracts c ON c.id = t.contract_id
//   WHERE c.publisher_id = :pub_id AND t.type = 'debit' AND t.amount < 0;
//
//   SELECT COUNT(*) FROM compensation_earnings ce
//   JOIN contract_compensations cc ON cc.id = ce.compensation_id
//   JOIN contracts c ON c.id = cc.contract_id
//   WHERE c.publisher_id = :pub_id AND ce.kind = 'distribute';
//
//   SELECT COUNT(*) FROM transactions t
//   JOIN contracts c ON c.id = t.contract_id
//   WHERE c.publisher_id = :pub_id AND c.buyer_id IS NULL AND t.type = 'debit';
//
// If the first query is 0 for the viewed publisher but non-zero for another publisher,
// sales belong to that other account — switch to the publisher on the contract.

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
	var amount, debitBalance float64
	err := billingSvc.pool.QueryRow(ctx,
		`SELECT c.publisher_id, ce.lead_id, ce.amount::float8, d.balance_after::float8
		 FROM compensation_earnings ce
		 JOIN contract_compensations cc ON cc.id = ce.compensation_id
		 JOIN contracts c ON c.id = cc.contract_id
		 JOIN LATERAL (
		   SELECT bt.balance_after::float8 AS balance_after
		   FROM transactions bt
		   WHERE bt.lead_id = ce.lead_id AND bt.contract_id = cc.contract_id
		     AND bt.type = 'debit' AND bt.amount < 0 AND bt.description <> 'lead disputed'
		     AND bt.balance_after IS NOT NULL
		   ORDER BY bt.created_at DESC, bt.id DESC
		   LIMIT 1
		 ) d ON true
		 WHERE ce.kind = 'distribute' AND ce.amount > 0
		 LIMIT 1`).Scan(&publisherID, &leadID, &amount, &debitBalance)
	if err != nil {
		t.Skip("no distribute earning with matching buyer debit in database")
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
	if sale.BalanceAfter == nil {
		t.Fatal("expected buyer balance_after on earning row")
	}
	if *sale.BalanceAfter != debitBalance {
		t.Fatalf("sale balance_after = %v, want %v (buyer debit balance)", *sale.BalanceAfter, debitBalance)
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
			if txn.BalanceAfter == nil {
				t.Fatal("expected buyer balance_after on return earning row")
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
	var amount, balanceAfter float64
	err := billingSvc.pool.QueryRow(ctx,
		`SELECT c.publisher_id, t.lead_id, ABS(t.amount::float8), t.balance_after::float8
		 FROM transactions t
		 JOIN contracts c ON c.id = t.contract_id AND c.deleted_at IS NULL
		 WHERE t.type = 'debit' AND t.amount < 0 AND t.lead_id IS NOT NULL
		   AND t.balance_after IS NOT NULL
		   AND NOT EXISTS (
		     SELECT 1 FROM compensation_earnings ce
		     JOIN contract_compensations cc ON cc.id = ce.compensation_id
		     WHERE ce.lead_id = t.lead_id AND cc.contract_id = t.contract_id
		       AND ce.kind = 'distribute'
		   )
		 LIMIT 1`).Scan(&publisherID, &leadID, &amount, &balanceAfter)
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
	if legacy.BalanceAfter == nil {
		t.Fatal("expected buyer balance_after on legacy Sale row")
	}
	if *legacy.BalanceAfter != balanceAfter {
		t.Fatalf("legacy sale balance_after = %v, want %v", *legacy.BalanceAfter, balanceAfter)
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

func TestListPublisherTransactions_openOfferEarningVisible(t *testing.T) {
	billingSvc, _ := connectPublisherLedgerTest(t)
	ctx := context.Background()

	var publisherID, leadID int64
	var amount float64
	err := billingSvc.pool.QueryRow(ctx,
		`SELECT c.publisher_id, ce.lead_id, ce.amount::float8
		 FROM compensation_earnings ce
		 JOIN contract_compensations cc ON cc.id = ce.compensation_id
		 JOIN contracts c ON c.id = cc.contract_id AND c.deleted_at IS NULL
		 WHERE c.buyer_id IS NULL AND ce.kind = 'distribute' AND ce.amount > 0
		 LIMIT 1`).Scan(&publisherID, &leadID, &amount)
	if err != nil {
		t.Skip("no open-offer distribute earning in database")
	}

	txns, err := billingSvc.ListPublisherTransactions(ctx, publisherID, 0, "")
	if err != nil {
		t.Fatal(err)
	}

	var sale *Transaction
	for i := range txns {
		if txns[i].LedgerSource == "earning" && txns[i].Category == "Sale" &&
			txns[i].LeadID != nil && *txns[i].LeadID == leadID {
			sale = &txns[i]
			break
		}
	}
	if sale == nil {
		t.Fatal("expected Sale earning row for open-offer contract")
	}
	if sale.Amount != amount {
		t.Fatalf("sale amount = %v, want %v", sale.Amount, amount)
	}
	if sale.CounterpartyName == nil || *sale.CounterpartyName == "" {
		t.Fatal("expected counterparty buyer name on open-offer earning row")
	}

	saleCount := 0
	for _, txn := range txns {
		if txn.Category == "Sale" && txn.LeadID != nil && *txn.LeadID == leadID {
			saleCount++
		}
	}
	if saleCount != 1 {
		t.Fatalf("sale row count for lead = %d, want 1 (no duplicate legacy row)", saleCount)
	}
}

func TestListPublisherTransactions_leadViewableRespectsCollaboration(t *testing.T) {
	billingSvc, _ := connectPublisherLedgerTest(t)
	ctx := context.Background()

	var publisherID, leadID int64
	var hasCollab bool
	err := billingSvc.pool.QueryRow(ctx,
		`SELECT c.publisher_id, ce.lead_id,
		        EXISTS (
		          SELECT 1 FROM buyer_collaborations bc
		          WHERE bc.publisher_id = c.publisher_id AND bc.buyer_id = l.owner_account_id AND bc.status = 'active'
		        )
		 FROM compensation_earnings ce
		 JOIN contract_compensations cc ON cc.id = ce.compensation_id
		 JOIN contracts c ON c.id = cc.contract_id AND c.deleted_at IS NULL
		 JOIN leads l ON l.id = ce.lead_id
		 WHERE ce.kind = 'distribute' AND l.owner_account_id <> c.publisher_id
		 LIMIT 1`).Scan(&publisherID, &leadID, &hasCollab)
	if err != nil {
		t.Skip("no buyer-owned distribute earning in database")
	}

	txns, err := billingSvc.ListPublisherTransactions(ctx, publisherID, 0, "")
	if err != nil {
		t.Fatal(err)
	}
	var sale *Transaction
	for i := range txns {
		if txns[i].LedgerSource == "earning" && txns[i].Category == "Sale" &&
			txns[i].LeadID != nil && *txns[i].LeadID == leadID {
			sale = &txns[i]
			break
		}
	}
	if sale == nil {
		t.Fatal("expected Sale earning row for buyer-owned lead")
	}
	if sale.LeadViewable != hasCollab {
		t.Fatalf("lead_viewable = %v, want %v (active collaboration = %v)", sale.LeadViewable, hasCollab, hasCollab)
	}
}

func TestListTransactions_returnedLeadNotViewable(t *testing.T) {
	billingSvc, _ := connectPublisherLedgerTest(t)
	ctx := context.Background()

	var buyerID, txID int64
	var ownerIsBuyer bool
	err := billingSvc.pool.QueryRow(ctx,
		`SELECT t.buyer_id, t.id, (l.owner_account_id = t.buyer_id)
		 FROM transactions t
		 JOIN leads l ON l.id = t.lead_id
		 WHERE t.type = 'credit' AND t.description = 'lead returned'
		 LIMIT 1`).Scan(&buyerID, &txID, &ownerIsBuyer)
	if err != nil {
		t.Skip("no return credit with lead in database")
	}

	txns, err := billingSvc.ListTransactions(ctx, buyerID, "")
	if err != nil {
		t.Fatal(err)
	}
	var ret *Transaction
	for i := range txns {
		if txns[i].ID == txID {
			ret = &txns[i]
			break
		}
	}
	if ret == nil {
		t.Fatalf("expected return credit txn id=%d in buyer ledger", txID)
	}
	if ret.LeadViewable != ownerIsBuyer {
		t.Fatalf("lead_viewable = %v, want %v (buyer still owns lead = %v)", ret.LeadViewable, ownerIsBuyer, ownerIsBuyer)
	}
}

func TestListPublisherTransactions_stageEarningShowsAsOfBalance(t *testing.T) {
	billingSvc, _ := connectPublisherLedgerTest(t)
	ctx := context.Background()

	var publisherID, earningID int64
	var asOfBalance float64
	err := billingSvc.pool.QueryRow(ctx,
		`SELECT c.publisher_id, ce.id, b.balance_after::float8
		 FROM compensation_earnings ce
		 JOIN contract_compensations cc ON cc.id = ce.compensation_id
		 JOIN contracts c ON c.id = cc.contract_id AND c.deleted_at IS NULL
		 JOIN leads l ON l.id = ce.lead_id
		 LEFT JOIN contract_participations cp
		   ON cp.contract_id = c.id AND cp.buyer_id = l.owner_account_id AND cp.status = 'active'
		 JOIN LATERAL (
		   SELECT bt.balance_after::float8 AS balance_after
		   FROM transactions bt
		   WHERE bt.buyer_id = COALESCE(c.buyer_id, cp.buyer_id, l.owner_account_id)
		     AND bt.balance_after IS NOT NULL
		     AND bt.created_at <= ce.created_at
		   ORDER BY bt.created_at DESC, bt.id DESC
		   LIMIT 1
		 ) b ON true
		 WHERE ce.kind = 'stage'
		 LIMIT 1`).Scan(&publisherID, &earningID, &asOfBalance)
	if err != nil {
		t.Skip("no stage earning with prior buyer transaction in database")
	}

	txns, err := billingSvc.ListPublisherTransactions(ctx, publisherID, 0, "")
	if err != nil {
		t.Fatal(err)
	}
	var stage *Transaction
	for i := range txns {
		if txns[i].LedgerSource == "earning" && txns[i].ID == earningID {
			stage = &txns[i]
			break
		}
	}
	if stage == nil {
		t.Fatal("expected Stage earning row")
	}
	if stage.BalanceAfter == nil {
		t.Fatal("expected as-of buyer balance_after on stage earning row")
	}
	if *stage.BalanceAfter != asOfBalance {
		t.Fatalf("stage balance_after = %v, want %v (as-of buyer balance)", *stage.BalanceAfter, asOfBalance)
	}
}
