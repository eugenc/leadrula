package billing

import (
	"context"
	"errors"

	stripeClient "github.com/echayko/leadrula/backend/internal/stripe"
	"github.com/echayko/leadrula/backend/pkg/httpx"
	"github.com/jackc/pgx/v5"
)

func (s *Service) requireStripe() error {
	if s.stripe == nil || !s.stripe.Enabled() {
		return httpx.ServiceUnavailable("stripe is not configured")
	}
	return nil
}

// ConnectStripe creates or re-links a Stripe Express account for a publisher.
func (s *Service) ConnectStripe(ctx context.Context, accountID int64, returnBaseURL string) (string, error) {
	if err := s.requireStripe(); err != nil {
		return "", err
	}
	var stripeAccountID *string
	var email string
	if err := s.pool.QueryRow(ctx,
		`SELECT a.stripe_account_id, u.email
		 FROM accounts a
		 JOIN users u ON u.account_id = a.id AND u.role = 'admin'
		 WHERE a.id = $1 LIMIT 1`, accountID).Scan(&stripeAccountID, &email); err != nil {
		return "", err
	}

	returnURL := returnBaseURL + "/p/billing?stripe=complete"
	refreshURL := returnBaseURL + "/p/billing?stripe=refresh"

	if stripeAccountID != nil && *stripeAccountID != "" {
		return s.stripe.CreateOnboardingLink(*stripeAccountID, returnURL, refreshURL)
	}

	acctID, err := s.stripe.CreateConnectAccount(email)
	if err != nil {
		return "", err
	}
	if _, err := s.pool.Exec(ctx,
		`UPDATE accounts SET stripe_account_id = $1, stripe_account_status = 'pending' WHERE id = $2`,
		acctID, accountID); err != nil {
		return "", err
	}
	return s.stripe.CreateOnboardingLink(acctID, returnURL, refreshURL)
}

// RefreshStripeStatus polls Stripe and persists Connect account status.
func (s *Service) RefreshStripeStatus(ctx context.Context, accountID int64) (string, error) {
	if err := s.requireStripe(); err != nil {
		return "", err
	}
	var stripeAccountID *string
	if err := s.pool.QueryRow(ctx,
		`SELECT stripe_account_id FROM accounts WHERE id = $1`, accountID).Scan(&stripeAccountID); err != nil {
		return "", err
	}
	if stripeAccountID == nil || *stripeAccountID == "" {
		return "none", nil
	}
	status, err := s.stripe.GetAccountStatus(*stripeAccountID)
	if err != nil {
		return "", err
	}
	if _, err := s.pool.Exec(ctx,
		`UPDATE accounts SET stripe_account_status = $1 WHERE id = $2`, status, accountID); err != nil {
		return "", err
	}
	return status, nil
}

func (s *Service) ensureStripeCustomer(ctx context.Context, accountID int64) (string, error) {
	if err := s.requireStripe(); err != nil {
		return "", err
	}
	var stripeCustomerID *string
	var email, name string
	if err := s.pool.QueryRow(ctx,
		`SELECT a.stripe_customer_id, u.email, a.name
		 FROM accounts a
		 JOIN users u ON u.account_id = a.id AND u.role = 'admin'
		 WHERE a.id = $1 LIMIT 1`,
		accountID).Scan(&stripeCustomerID, &email, &name); err != nil {
		return "", err
	}
	if stripeCustomerID != nil && *stripeCustomerID != "" {
		return *stripeCustomerID, nil
	}
	custID, err := s.stripe.EnsureCustomer(email, name)
	if err != nil {
		return "", err
	}
	if _, err := s.pool.Exec(ctx,
		`UPDATE accounts SET stripe_customer_id = $1 WHERE id = $2`, custID, accountID); err != nil {
		return "", err
	}
	return custID, nil
}

// CreateSetupIntent returns a client_secret for saving a payment method.
func (s *Service) CreateSetupIntent(ctx context.Context, accountID int64) (string, error) {
	custID, err := s.ensureStripeCustomer(ctx, accountID)
	if err != nil {
		return "", err
	}
	return s.stripe.CreateSetupIntent(custID)
}

// ListPaymentMethods returns saved cards for the buyer account.
func (s *Service) ListPaymentMethods(ctx context.Context, accountID int64) ([]stripeClient.PaymentMethod, error) {
	custID, err := s.ensureStripeCustomer(ctx, accountID)
	if err != nil {
		return nil, err
	}
	methods, err := s.stripe.ListPaymentMethods(custID)
	if err != nil {
		return nil, err
	}
	if methods == nil {
		return []stripeClient.PaymentMethod{}, nil
	}
	return methods, nil
}

// DetachPaymentMethod removes a saved card.
func (s *Service) DetachPaymentMethod(ctx context.Context, accountID int64, pmID string) error {
	if err := s.requireStripe(); err != nil {
		return err
	}
	custID, err := s.ensureStripeCustomer(ctx, accountID)
	if err != nil {
		return err
	}
	methods, err := s.stripe.ListPaymentMethods(custID)
	if err != nil {
		return err
	}
	found := false
	for _, m := range methods {
		if m.ID == pmID {
			found = true
			break
		}
	}
	if !found {
		return httpx.NotFound("payment method not found")
	}
	return s.stripe.DetachPaymentMethod(pmID)
}

// CreateTopupIntent creates a Stripe PaymentIntent and returns the client_secret.
func (s *Service) CreateTopupIntent(ctx context.Context, accountID int64, amountCents int64) (string, error) {
	if amountCents < 500 {
		return "", httpx.Validation("minimum top-up is $5.00")
	}
	custID, err := s.ensureStripeCustomer(ctx, accountID)
	if err != nil {
		return "", err
	}
	var publicID string
	if err := s.pool.QueryRow(ctx,
		`SELECT public_id::text FROM accounts WHERE id = $1`, accountID).Scan(&publicID); err != nil {
		return "", err
	}
	_, clientSecret, err := s.stripe.CreateTopupIntent(amountCents, custID, publicID)
	if err != nil {
		return "", err
	}
	return clientSecret, nil
}

// ConfirmTopup credits the buyer balance after payment_intent.succeeded (idempotent).
func (s *Service) ConfirmTopup(ctx context.Context, buyerPublicID string, amount float64, piID, chargeID string) error {
	var exists bool
	if err := s.pool.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM transactions WHERE stripe_payment_intent_id = $1)`,
		piID).Scan(&exists); err != nil {
		return err
	}
	if exists {
		return nil
	}

	var buyerID int64
	if err := s.pool.QueryRow(ctx,
		`SELECT id FROM accounts WHERE public_id = $1`, buyerPublicID).Scan(&buyerID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return httpx.NotFound("buyer not found")
		}
		return err
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if err := EnsureBalance(ctx, tx, buyerID); err != nil {
		return err
	}
	var balance float64
	if err := tx.QueryRow(ctx,
		`SELECT balance::float8 FROM buyer_balances WHERE buyer_id = $1 FOR UPDATE`,
		buyerID).Scan(&balance); err != nil {
		return err
	}
	newBal := balance + amount
	if _, err := tx.Exec(ctx,
		`UPDATE buyer_balances SET balance = $2 WHERE buyer_id = $1`, buyerID, newBal); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx,
		`INSERT INTO transactions
		   (buyer_id, type, amount, balance_after, description,
		    stripe_payment_intent_id, stripe_charge_id)
		 VALUES ($1, 'topup', $2, $3, 'balance top-up via Stripe', $4, $5)`,
		buyerID, amount, newBal, piID, chargeID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
