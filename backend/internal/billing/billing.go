// Package billing manages buyer balances, transactions and disputes.
package billing

import (
	"context"
	"errors"
	"time"

	"github.com/echayko/leadrula/backend/internal/accounts"
	"github.com/echayko/leadrula/backend/internal/contracts"
	"github.com/echayko/leadrula/backend/internal/database"
	"github.com/echayko/leadrula/backend/internal/notifications"
	stripeClient "github.com/echayko/leadrula/backend/internal/stripe"
	"github.com/echayko/leadrula/backend/pkg/httpx"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Transaction struct {
	ID                      int64     `json:"id"`
	PublicID                string    `json:"public_id"`
	BuyerID                 int64     `json:"buyer_id"`
	LeadID                  *int64    `json:"lead_id"`
	LeadName                *string   `json:"lead_name,omitempty"`
	BuyerName               *string   `json:"buyer_name,omitempty"`
	PublisherName           *string   `json:"publisher_name,omitempty"`
	ContractID              *int64    `json:"contract_id"`
	Type                    string    `json:"type"`
	Side                    string    `json:"side,omitempty"`
	Category                string    `json:"category,omitempty"`
	CounterpartyName        *string   `json:"counterparty_name,omitempty"`
	CounterpartyAccountType *string   `json:"counterparty_account_type,omitempty"`
	LedgerSource            string    `json:"ledger_source,omitempty"`
	Amount                  float64   `json:"amount"`
	BalanceAfter            *float64  `json:"balance_after"`
	Description             string    `json:"description"`
	CreatedAt               time.Time `json:"created_at"`
}

type Dispute struct {
	ID                      int64     `json:"id"`
	TransactionID           int64     `json:"transaction_id"`
	BuyerID                 int64     `json:"buyer_id"`
	BuyerName               string    `json:"buyer_name,omitempty"`
	CounterpartyName        string    `json:"counterparty_name,omitempty"`
	CounterpartyAccountType string    `json:"counterparty_account_type,omitempty"`
	Reason                  string    `json:"reason"`
	Status                  string    `json:"status"`
	Amount                  float64   `json:"amount,omitempty"`
	CreatedAt               time.Time `json:"created_at"`
}

type Service struct {
	pool                *pgxpool.Pool
	notif               *notifications.Service
	accounts            *accounts.Repository
	stripe              *stripeClient.Client
	encKey              []byte
	stripeOAuthRedirect string
}

func NewService(pool *pgxpool.Pool, notif *notifications.Service, acc *accounts.Repository, sc *stripeClient.Client, encKey []byte, stripeOAuthRedirect string) *Service {
	return &Service{
		pool: pool, notif: notif, accounts: acc, stripe: sc,
		encKey: encKey, stripeOAuthRedirect: stripeOAuthRedirect,
	}
}

// EnsureBalance creates a zero balance row if none exists.
func EnsureBalance(ctx context.Context, q database.Querier, buyerID int64) error {
	_, err := q.Exec(ctx,
		`INSERT INTO buyer_balances(buyer_id, balance) VALUES ($1, 0)
		 ON CONFLICT (buyer_id) DO NOTHING`, buyerID)
	return err
}

// Debit charges the buyer for a distributed lead. Locks the balance row
// (FOR UPDATE) to serialize concurrent debits. Allows negative balances.
// Must run inside the caller's transaction (q is a pgx.Tx).
func Debit(ctx context.Context, q database.Querier, buyerID int64, amount float64, leadID, contractID int64, desc string) error {
	if err := EnsureBalance(ctx, q, buyerID); err != nil {
		return err
	}
	var balance float64
	if err := q.QueryRow(ctx,
		`SELECT balance::float8 FROM buyer_balances WHERE buyer_id = $1 FOR UPDATE`, buyerID).Scan(&balance); err != nil {
		return err
	}
	newBal := balance - amount
	if _, err := q.Exec(ctx,
		`UPDATE buyer_balances SET balance = $2 WHERE buyer_id = $1`, buyerID, newBal); err != nil {
		return err
	}
	_, err := q.Exec(ctx,
		`INSERT INTO transactions(buyer_id, lead_id, contract_id, type, amount, balance_after, description)
		 VALUES ($1, NULLIF($2,0), NULLIF($3,0), 'debit', $4, $5, $6)`,
		buyerID, leadID, contractID, -amount, newBal, desc)
	return err
}

// Credit refunds the buyer inside the caller's transaction (mirror of Debit).
func Credit(ctx context.Context, q database.Querier, buyerID int64, amount float64, leadID, contractID int64, desc string) error {
	if amount <= 0 {
		return nil
	}
	if err := EnsureBalance(ctx, q, buyerID); err != nil {
		return err
	}
	var balance float64
	if err := q.QueryRow(ctx,
		`SELECT balance::float8 FROM buyer_balances WHERE buyer_id = $1 FOR UPDATE`, buyerID).Scan(&balance); err != nil {
		return err
	}
	newBal := balance + amount
	if _, err := q.Exec(ctx,
		`UPDATE buyer_balances SET balance = $2 WHERE buyer_id = $1`, buyerID, newBal); err != nil {
		return err
	}
	_, err := q.Exec(ctx,
		`INSERT INTO transactions(buyer_id, lead_id, contract_id, type, amount, balance_after, description)
		 VALUES ($1, NULLIF($2,0), NULLIF($3,0), 'credit', $4, $5, $6)`,
		buyerID, leadID, contractID, amount, newBal, desc)
	return err
}

// DistributeDebitAmount returns the absolute value of the latest distribute debit for a lead.
func DistributeDebitAmount(ctx context.Context, q database.Querier, buyerID, leadID, contractID int64) (float64, error) {
	var amount *float64
	err := q.QueryRow(ctx,
		`SELECT ABS(amount::float8) FROM transactions
		 WHERE buyer_id = $1 AND lead_id = $2 AND contract_id = $3 AND type = 'debit' AND amount < 0
		 ORDER BY created_at DESC LIMIT 1`,
		buyerID, leadID, contractID).Scan(&amount)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	if amount == nil {
		return 0, nil
	}
	return *amount, nil
}

// ReturnCreditExists reports whether this lead was already refunded on return.
func ReturnCreditExists(ctx context.Context, q database.Querier, buyerID, leadID, contractID int64) (bool, error) {
	var ok bool
	err := q.QueryRow(ctx,
		`SELECT EXISTS(
		   SELECT 1 FROM transactions
		   WHERE buyer_id = $1 AND lead_id = $2 AND contract_id = $3
		     AND type = 'credit' AND description = 'lead returned'
		 )`, buyerID, leadID, contractID).Scan(&ok)
	return ok, err
}

func (s *Service) GetBalance(ctx context.Context, buyerID int64) (float64, error) {
	if err := EnsureBalance(ctx, s.pool, buyerID); err != nil {
		return 0, err
	}
	var bal float64
	err := s.pool.QueryRow(ctx, `SELECT balance::float8 FROM buyer_balances WHERE buyer_id = $1`, buyerID).Scan(&bal)
	return bal, err
}

// Topup records a credit and increases the balance.
func (s *Service) Topup(ctx context.Context, buyerID int64, amount float64, desc string) (*Transaction, error) {
	if amount <= 0 {
		return nil, httpx.Validation("amount must be positive")
	}
	return s.creditTxn(ctx, buyerID, amount, "credit", desc)
}

func (s *Service) creditTxn(ctx context.Context, buyerID int64, amount float64, ttype, desc string) (*Transaction, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	if err := EnsureBalance(ctx, tx, buyerID); err != nil {
		return nil, err
	}
	var balance float64
	if err := tx.QueryRow(ctx, `SELECT balance::float8 FROM buyer_balances WHERE buyer_id=$1 FOR UPDATE`, buyerID).Scan(&balance); err != nil {
		return nil, err
	}
	newBal := balance + amount
	if _, err := tx.Exec(ctx, `UPDATE buyer_balances SET balance=$2 WHERE buyer_id=$1`, buyerID, newBal); err != nil {
		return nil, err
	}
	t, err := insertTxn(ctx, tx, buyerID, ttype, amount, newBal, desc)
	if err != nil {
		return nil, err
	}
	return t, tx.Commit(ctx)
}

func insertTxn(ctx context.Context, q database.Querier, buyerID int64, ttype string, amount, balanceAfter float64, desc string) (*Transaction, error) {
	t := &Transaction{BalanceAfter: &balanceAfter}
	err := q.QueryRow(ctx,
		`INSERT INTO transactions(buyer_id, type, amount, balance_after, description)
		 VALUES ($1,$2,$3,$4,$5)
		 RETURNING id, public_id, buyer_id, lead_id, contract_id, type, amount::float8, balance_after::float8, description, created_at`,
		buyerID, ttype, amount, balanceAfter, desc).Scan(
		&t.ID, &t.PublicID, &t.BuyerID, &t.LeadID, &t.ContractID, &t.Type, &t.Amount, &t.BalanceAfter, &t.Description, &t.CreatedAt)
	return t, err
}

// ListTransactions lists a buyer's ledger; if buyerID is 0, lists all (publisher).
func (s *Service) ListTransactions(ctx context.Context, buyerID int64, txType string) ([]Transaction, error) {
	q := `SELECT t.id, t.public_id, t.buyer_id, t.lead_id, t.contract_id, t.type,
	             t.amount::float8, t.balance_after::float8, t.description, t.created_at,
	             NULLIF(trim(coalesce(l.first_name,'') || ' ' || coalesce(l.last_name,'')), ''),
	             NULLIF(trim(buyer.name), ''),
	             NULLIF(COALESCE(pub_c.name, pub_l.name), '')
	      FROM transactions t
	      LEFT JOIN leads l ON l.id = t.lead_id
	      LEFT JOIN accounts buyer ON buyer.id = t.buyer_id
	      LEFT JOIN contracts c ON c.id = t.contract_id
	      LEFT JOIN accounts pub_c ON pub_c.id = c.publisher_id
	      LEFT JOIN accounts pub_l ON pub_l.id = l.publisher_id
	      WHERE ($1 = 0 OR t.buyer_id = $1)
	        AND ($2 = '' OR t.type = $2::txn_type)
	      ORDER BY t.created_at DESC LIMIT 500`
	rows, err := s.pool.Query(ctx, q, buyerID, txType)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Transaction
	for rows.Next() {
		var t Transaction
		var bal float64
		if err := rows.Scan(&t.ID, &t.PublicID, &t.BuyerID, &t.LeadID, &t.ContractID, &t.Type,
			&t.Amount, &bal, &t.Description, &t.CreatedAt, &t.LeadName, &t.BuyerName, &t.PublisherName); err != nil {
			return nil, err
		}
		t.BalanceAfter = &bal
		out = append(out, t)
	}
	return out, rows.Err()
}

const publisherTxnScope = `
  (
    c.publisher_id = $1
    OR (t.buyer_id = $1 AND c.publisher_id IS NOT NULL AND c.publisher_id <> $1)
    OR (
      t.type IN ('credit', 'topup', 'dispute_credit', 'manual_invoice', 'compensation_payout')
      AND EXISTS (
        SELECT 1 FROM partnerships p
        WHERE p.publisher_id = $1 AND p.buyer_id = t.buyer_id AND p.status = 'active'
      )
    )
  )`

const txnSideExpr = `
  CASE
    WHEN t.type IN ('credit', 'topup', 'dispute_credit')
      AND EXISTS (
        SELECT 1 FROM partnerships p
        WHERE p.publisher_id = $1 AND p.buyer_id = t.buyer_id AND p.status = 'active'
      ) THEN 'prepay'
    WHEN c.publisher_id = $1 THEN 'sale'
    WHEN t.buyer_id = $1 THEN 'purchase'
    WHEN t.type IN ('manual_invoice', 'compensation_payout')
      AND EXISTS (
        SELECT 1 FROM partnerships p
        WHERE p.publisher_id = $1 AND p.buyer_id = t.buyer_id AND p.status = 'active'
      ) THEN 'sale'
    ELSE ''
  END`

const txnCategoryExpr = `
  CASE
    WHEN ` + txnSideExpr + ` = 'prepay' AND t.type = 'topup' THEN 'Topup'
    WHEN ` + txnSideExpr + ` = 'prepay' AND t.type = 'credit' THEN 'Credit'
    WHEN ` + txnSideExpr + ` = 'prepay' AND t.type = 'dispute_credit' THEN 'Refund'
    WHEN ` + txnSideExpr + ` = 'sale' AND t.type = 'manual_invoice' THEN 'Invoice'
    WHEN ` + txnSideExpr + ` = 'sale' AND t.type = 'compensation_payout' THEN 'Compensation payout'
    WHEN ` + txnSideExpr + ` = 'sale' THEN 'Sale'
    WHEN ` + txnSideExpr + ` = 'purchase' THEN 'Purchase'
    ELSE ''
  END`

const txnCounterpartyNameExpr = `
  CASE
    WHEN ` + txnSideExpr + ` IN ('prepay', 'sale') THEN NULLIF(trim(buyer.name), '')
    WHEN ` + txnSideExpr + ` = 'purchase' THEN NULLIF(trim(pub_c.name), '')
  END`

const txnCounterpartyTypeExpr = `
  CASE
    WHEN ` + txnSideExpr + ` IN ('prepay', 'sale') THEN buyer.type::text
    WHEN ` + txnSideExpr + ` = 'purchase' THEN 'publisher'
  END`

const earningCategoryExpr = `
  CASE ce.kind
    WHEN 'distribute' THEN 'Sale'
    WHEN 'return' THEN 'Return'
    WHEN 'dispute' THEN 'Dispute'
    WHEN 'stage' THEN 'Stage'
    ELSE ''
  END`

const publisherTxnFilter = `
  (
    (` + txnSideExpr + ` = 'prepay' AND t.type IN ('topup', 'manual_invoice'))
    OR (` + txnSideExpr + ` = 'purchase' AND t.type = 'debit')
    OR (` + txnSideExpr + ` = 'sale' AND t.type = 'manual_invoice')
  )`

const noDistributeEarning = `
  NOT EXISTS (
    SELECT 1 FROM compensation_earnings ce
    JOIN contract_compensations cc ON cc.id = ce.compensation_id
    WHERE ce.lead_id = t.lead_id AND cc.contract_id = t.contract_id
      AND ce.kind = 'distribute'
  )`

const noReturnEarning = `
  NOT EXISTS (
    SELECT 1 FROM compensation_earnings ce
    JOIN contract_compensations cc ON cc.id = ce.compensation_id
    WHERE ce.lead_id = t.lead_id AND cc.contract_id = t.contract_id
      AND ce.kind = 'return'
  )`

const noDisputeEarning = `
  NOT EXISTS (
    SELECT 1 FROM compensation_earnings ce
    JOIN contract_compensations cc ON cc.id = ce.compensation_id
    WHERE ce.lead_id = orig.lead_id AND cc.contract_id = orig.contract_id
      AND ce.kind = 'dispute'
  )`

const legacyCategoryExpr = `
  CASE
    WHEN t.type = 'debit' THEN 'Sale'
    WHEN t.type = 'credit' AND t.description = 'lead returned' THEN 'Return'
    WHEN t.type = 'dispute_credit' THEN 'Dispute'
    ELSE ''
  END`

const legacyAmountExpr = `
  CASE
    WHEN t.type = 'debit' THEN ABS(t.amount::float8)
    WHEN t.type IN ('credit', 'dispute_credit') THEN -ABS(t.amount::float8)
    ELSE t.amount::float8
  END`

// ListPublisherTransactions lists publisher-centric activity: lead earnings, buyer prepay, and publisher purchases.
func (s *Service) ListPublisherTransactions(ctx context.Context, publisherID int64, filterBuyerID int64, txType string) ([]Transaction, error) {
	q := `
	  SELECT * FROM (
	    SELECT ce.id,
	           ''::text AS public_id,
	           c.buyer_id,
	           ce.lead_id,
	           cc.contract_id,
	           ce.kind::text AS type,
	           ce.amount::float8,
	           NULL::float8 AS balance_after,
	           ''::text AS description,
	           ce.created_at,
	           NULLIF(trim(coalesce(l.first_name,'') || ' ' || coalesce(l.last_name,'')), ''),
	           NULLIF(trim(buyer.name), ''),
	           NULL::text AS publisher_name,
	           'sale'::text AS side,
	           ` + earningCategoryExpr + ` AS category,
	           NULLIF(trim(buyer.name), '') AS counterparty_name,
	           buyer.type::text AS counterparty_account_type,
	           'earning'::text AS ledger_source
	    FROM compensation_earnings ce
	    JOIN contract_compensations cc ON cc.id = ce.compensation_id
	    JOIN contracts c ON c.id = cc.contract_id AND c.publisher_id = $1 AND c.deleted_at IS NULL
	    JOIN accounts buyer ON buyer.id = c.buyer_id
	    LEFT JOIN leads l ON l.id = ce.lead_id
	    WHERE ce.kind IN ('distribute', 'return', 'dispute', 'stage')
	      AND ($2 = 0 OR c.buyer_id = $2)

	    UNION ALL

	    SELECT t.id,
	           t.public_id::text,
	           t.buyer_id,
	           t.lead_id,
	           t.contract_id,
	           t.type::text,
	           t.amount::float8,
	           t.balance_after::float8,
	           t.description,
	           t.created_at,
	           NULLIF(trim(coalesce(l.first_name,'') || ' ' || coalesce(l.last_name,'')), ''),
	           NULLIF(trim(buyer.name), ''),
	           NULLIF(COALESCE(pub_c.name, pub_l.name), ''),
	           ` + txnSideExpr + `,
	           ` + txnCategoryExpr + `,
	           ` + txnCounterpartyNameExpr + `,
	           ` + txnCounterpartyTypeExpr + `,
	           'transaction'::text AS ledger_source
	    FROM transactions t
	    LEFT JOIN leads l ON l.id = t.lead_id
	    LEFT JOIN accounts buyer ON buyer.id = t.buyer_id
	    LEFT JOIN contracts c ON c.id = t.contract_id
	    LEFT JOIN accounts pub_c ON pub_c.id = c.publisher_id
	    LEFT JOIN accounts pub_l ON pub_l.id = l.publisher_id
	    WHERE ` + publisherTxnScope + `
	      AND ($2 = 0 OR t.buyer_id = $2)
	      AND ($3 = '' OR t.type = $3::txn_type)
	      AND ` + publisherTxnFilter + `

	    UNION ALL

	    SELECT t.id,
	           t.public_id::text,
	           t.buyer_id,
	           t.lead_id,
	           t.contract_id,
	           t.type::text,
	           ` + legacyAmountExpr + `,
	           NULL::float8 AS balance_after,
	           t.description,
	           t.created_at,
	           NULLIF(trim(coalesce(l.first_name,'') || ' ' || coalesce(l.last_name,'')), ''),
	           NULLIF(trim(buyer.name), ''),
	           NULLIF(COALESCE(pub_c.name, pub_l.name), ''),
	           'sale'::text AS side,
	           ` + legacyCategoryExpr + `,
	           NULLIF(trim(buyer.name), '') AS counterparty_name,
	           buyer.type::text AS counterparty_account_type,
	           'legacy'::text AS ledger_source
	    FROM transactions t
	    LEFT JOIN leads l ON l.id = t.lead_id
	    LEFT JOIN accounts buyer ON buyer.id = t.buyer_id
	    LEFT JOIN contracts c ON c.id = t.contract_id
	    LEFT JOIN accounts pub_c ON pub_c.id = c.publisher_id
	    LEFT JOIN accounts pub_l ON pub_l.id = l.publisher_id
	    WHERE c.publisher_id = $1 AND c.deleted_at IS NULL
	      AND ($2 = 0 OR t.buyer_id = $2)
	      AND ($3 = '' OR t.type = $3::txn_type)
	      AND (
	        (t.type = 'debit' AND t.amount < 0 AND t.lead_id IS NOT NULL AND ` + noDistributeEarning + `)
	        OR (t.type = 'credit' AND t.description = 'lead returned' AND t.lead_id IS NOT NULL AND ` + noReturnEarning + `)
	      )

	    UNION ALL

	    SELECT t.id,
	           t.public_id::text,
	           t.buyer_id,
	           orig.lead_id,
	           orig.contract_id,
	           t.type::text,
	           -ABS(t.amount::float8),
	           NULL::float8 AS balance_after,
	           t.description,
	           t.created_at,
	           NULLIF(trim(coalesce(l.first_name,'') || ' ' || coalesce(l.last_name,'')), ''),
	           NULLIF(trim(buyer.name), ''),
	           NULLIF(COALESCE(pub_c.name, pub_l.name), ''),
	           'sale'::text AS side,
	           'Dispute'::text AS category,
	           NULLIF(trim(buyer.name), '') AS counterparty_name,
	           buyer.type::text AS counterparty_account_type,
	           'legacy'::text AS ledger_source
	    FROM transactions t
	    JOIN disputes d ON d.buyer_id = t.buyer_id AND d.status = 'accepted'
	    JOIN transactions orig ON orig.id = d.transaction_id
	    LEFT JOIN leads l ON l.id = orig.lead_id
	    LEFT JOIN accounts buyer ON buyer.id = t.buyer_id
	    LEFT JOIN contracts c ON c.id = orig.contract_id
	    LEFT JOIN accounts pub_c ON pub_c.id = c.publisher_id
	    LEFT JOIN accounts pub_l ON pub_l.id = l.publisher_id
	    WHERE t.type = 'dispute_credit'
	      AND c.publisher_id = $1 AND c.deleted_at IS NULL
	      AND orig.lead_id IS NOT NULL
	      AND ($2 = 0 OR t.buyer_id = $2)
	      AND ($3 = '' OR t.type = $3::txn_type)
	      AND ` + noDisputeEarning + `
	  ) combined
	  ORDER BY created_at DESC
	  LIMIT 500`
	rows, err := s.pool.Query(ctx, q, publisherID, filterBuyerID, txType)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Transaction
	for rows.Next() {
		var t Transaction
		if err := rows.Scan(&t.ID, &t.PublicID, &t.BuyerID, &t.LeadID, &t.ContractID, &t.Type,
			&t.Amount, &t.BalanceAfter, &t.Description, &t.CreatedAt, &t.LeadName, &t.BuyerName, &t.PublisherName,
			&t.Side, &t.Category, &t.CounterpartyName, &t.CounterpartyAccountType, &t.LedgerSource); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// ListPublisherDisputes lists open disputes on this publisher's contract sales.
func (s *Service) ListPublisherDisputes(ctx context.Context, publisherID int64, status string) ([]Dispute, error) {
	q := `SELECT d.id, d.transaction_id, d.buyer_id, d.reason, d.status, d.created_at,
	             abs(t.amount)::float8,
	             NULLIF(trim(buyer.name), ''),
	             buyer.type::text
	      FROM disputes d
	      JOIN transactions t ON t.id = d.transaction_id
	      JOIN accounts buyer ON buyer.id = d.buyer_id
	      LEFT JOIN contracts c ON c.id = t.contract_id
	      WHERE (
	        c.publisher_id = $1
	        OR (
	          t.type = 'manual_invoice'
	          AND EXISTS (
	            SELECT 1 FROM partnerships p
	            WHERE p.publisher_id = $1 AND p.buyer_id = d.buyer_id AND p.status = 'active'
	          )
	        )
	      )
	        AND ($2 = '' OR d.status = $2::dispute_status)
	      ORDER BY d.created_at DESC`
	rows, err := s.pool.Query(ctx, q, publisherID, status)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Dispute
	for rows.Next() {
		var d Dispute
		var cpName *string
		var cpType *string
		if err := rows.Scan(&d.ID, &d.TransactionID, &d.BuyerID, &d.Reason, &d.Status, &d.CreatedAt,
			&d.Amount, &cpName, &cpType); err != nil {
			return nil, err
		}
		if cpName != nil {
			d.BuyerName = *cpName
			d.CounterpartyName = *cpName
		}
		if cpType != nil {
			d.CounterpartyAccountType = *cpType
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

// ── Disputes ──────────────────────────────────────────────────────

func (s *Service) OpenDispute(ctx context.Context, buyerID, transactionID int64, reason string) (*Dispute, error) {
	// verify the transaction belongs to the buyer and is a debit
	var ttype string
	err := s.pool.QueryRow(ctx, `SELECT type FROM transactions WHERE id=$1 AND buyer_id=$2`, transactionID, buyerID).Scan(&ttype)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, httpx.NotFound("transaction not found")
	}
	if err != nil {
		return nil, err
	}
	if ttype != "debit" {
		return nil, httpx.BusinessRule("only debit transactions can be disputed")
	}
	d := &Dispute{}
	err = s.pool.QueryRow(ctx,
		`INSERT INTO disputes(transaction_id, buyer_id, reason) VALUES ($1,$2,$3)
		 RETURNING id, transaction_id, buyer_id, reason, status, created_at`,
		transactionID, buyerID, reason).Scan(&d.ID, &d.TransactionID, &d.BuyerID, &d.Reason, &d.Status, &d.CreatedAt)
	return d, err
}

func (s *Service) ListDisputes(ctx context.Context, buyerID int64, status string) ([]Dispute, error) {
	q := `SELECT d.id, d.transaction_id, d.buyer_id, d.reason, d.status, d.created_at,
	             abs(t.amount)::float8, a.name
	      FROM disputes d
	      JOIN transactions t ON t.id = d.transaction_id
	      JOIN accounts a ON a.id = d.buyer_id
	      WHERE ($1 = 0 OR d.buyer_id = $1)
	        AND ($2 = '' OR d.status = $2::dispute_status)
	      ORDER BY d.created_at DESC`
	rows, err := s.pool.Query(ctx, q, buyerID, status)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Dispute
	for rows.Next() {
		var d Dispute
		if err := rows.Scan(&d.ID, &d.TransactionID, &d.BuyerID, &d.Reason, &d.Status, &d.CreatedAt, &d.Amount, &d.BuyerName); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

// AcceptDispute credits the buyer the disputed amount and marks it accepted.
func (s *Service) AcceptDispute(ctx context.Context, disputeID, adminID int64) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	var buyerID, txnID int64
	var status string
	var amount float64
	var leadID, contractID *int64
	err = tx.QueryRow(ctx,
		`SELECT d.buyer_id, d.transaction_id, d.status, abs(t.amount)::float8, t.lead_id, t.contract_id
		 FROM disputes d JOIN transactions t ON t.id = d.transaction_id
		 WHERE d.id = $1 FOR UPDATE`, disputeID).Scan(&buyerID, &txnID, &status, &amount, &leadID, &contractID)
	if errors.Is(err, pgx.ErrNoRows) {
		return httpx.NotFound("dispute not found")
	}
	if err != nil {
		return err
	}
	if status != "open" {
		return httpx.BusinessRule("dispute already resolved")
	}

	var balance float64
	if err := tx.QueryRow(ctx, `SELECT balance::float8 FROM buyer_balances WHERE buyer_id=$1 FOR UPDATE`, buyerID).Scan(&balance); err != nil {
		return err
	}
	newBal := balance + amount
	if _, err := tx.Exec(ctx, `UPDATE buyer_balances SET balance=$2 WHERE buyer_id=$1`, buyerID, newBal); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx,
		`INSERT INTO transactions(buyer_id, type, amount, balance_after, description)
		 VALUES ($1,'dispute_credit',$2,$3,$4)`,
		buyerID, amount, newBal, "dispute accepted"); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx,
		`UPDATE disputes SET status='accepted', resolved_by=$2, resolved_at=now() WHERE id=$1`,
		disputeID, adminID); err != nil {
		return err
	}
	emails, err := s.notifyBuyerAdmins(ctx, tx, buyerID, "accepted")
	if err != nil {
		return err
	}
	if err := contracts.RecordEarningDispute(ctx, tx, txnID, leadID, contractID, amount); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return err
	}
	s.notif.SendEmails(emails)
	return nil
}

func (s *Service) RejectDispute(ctx context.Context, disputeID, adminID int64) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	var buyerID int64
	var status string
	err = tx.QueryRow(ctx, `SELECT buyer_id, status FROM disputes WHERE id=$1 FOR UPDATE`, disputeID).Scan(&buyerID, &status)
	if errors.Is(err, pgx.ErrNoRows) {
		return httpx.NotFound("dispute not found")
	}
	if err != nil {
		return err
	}
	if status != "open" {
		return httpx.BusinessRule("dispute already resolved")
	}
	if _, err := tx.Exec(ctx,
		`UPDATE disputes SET status='rejected', resolved_by=$2, resolved_at=now() WHERE id=$1`,
		disputeID, adminID); err != nil {
		return err
	}
	emails, err := s.notifyBuyerAdmins(ctx, tx, buyerID, "rejected")
	if err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return err
	}
	s.notif.SendEmails(emails)
	return nil
}

func (s *Service) notifyBuyerAdmins(ctx context.Context, q database.Querier, buyerID int64, outcome string) ([]notifications.EmailJob, error) {
	ids, err := s.accounts.AdminUserIDs(ctx, q, buyerID)
	if err != nil {
		return nil, err
	}
	return s.notif.Deliver(ctx, q, notifications.DeliverParams{
		AccountID: buyerID,
		UserIDs:   ids,
		EventType: "dispute_update",
		Payload:   map[string]any{"outcome": outcome},
	})
}
