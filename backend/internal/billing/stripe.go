package billing

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/echayko/leadrula/backend/internal/accounts"
	stripeClient "github.com/echayko/leadrula/backend/internal/stripe"
	"github.com/echayko/leadrula/backend/pkg/httpx"
	"github.com/jackc/pgx/v5"
)

type StripeIntentResult struct {
	ClientSecret   string `json:"client_secret"`
	PublishableKey string `json:"publishable_key,omitempty"`
}

type BuyerStripeConfig struct {
	BuyerKind      string `json:"buyer_kind"`
	PublishableKey string `json:"publishable_key,omitempty"`
}

type PublisherKeysStatus struct {
	Status               string `json:"status"`
	PublishableKeyPrefix string `json:"publishable_key_prefix,omitempty"`
}

func (s *Service) requireStripe() error {
	if s.stripe == nil || !s.stripe.Enabled() {
		return httpx.ServiceUnavailable("stripe is not configured")
	}
	return nil
}

func (s *Service) requireEncKey() error {
	if len(s.encKey) != 32 {
		return httpx.ServiceUnavailable("encryption key not configured")
	}
	return nil
}

// PublisherConnectReady reports whether a publisher has an active Connect Standard account.
func (s *Service) PublisherConnectReady(ctx context.Context, publisherID int64) error {
	var status, connectType *string
	if err := s.pool.QueryRow(ctx,
		`SELECT stripe_account_status, stripe_connect_type
		 FROM accounts WHERE id = $1 AND type = 'publisher'`,
		publisherID).Scan(&status, &connectType); err != nil {
		return err
	}
	if status == nil || *status != "active" {
		return httpx.BusinessRule("connect stripe in billing for marketplace payouts")
	}
	if connectType == nil || *connectType != "standard" {
		return httpx.BusinessRule("connect stripe in billing for marketplace payouts")
	}
	return nil
}

// PublisherDirectStripeReady reports whether a publisher has verified API keys for direct buyers.
func (s *Service) PublisherDirectStripeReady(ctx context.Context, publisherID int64) error {
	var status string
	if err := s.pool.QueryRow(ctx,
		`SELECT stripe_keys_status FROM accounts WHERE id = $1 AND type = 'publisher'`,
		publisherID).Scan(&status); err != nil {
		return err
	}
	if status != "verified" {
		return httpx.BusinessRule("publisher has not finished stripe setup — pay manually or add stripe keys in billing")
	}
	return nil
}

// ConnectStripe returns a Connect Standard OAuth URL for a publisher.
func (s *Service) ConnectStripe(ctx context.Context, accountID int64, returnBaseURL string) (string, error) {
	if err := s.requireStripe(); err != nil {
		return "", err
	}
	if s.stripeOAuthRedirect == "" {
		return "", httpx.ServiceUnavailable("stripe oauth redirect not configured")
	}
	state, err := s.signOAuthState(accountID, returnBaseURL)
	if err != nil {
		return "", err
	}
	return s.stripe.ConnectOAuthURL(s.stripeOAuthRedirect, state)
}

func (s *Service) signOAuthState(accountID int64, returnBaseURL string) (string, error) {
	if len(s.encKey) != 32 {
		return "", httpx.ServiceUnavailable("encryption key not configured")
	}
	payload := fmt.Sprintf("%d|%s|%d", accountID, returnBaseURL, time.Now().Unix())
	mac := hmac.New(sha256.New, s.encKey)
	mac.Write([]byte(payload))
	sig := hex.EncodeToString(mac.Sum(nil))
	raw := payload + "|" + sig
	return base64.RawURLEncoding.EncodeToString([]byte(raw)), nil
}

func (s *Service) parseOAuthState(state string) (accountID int64, returnBaseURL string, err error) {
	if len(s.encKey) != 32 {
		return 0, "", httpx.ServiceUnavailable("encryption key not configured")
	}
	decoded, err := base64.RawURLEncoding.DecodeString(state)
	if err != nil {
		return 0, "", httpx.Validation("invalid oauth state")
	}
	parts := strings.Split(string(decoded), "|")
	if len(parts) != 4 {
		return 0, "", httpx.Validation("invalid oauth state")
	}
	accountID, err = strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return 0, "", httpx.Validation("invalid oauth state")
	}
	returnBaseURL = parts[1]
	ts, err := strconv.ParseInt(parts[2], 10, 64)
	if err != nil {
		return 0, "", httpx.Validation("invalid oauth state")
	}
	if time.Now().Unix()-ts > 3600 {
		return 0, "", httpx.Validation("oauth state expired")
	}
	payload := fmt.Sprintf("%s|%s|%d", parts[0], parts[1], ts)
	mac := hmac.New(sha256.New, s.encKey)
	mac.Write([]byte(payload))
	expected := hex.EncodeToString(mac.Sum(nil))
	if !hmac.Equal([]byte(expected), []byte(parts[3])) {
		return 0, "", httpx.Validation("invalid oauth state")
	}
	return accountID, returnBaseURL, nil
}

// CompleteStripeOAuth exchanges an OAuth code and stores the connected account.
func (s *Service) CompleteStripeOAuth(ctx context.Context, code, state string) (string, error) {
	if err := s.requireStripe(); err != nil {
		return "", err
	}
	accountID, returnBaseURL, err := s.parseOAuthState(state)
	if err != nil {
		return "", err
	}
	acctID, err := s.stripe.ExchangeOAuthCode(code)
	if err != nil {
		return "", err
	}
	status, err := s.stripe.GetAccountStatus(acctID)
	if err != nil {
		return "", err
	}
	if _, err := s.pool.Exec(ctx,
		`UPDATE accounts SET stripe_account_id = $1, stripe_connect_type = 'standard', stripe_account_status = $2
		 WHERE id = $3 AND type = 'publisher'`,
		acctID, status, accountID); err != nil {
		return "", err
	}
	return returnBaseURL, nil
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

// SavePublisherStripeKeys validates and stores encrypted publisher API keys.
func (s *Service) SavePublisherStripeKeys(ctx context.Context, publisherID int64, secretKey, publishableKey string) error {
	if err := s.requireEncKey(); err != nil {
		return err
	}
	secretKey = strings.TrimSpace(secretKey)
	publishableKey = strings.TrimSpace(publishableKey)
	if secretKey == "" || publishableKey == "" {
		return httpx.Validation("secret and publishable keys are required")
	}
	if !strings.HasPrefix(secretKey, "sk_") {
		return httpx.Validation("invalid secret key format")
	}
	if !strings.HasPrefix(publishableKey, "pk_") {
		return httpx.Validation("invalid publishable key format")
	}
	if !keysMatchMode(secretKey, publishableKey) {
		return httpx.Validation("secret and publishable keys must both be test or live mode")
	}
	if err := stripeClient.ValidateSecretKey(secretKey); err != nil {
		return httpx.Validation("stripe rejected the secret key")
	}
	enc, err := encryptKey(s.encKey, []byte(secretKey))
	if err != nil {
		return err
	}
	_, err = s.pool.Exec(ctx,
		`UPDATE accounts SET stripe_secret_key_encrypted = $2, stripe_publishable_key = $3, stripe_keys_status = 'verified'
		 WHERE id = $1 AND type = 'publisher'`,
		publisherID, enc, publishableKey)
	return err
}

// PublisherStripeKeysStatus returns key setup status without exposing secrets.
func (s *Service) PublisherStripeKeysStatus(ctx context.Context, publisherID int64) (*PublisherKeysStatus, error) {
	var status string
	var pk *string
	if err := s.pool.QueryRow(ctx,
		`SELECT stripe_keys_status, stripe_publishable_key FROM accounts WHERE id = $1`,
		publisherID).Scan(&status, &pk); err != nil {
		return nil, err
	}
	out := &PublisherKeysStatus{Status: status}
	if pk != nil && *pk != "" {
		out.PublishableKeyPrefix = publishablePrefix(*pk)
	}
	return out, nil
}

func (s *Service) publisherSecretKey(ctx context.Context, publisherID int64) (string, string, error) {
	var enc []byte
	var pk *string
	var status string
	if err := s.pool.QueryRow(ctx,
		`SELECT stripe_secret_key_encrypted, stripe_publishable_key, stripe_keys_status
		 FROM accounts WHERE id = $1 AND type = 'publisher'`,
		publisherID).Scan(&enc, &pk, &status); err != nil {
		return "", "", err
	}
	if status != "verified" || len(enc) == 0 {
		return "", "", httpx.BusinessRule("publisher stripe keys are not configured")
	}
	if len(s.encKey) != 32 {
		return "", "", httpx.ServiceUnavailable("encryption key not configured")
	}
	secret, err := decryptKey(s.encKey, enc)
	if err != nil {
		return "", "", err
	}
	publishable := ""
	if pk != nil {
		publishable = *pk
	}
	return string(secret), publishable, nil
}

func (s *Service) buyerKind(ctx context.Context, buyerID int64) (string, error) {
	var kind string
	if err := s.pool.QueryRow(ctx,
		`SELECT buyer_kind::text FROM accounts WHERE id = $1 AND type = 'buyer'`, buyerID).Scan(&kind); err != nil {
		return "", err
	}
	return kind, nil
}

func (s *Service) directBuyerPublisherID(ctx context.Context, buyerID int64) (int64, error) {
	var publisherID int64
	err := s.pool.QueryRow(ctx,
		`SELECT pt.publisher_id FROM partnerships pt
		 JOIN accounts b ON b.id = pt.buyer_id
		 WHERE pt.buyer_id = $1 AND pt.status = 'active' AND b.buyer_kind = 'direct'
		 LIMIT 1`, buyerID).Scan(&publisherID)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, httpx.BusinessRule("no active direct publisher partnership")
	}
	return publisherID, err
}

// BuyerStripeConfig returns billing stripe context for the buyer UI.
func (s *Service) BuyerStripeConfig(ctx context.Context, buyerID int64) (*BuyerStripeConfig, error) {
	kind, err := s.buyerKind(ctx, buyerID)
	if err != nil {
		return nil, err
	}
	cfg := &BuyerStripeConfig{BuyerKind: kind}
	if kind != accounts.BuyerKindDirect {
		return cfg, nil
	}
	pubID, err := s.directBuyerPublisherID(ctx, buyerID)
	if err != nil {
		return cfg, nil
	}
	_, pk, err := s.publisherSecretKey(ctx, pubID)
	if err != nil {
		return cfg, nil
	}
	cfg.PublishableKey = pk
	return cfg, nil
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

func (s *Service) ensurePublisherStripeCustomer(ctx context.Context, buyerID, publisherID int64, secretKey string) (string, error) {
	var custID string
	err := s.pool.QueryRow(ctx,
		`SELECT stripe_customer_id FROM buyer_publisher_stripe WHERE buyer_id = $1 AND publisher_id = $2`,
		buyerID, publisherID).Scan(&custID)
	if err == nil && custID != "" {
		return custID, nil
	}
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return "", err
	}
	var email, name string
	if err := s.pool.QueryRow(ctx,
		`SELECT u.email, a.name
		 FROM accounts a
		 JOIN users u ON u.account_id = a.id AND u.role = 'admin'
		 WHERE a.id = $1 LIMIT 1`,
		buyerID).Scan(&email, &name); err != nil {
		return "", err
	}
	newID, err := stripeClient.EnsureCustomer(secretKey, email, name)
	if err != nil {
		return "", err
	}
	_, err = s.pool.Exec(ctx,
		`INSERT INTO buyer_publisher_stripe(buyer_id, publisher_id, stripe_customer_id)
		 VALUES ($1, $2, $3)
		 ON CONFLICT (buyer_id, publisher_id) DO UPDATE SET stripe_customer_id = EXCLUDED.stripe_customer_id`,
		buyerID, publisherID, newID)
	if err != nil {
		return "", err
	}
	return newID, nil
}

// CreateSetupIntent returns a client_secret for saving a payment method.
func (s *Service) CreateSetupIntent(ctx context.Context, accountID int64) (*StripeIntentResult, error) {
	kind, err := s.buyerKind(ctx, accountID)
	if err != nil {
		return nil, err
	}
	if kind == accounts.BuyerKindDirect {
		pubID, err := s.directBuyerPublisherID(ctx, accountID)
		if err != nil {
			return nil, err
		}
		secret, pk, err := s.publisherSecretKey(ctx, pubID)
		if err != nil {
			return nil, err
		}
		custID, err := s.ensurePublisherStripeCustomer(ctx, accountID, pubID, secret)
		if err != nil {
			return nil, err
		}
		cs, err := stripeClient.CreateSetupIntent(secret, custID)
		if err != nil {
			return nil, err
		}
		return &StripeIntentResult{ClientSecret: cs, PublishableKey: pk}, nil
	}
	if err := s.requireStripe(); err != nil {
		return nil, err
	}
	custID, err := s.ensureStripeCustomer(ctx, accountID)
	if err != nil {
		return nil, err
	}
	cs, err := s.stripe.CreateSetupIntent(custID)
	if err != nil {
		return nil, err
	}
	return &StripeIntentResult{ClientSecret: cs}, nil
}

// ListPaymentMethods returns saved cards for the buyer account.
func (s *Service) ListPaymentMethods(ctx context.Context, accountID int64) ([]stripeClient.PaymentMethod, error) {
	kind, err := s.buyerKind(ctx, accountID)
	if err != nil {
		return nil, err
	}
	if kind == accounts.BuyerKindDirect {
		pubID, err := s.directBuyerPublisherID(ctx, accountID)
		if err != nil {
			return []stripeClient.PaymentMethod{}, nil
		}
		secret, _, err := s.publisherSecretKey(ctx, pubID)
		if err != nil {
			return []stripeClient.PaymentMethod{}, nil
		}
		var custID string
		err = s.pool.QueryRow(ctx,
			`SELECT stripe_customer_id FROM buyer_publisher_stripe WHERE buyer_id = $1 AND publisher_id = $2`,
			accountID, pubID).Scan(&custID)
		if errors.Is(err, pgx.ErrNoRows) || custID == "" {
			return []stripeClient.PaymentMethod{}, nil
		}
		if err != nil {
			return nil, err
		}
		methods, err := stripeClient.ListPaymentMethods(secret, custID)
		if err != nil {
			return nil, err
		}
		if methods == nil {
			return []stripeClient.PaymentMethod{}, nil
		}
		return methods, nil
	}
	if err := s.requireStripe(); err != nil {
		return nil, err
	}
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
	kind, err := s.buyerKind(ctx, accountID)
	if err != nil {
		return err
	}
	if kind == accounts.BuyerKindDirect {
		pubID, err := s.directBuyerPublisherID(ctx, accountID)
		if err != nil {
			return err
		}
		secret, _, err := s.publisherSecretKey(ctx, pubID)
		if err != nil {
			return err
		}
		custID, err := s.ensurePublisherStripeCustomer(ctx, accountID, pubID, secret)
		if err != nil {
			return err
		}
		methods, err := stripeClient.ListPaymentMethods(secret, custID)
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
		return stripeClient.DetachPaymentMethod(secret, pmID)
	}
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
func (s *Service) CreateTopupIntent(ctx context.Context, accountID int64, amountCents int64) (*StripeIntentResult, error) {
	if amountCents < 500 {
		return nil, httpx.Validation("minimum top-up is $5.00")
	}
	kind, err := s.buyerKind(ctx, accountID)
	if err != nil {
		return nil, err
	}
	var buyerPublicID string
	if err := s.pool.QueryRow(ctx,
		`SELECT public_id::text FROM accounts WHERE id = $1`, accountID).Scan(&buyerPublicID); err != nil {
		return nil, err
	}

	if kind == accounts.BuyerKindDirect {
		pubID, err := s.directBuyerPublisherID(ctx, accountID)
		if err != nil {
			return nil, err
		}
		secret, pk, err := s.publisherSecretKey(ctx, pubID)
		if err != nil {
			return nil, err
		}
		custID, err := s.ensurePublisherStripeCustomer(ctx, accountID, pubID, secret)
		if err != nil {
			return nil, err
		}
		_, cs, err := stripeClient.CreateTopupIntent(secret, amountCents, custID, buyerPublicID)
		if err != nil {
			return nil, err
		}
		return &StripeIntentResult{ClientSecret: cs, PublishableKey: pk}, nil
	}

	if err := s.requireStripe(); err != nil {
		return nil, err
	}
	custID, err := s.ensureStripeCustomer(ctx, accountID)
	if err != nil {
		return nil, err
	}
	_, cs, err := s.stripe.CreateTopupIntent(amountCents, custID, buyerPublicID)
	if err != nil {
		return nil, err
	}
	return &StripeIntentResult{ClientSecret: cs}, nil
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

// ConfirmDirectTopup verifies a publisher-account topup PI and credits balance.
func (s *Service) ConfirmDirectTopup(ctx context.Context, buyerID int64, piID string) error {
	pubID, err := s.directBuyerPublisherID(ctx, buyerID)
	if err != nil {
		return err
	}
	secret, _, err := s.publisherSecretKey(ctx, pubID)
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
	if pi.Metadata["purpose"] != "balance_topup" {
		return httpx.Validation("invalid payment purpose")
	}
	chargeID := ""
	if pi.LatestCharge != nil {
		chargeID = pi.LatestCharge.ID
	}
	return s.ConfirmTopup(ctx, pi.Metadata["buyer_public_id"], float64(pi.Amount)/100.0, pi.ID, chargeID)
}
