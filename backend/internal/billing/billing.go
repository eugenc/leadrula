// Package billing manages buyer balances, transactions and disputes.
package billing

import (
	"context"
	"errors"
	"time"

	"github.com/echayko/leadrula/backend/internal/accounts"
	"github.com/echayko/leadrula/backend/internal/database"
	"github.com/echayko/leadrula/backend/internal/notifications"
	"github.com/echayko/leadrula/backend/pkg/httpx"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Transaction struct {
	ID           int64     `json:"id"`
	PublicID     string    `json:"public_id"`
	BuyerID      int64     `json:"buyer_id"`
	LeadID       *int64    `json:"lead_id"`
	LeadName     *string   `json:"lead_name,omitempty"`
	ContractID   *int64    `json:"contract_id"`
	Type         string    `json:"type"`
	Amount       float64   `json:"amount"`
	BalanceAfter float64   `json:"balance_after"`
	Description  string    `json:"description"`
	CreatedAt    time.Time `json:"created_at"`
}

type Dispute struct {
	ID            int64     `json:"id"`
	TransactionID int64     `json:"transaction_id"`
	BuyerID       int64     `json:"buyer_id"`
	BuyerName     string    `json:"buyer_name,omitempty"`
	Reason        string    `json:"reason"`
	Status        string    `json:"status"`
	Amount        float64   `json:"amount,omitempty"`
	CreatedAt     time.Time `json:"created_at"`
}

type Service struct {
	pool     *pgxpool.Pool
	notif    *notifications.Service
	accounts *accounts.Repository
}

func NewService(pool *pgxpool.Pool, notif *notifications.Service, acc *accounts.Repository) *Service {
	return &Service{pool: pool, notif: notif, accounts: acc}
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

// ManualInvoice records a publisher-created debit/charge against a buyer.
func (s *Service) ManualInvoice(ctx context.Context, buyerID int64, amount float64, desc string) (*Transaction, error) {
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
	newBal := balance - amount
	if _, err := tx.Exec(ctx, `UPDATE buyer_balances SET balance=$2 WHERE buyer_id=$1`, buyerID, newBal); err != nil {
		return nil, err
	}
	t, err := insertTxn(ctx, tx, buyerID, "manual_invoice", -amount, newBal, desc)
	if err != nil {
		return nil, err
	}
	return t, tx.Commit(ctx)
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
	t := &Transaction{}
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
	             NULLIF(trim(coalesce(l.first_name,'') || ' ' || coalesce(l.last_name,'')), '')
	      FROM transactions t LEFT JOIN leads l ON l.id = t.lead_id
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
		if err := rows.Scan(&t.ID, &t.PublicID, &t.BuyerID, &t.LeadID, &t.ContractID, &t.Type,
			&t.Amount, &t.BalanceAfter, &t.Description, &t.CreatedAt, &t.LeadName); err != nil {
			return nil, err
		}
		out = append(out, t)
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
	err = tx.QueryRow(ctx,
		`SELECT d.buyer_id, d.transaction_id, d.status, abs(t.amount)::float8
		 FROM disputes d JOIN transactions t ON t.id = d.transaction_id
		 WHERE d.id = $1 FOR UPDATE`, disputeID).Scan(&buyerID, &txnID, &status, &amount)
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
	if err := s.notifyBuyerAdmins(ctx, tx, buyerID, "accepted"); err != nil {
		return err
	}
	return tx.Commit(ctx)
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
	if err := s.notifyBuyerAdmins(ctx, tx, buyerID, "rejected"); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *Service) notifyBuyerAdmins(ctx context.Context, q database.Querier, buyerID int64, outcome string) error {
	ids, err := s.accounts.AdminUserIDs(ctx, q, buyerID)
	if err != nil {
		return err
	}
	return s.notif.Enqueue(ctx, q, ids, "dispute_update", map[string]any{"outcome": outcome})
}
