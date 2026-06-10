package billing

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/echayko/leadrula/backend/internal/accounts"
	"github.com/echayko/leadrula/backend/internal/notifications"
	stripeClient "github.com/echayko/leadrula/backend/internal/stripe"
	"github.com/echayko/leadrula/backend/pkg/httpx"
	"github.com/jackc/pgx/v5"
)

const (
	InvoiceKindStartingBalance = "starting_balance"
	InvoiceKindPrepayRequest   = "prepay_request"
)

var manualPaymentMethods = map[string]bool{
	"bank_transfer": true,
	"check":         true,
	"cash":          true,
	"other_digital": true,
	"other":         true,
}

type Invoice struct {
	ID              int64      `json:"id"`
	PublicID        string     `json:"public_id"`
	PublisherID     int64      `json:"publisher_id"`
	BuyerID         int64      `json:"buyer_id"`
	BuyerName       *string    `json:"buyer_name,omitempty"`
	PublisherName   *string    `json:"publisher_name,omitempty"`
	Amount          float64    `json:"amount"`
	Description     string     `json:"description"`
	Kind            string     `json:"kind"`
	Status          string     `json:"status"`
	PaymentMethod   *string    `json:"payment_method,omitempty"`
	PaymentNote     *string    `json:"payment_note,omitempty"`
	PaidAt          *time.Time `json:"paid_at,omitempty"`
	CreatedAt       time.Time  `json:"created_at"`
	OnlinePayable   *bool      `json:"online_payable,omitempty"`
}

const invoiceCols = `i.id, i.public_id, i.publisher_id, i.buyer_id,
	i.amount::float8, i.description, i.kind, i.status::text,
	i.payment_method::text, i.payment_note, i.paid_at, i.created_at,
	NULLIF(trim(buyer.name), ''), NULLIF(trim(pub.name), '')`

func scanInvoice(row pgx.Row) (*Invoice, error) {
	inv := &Invoice{}
	var pm, note *string
	err := row.Scan(
		&inv.ID, &inv.PublicID, &inv.PublisherID, &inv.BuyerID,
		&inv.Amount, &inv.Description, &inv.Kind, &inv.Status,
		&pm, &note, &inv.PaidAt, &inv.CreatedAt,
		&inv.BuyerName, &inv.PublisherName,
	)
	if err != nil {
		return nil, err
	}
	inv.PaymentMethod = pm
	inv.PaymentNote = note
	return inv, nil
}

func scanBuyerInvoice(row pgx.Row) (*Invoice, error) {
	inv := &Invoice{}
	var pm, note *string
	var onlinePayable bool
	err := row.Scan(
		&inv.ID, &inv.PublicID, &inv.PublisherID, &inv.BuyerID,
		&inv.Amount, &inv.Description, &inv.Kind, &inv.Status,
		&pm, &note, &inv.PaidAt, &inv.CreatedAt,
		&inv.BuyerName, &inv.PublisherName, &onlinePayable,
	)
	if err != nil {
		return nil, err
	}
	inv.PaymentMethod = pm
	inv.PaymentNote = note
	inv.OnlinePayable = &onlinePayable
	return inv, nil
}

// PublisherStripeReady reports whether a publisher can collect prepay for direct buyers online.
func (s *Service) PublisherStripeReady(ctx context.Context, publisherID int64) error {
	return s.PublisherDirectStripeReady(ctx, publisherID)
}

func (s *Service) validateInvoicePartnership(ctx context.Context, publisherID, buyerID int64) error {
	var ok bool
	if err := s.pool.QueryRow(ctx,
		`SELECT EXISTS(
			SELECT 1 FROM partnerships
			WHERE publisher_id = $1 AND buyer_id = $2 AND status = 'active'
		)`, publisherID, buyerID).Scan(&ok); err != nil {
		return err
	}
	if !ok {
		return httpx.BusinessRule("no active partnership with this buyer")
	}
	return nil
}

// CreateInvoice opens a prepay invoice for a buyer. Balance is credited only when paid.
func (s *Service) CreateInvoice(ctx context.Context, publisherID, buyerID int64, amount float64, desc, kind string) (*Invoice, error) {
	desc = strings.TrimSpace(desc)
	kind = strings.TrimSpace(kind)
	if amount <= 0 {
		return nil, httpx.Validation("amount must be positive")
	}
	if kind != InvoiceKindStartingBalance && kind != InvoiceKindPrepayRequest {
		return nil, httpx.Validation("invalid invoice kind")
	}
	if kind == InvoiceKindPrepayRequest && desc == "" {
		return nil, httpx.Validation("description is required")
	}
	if kind == InvoiceKindStartingBalance && desc == "" {
		desc = "Starting balance"
	}
	if err := s.validateInvoicePartnership(ctx, publisherID, buyerID); err != nil {
		return nil, err
	}

	row := s.pool.QueryRow(ctx,
		`INSERT INTO invoices(publisher_id, buyer_id, amount, description, kind)
		 VALUES ($1, $2, $3, $4, $5)
		 RETURNING id, public_id, publisher_id, buyer_id,
		           amount::float8, description, kind, status::text,
		           NULL::invoice_payment_method, NULL::text, NULL::timestamptz, created_at,
		           NULL::text, NULL::text`,
		publisherID, buyerID, amount, desc, kind)

	inv := &Invoice{}
	var pm, note *string
	if err := row.Scan(
		&inv.ID, &inv.PublicID, &inv.PublisherID, &inv.BuyerID,
		&inv.Amount, &inv.Description, &inv.Kind, &inv.Status,
		&pm, &note, &inv.PaidAt, &inv.CreatedAt,
		&inv.BuyerName, &inv.PublisherName,
	); err != nil {
		return nil, err
	}

	if err := s.notifyInvoiceCreated(ctx, buyerID, inv); err != nil {
		log.Printf("invoice notify failed buyer=%d invoice=%d: %v", buyerID, inv.ID, err)
	}
	return inv, nil
}

func (s *Service) notifyInvoiceCreated(ctx context.Context, buyerID int64, inv *Invoice) error {
	ids, err := s.accounts.AdminUserIDs(ctx, s.pool, buyerID)
	if err != nil {
		return err
	}
	emails, err := s.notif.Deliver(ctx, s.pool, notifications.DeliverParams{
		AccountID: buyerID,
		UserIDs:   ids,
		EventType: "new_invoice",
		Payload: map[string]any{
			"invoice_id": inv.ID,
			"amount":     inv.Amount,
		},
	})
	if err != nil {
		return err
	}
	s.notif.SendEmails(emails)
	return nil
}

func (s *Service) ListPublisherInvoices(ctx context.Context, publisherID int64, status string) ([]Invoice, error) {
	q := `SELECT ` + invoiceCols + `
	      FROM invoices i
	      JOIN accounts buyer ON buyer.id = i.buyer_id
	      JOIN accounts pub ON pub.id = i.publisher_id
	      WHERE i.publisher_id = $1
	        AND ($2 = '' OR i.status = $2::invoice_status)
	      ORDER BY i.created_at DESC LIMIT 500`
	return s.queryInvoices(ctx, q, publisherID, status)
}

func (s *Service) ListBuyerInvoices(ctx context.Context, buyerID int64, status string) ([]Invoice, error) {
	const buyerInvoiceCols = invoiceCols + `,
		CASE
		  WHEN buyer.buyer_kind = 'marketplace' THEN true
		  WHEN pub.stripe_keys_status = 'verified' THEN true
		  ELSE false
		END`
	q := `SELECT ` + buyerInvoiceCols + `
	      FROM invoices i
	      JOIN accounts buyer ON buyer.id = i.buyer_id
	      JOIN accounts pub ON pub.id = i.publisher_id
	      WHERE i.buyer_id = $1
	        AND ($2 = '' OR i.status = $2::invoice_status)
	      ORDER BY i.created_at DESC LIMIT 500`
	rows, err := s.pool.Query(ctx, q, buyerID, status)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Invoice
	for rows.Next() {
		inv, err := scanBuyerInvoice(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *inv)
	}
	return out, rows.Err()
}

func (s *Service) queryInvoices(ctx context.Context, q string, id int64, status string) ([]Invoice, error) {
	rows, err := s.pool.Query(ctx, q, id, status)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Invoice
	for rows.Next() {
		inv, err := scanInvoice(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *inv)
	}
	return out, rows.Err()
}

func (s *Service) getInvoiceForBuyer(ctx context.Context, buyerID, invoiceID int64) (*Invoice, error) {
	inv, err := scanInvoice(s.pool.QueryRow(ctx,
		`SELECT `+invoiceCols+`
		 FROM invoices i
		 JOIN accounts buyer ON buyer.id = i.buyer_id
		 JOIN accounts pub ON pub.id = i.publisher_id
		 WHERE i.id = $1 AND i.buyer_id = $2`, invoiceID, buyerID))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, httpx.NotFound("invoice not found")
	}
	return inv, err
}

func (s *Service) getInvoiceForPublisher(ctx context.Context, publisherID, invoiceID int64) (*Invoice, error) {
	inv, err := scanInvoice(s.pool.QueryRow(ctx,
		`SELECT `+invoiceCols+`
		 FROM invoices i
		 JOIN accounts buyer ON buyer.id = i.buyer_id
		 JOIN accounts pub ON pub.id = i.publisher_id
		 WHERE i.id = $1 AND i.publisher_id = $2`, invoiceID, publisherID))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, httpx.NotFound("invoice not found")
	}
	return inv, err
}

// CreateInvoicePaymentIntent returns a Stripe client_secret for paying an open invoice.
func (s *Service) CreateInvoicePaymentIntent(ctx context.Context, buyerID, invoiceID int64) (*StripeIntentResult, error) {
	inv, err := s.getInvoiceForBuyer(ctx, buyerID, invoiceID)
	if err != nil {
		return nil, err
	}
	if inv.Status != "open" {
		return nil, httpx.BusinessRule("invoice is not open")
	}

	amountCents := int64(inv.Amount * 100)
	if amountCents < 50 {
		return nil, httpx.Validation("minimum invoice payment is $0.50")
	}

	var buyerPublicID string
	if err := s.pool.QueryRow(ctx,
		`SELECT public_id::text FROM accounts WHERE id = $1`, buyerID).Scan(&buyerPublicID); err != nil {
		return nil, err
	}

	var buyerKind string
	if err := s.pool.QueryRow(ctx,
		`SELECT buyer_kind::text FROM accounts WHERE id = $1`, buyerID).Scan(&buyerKind); err != nil {
		return nil, err
	}

	if buyerKind == accounts.BuyerKindDirect {
		if err := s.PublisherDirectStripeReady(ctx, inv.PublisherID); err != nil {
			return nil, err
		}
		secret, pk, err := s.publisherSecretKey(ctx, inv.PublisherID)
		if err != nil {
			return nil, err
		}
		custID, err := s.ensurePublisherStripeCustomer(ctx, buyerID, inv.PublisherID, secret)
		if err != nil {
			return nil, err
		}
		piID, clientSecret, err := stripeClient.CreatePublisherInvoicePaymentIntent(
			secret, amountCents, custID, inv.PublicID, buyerPublicID)
		if err != nil {
			return nil, err
		}
		if _, err := s.pool.Exec(ctx,
			`UPDATE invoices SET stripe_payment_intent_id = $2 WHERE id = $1 AND status = 'open'`,
			invoiceID, piID); err != nil {
			return nil, err
		}
		return &StripeIntentResult{ClientSecret: clientSecret, PublishableKey: pk}, nil
	}

	if err := s.requireStripe(); err != nil {
		return nil, err
	}
	custID, err := s.ensureStripeCustomer(ctx, buyerID)
	if err != nil {
		return nil, err
	}
	piID, clientSecret, err := s.stripe.CreateInvoicePaymentIntent(
		amountCents, custID, inv.PublicID, buyerPublicID, nil)
	if err != nil {
		return nil, err
	}
	if _, err := s.pool.Exec(ctx,
		`UPDATE invoices SET stripe_payment_intent_id = $2 WHERE id = $1 AND status = 'open'`,
		invoiceID, piID); err != nil {
		return nil, err
	}
	return &StripeIntentResult{ClientSecret: clientSecret}, nil
}

// ConfirmDirectInvoicePayment verifies a publisher-account invoice PI and settles the invoice.
func (s *Service) ConfirmDirectInvoicePayment(ctx context.Context, buyerID, invoiceID int64, piID string) error {
	inv, err := s.getInvoiceForBuyer(ctx, buyerID, invoiceID)
	if err != nil {
		return err
	}
	secret, _, err := s.publisherSecretKey(ctx, inv.PublisherID)
	if err != nil {
		return err
	}
	pi, err := stripeClient.GetPaymentIntent(secret, piID)
	if err != nil {
		return err
	}
	if pi.Status != "succeeded" {
		return httpx.BusinessRule("payment not completed")
	}
	if pi.Metadata["purpose"] != "invoice_payment" {
		return httpx.Validation("invalid payment purpose")
	}
	if pi.Metadata["invoice_public_id"] != inv.PublicID {
		return httpx.Validation("payment does not match invoice")
	}
	chargeID := ""
	if pi.LatestCharge != nil {
		chargeID = pi.LatestCharge.ID
	}
	return s.ConfirmInvoicePayment(ctx, inv.PublicID, float64(pi.Amount)/100.0, pi.ID, chargeID)
}

// ConfirmInvoicePayment credits balance after Stripe payment (idempotent).
func (s *Service) ConfirmInvoicePayment(ctx context.Context, invoicePublicID string, amount float64, piID, chargeID string) error {
	var exists bool
	if err := s.pool.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM invoices WHERE stripe_payment_intent_id = $1 AND status = 'paid')`,
		piID).Scan(&exists); err != nil {
		return err
	}
	if exists {
		return nil
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	note := "stripe"
	if err := s.settleInvoiceTx(ctx, tx, invoicePublicID, amount, "stripe", &note, nil, &piID, &chargeID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// MarkInvoicePaid records manual payment by the publisher.
func (s *Service) MarkInvoicePaid(ctx context.Context, publisherID, invoiceID, adminID int64, paymentMethod, note string) (*Invoice, error) {
	paymentMethod = strings.TrimSpace(paymentMethod)
	note = strings.TrimSpace(note)
	if !manualPaymentMethods[paymentMethod] {
		return nil, httpx.Validation("invalid payment method")
	}
	if paymentMethod == "other" && note == "" {
		return nil, httpx.Validation("payment note is required for other")
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	var publicID string
	var amount float64
	var status string
	err = tx.QueryRow(ctx,
		`SELECT public_id::text, amount::float8, status::text
		 FROM invoices WHERE id = $1 AND publisher_id = $2 FOR UPDATE`,
		invoiceID, publisherID).Scan(&publicID, &amount, &status)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, httpx.NotFound("invoice not found")
	}
	if err != nil {
		return nil, err
	}
	if status != "open" {
		return nil, httpx.BusinessRule("invoice is not open")
	}

	var notePtr *string
	if note != "" {
		notePtr = &note
	}
	if err := s.settleInvoiceTx(ctx, tx, publicID, amount, paymentMethod, notePtr, &adminID, nil, nil); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return s.getInvoiceForPublisher(ctx, publisherID, invoiceID)
}

// VoidInvoice cancels an open invoice.
func (s *Service) VoidInvoice(ctx context.Context, publisherID, invoiceID int64) (*Invoice, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	var status string
	err = tx.QueryRow(ctx,
		`SELECT status::text FROM invoices WHERE id = $1 AND publisher_id = $2 FOR UPDATE`,
		invoiceID, publisherID).Scan(&status)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, httpx.NotFound("invoice not found")
	}
	if err != nil {
		return nil, err
	}
	if status != "open" {
		return nil, httpx.BusinessRule("invoice is not open")
	}
	if _, err := tx.Exec(ctx, `UPDATE invoices SET status = 'void' WHERE id = $1`, invoiceID); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return s.getInvoiceForPublisher(ctx, publisherID, invoiceID)
}

func (s *Service) settleInvoiceTx(
	ctx context.Context, tx pgx.Tx,
	invoicePublicID string, amount float64,
	paymentMethod string, paymentNote *string,
	paidBy *int64, stripePI, stripeCharge *string,
) error {
	var invID, buyerID int64
	var invAmount float64
	var status, desc string
	err := tx.QueryRow(ctx,
		`SELECT id, buyer_id, amount::float8, status::text, description
		 FROM invoices WHERE public_id = $1 FOR UPDATE`,
		invoicePublicID).Scan(&invID, &buyerID, &invAmount, &status, &desc)
	if errors.Is(err, pgx.ErrNoRows) {
		return httpx.NotFound("invoice not found")
	}
	if err != nil {
		return err
	}
	if status != "open" {
		return nil
	}
	if amount > 0 && invAmount != amount {
		return httpx.BusinessRule("payment amount does not match invoice")
	}
	creditAmount := invAmount

	if err := EnsureBalance(ctx, tx, buyerID); err != nil {
		return err
	}
	var balance float64
	if err := tx.QueryRow(ctx,
		`SELECT balance::float8 FROM buyer_balances WHERE buyer_id = $1 FOR UPDATE`,
		buyerID).Scan(&balance); err != nil {
		return err
	}
	newBal := balance + creditAmount
	if _, err := tx.Exec(ctx,
		`UPDATE buyer_balances SET balance = $2 WHERE buyer_id = $1`, buyerID, newBal); err != nil {
		return err
	}

	txnDesc := fmt.Sprintf("prepay invoice — %s", desc)
	var txnID int64
	if stripePI != nil && *stripePI != "" {
		err = tx.QueryRow(ctx,
			`INSERT INTO transactions
			   (buyer_id, type, amount, balance_after, description, stripe_payment_intent_id, stripe_charge_id)
			 VALUES ($1, 'manual_invoice', $2, $3, $4, $5, $6)
			 RETURNING id`,
			buyerID, creditAmount, newBal, txnDesc, *stripePI, stripeCharge).Scan(&txnID)
	} else {
		err = tx.QueryRow(ctx,
			`INSERT INTO transactions(buyer_id, type, amount, balance_after, description)
			 VALUES ($1, 'manual_invoice', $2, $3, $4) RETURNING id`,
			buyerID, creditAmount, newBal, txnDesc).Scan(&txnID)
	}
	if err != nil {
		return err
	}

	_, err = tx.Exec(ctx,
		`UPDATE invoices SET
		   status = 'paid',
		   payment_method = $2::invoice_payment_method,
		   payment_note = $3,
		   paid_at = now(),
		   paid_by = $4,
		   stripe_payment_intent_id = COALESCE($5, stripe_payment_intent_id),
		   credit_txn_id = $6
		 WHERE id = $1`,
		invID, paymentMethod, paymentNote, paidBy, stripePI, txnID)
	if err != nil {
		return err
	}
	return nil
}

// CreatePrepayInvoice is the publisher-facing entry for ad-hoc prepay requests.
func (s *Service) CreatePrepayInvoice(ctx context.Context, publisherID, buyerID int64, amount float64, desc string) (*Invoice, error) {
	return s.CreateInvoice(ctx, publisherID, buyerID, amount, desc, InvoiceKindPrepayRequest)
}

// CreateStartingBalanceInvoice opens the initial prepay invoice for a new direct buyer.
func (s *Service) CreateStartingBalanceInvoice(ctx context.Context, publisherID, buyerID int64, amount float64) (*Invoice, error) {
	return s.CreateInvoice(ctx, publisherID, buyerID, amount, "Starting balance", InvoiceKindStartingBalance)
}
