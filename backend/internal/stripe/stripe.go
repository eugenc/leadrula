// Package stripe wraps the Stripe SDK for Connect onboarding and PaymentIntents.
package stripe

import (
	"fmt"
	"net/url"

	"github.com/stripe/stripe-go/v78"
	"github.com/stripe/stripe-go/v78/account"
	"github.com/stripe/stripe-go/v78/customer"
	"github.com/stripe/stripe-go/v78/oauth"
	"github.com/stripe/stripe-go/v78/paymentintent"
	"github.com/stripe/stripe-go/v78/paymentmethod"
	"github.com/stripe/stripe-go/v78/setupintent"
	"github.com/stripe/stripe-go/v78/transfer"
)

// PaymentMethod is a saved card on a Stripe customer.
type PaymentMethod struct {
	ID        string `json:"id"`
	Brand     string `json:"brand"`
	Last4     string `json:"last4"`
	ExpMonth  int64  `json:"exp_month"`
	ExpYear   int64  `json:"exp_year"`
	IsDefault bool   `json:"is_default"`
}

// Client holds Stripe config and exposes platform operations.
type Client struct {
	secretKey     string
	platformFee   float64
	connectClient string
}

// New initialises the Stripe client and sets the global API key.
func New(secretKey, connectClientID string, platformFee float64) *Client {
	stripe.Key = secretKey
	return &Client{secretKey: secretKey, platformFee: platformFee, connectClient: connectClientID}
}

// Enabled reports whether Stripe API calls can be made.
func (c *Client) Enabled() bool {
	return c != nil && c.secretKey != ""
}

// ConnectOAuthURL builds the Stripe Connect Standard OAuth authorize URL.
func (c *Client) ConnectOAuthURL(redirectURI, state string) (string, error) {
	if c.connectClient == "" {
		return "", fmt.Errorf("stripe connect client id not configured")
	}
	v := url.Values{
		"response_type": {"code"},
		"client_id":     {c.connectClient},
		"scope":         {"read_write"},
		"redirect_uri":  {redirectURI},
		"state":         {state},
	}
	return "https://connect.stripe.com/oauth/authorize?" + v.Encode(), nil
}

// ExchangeOAuthCode completes Connect Standard OAuth and returns the linked account ID.
func (c *Client) ExchangeOAuthCode(code string) (string, error) {
	params := &stripe.OAuthTokenParams{
		GrantType: stripe.String("authorization_code"),
		Code:      stripe.String(code),
	}
	resp, err := oauth.New(params)
	if err != nil {
		return "", err
	}
	return resp.StripeUserID, nil
}

// GetAccountStatus returns active, restricted, or pending for a connected account.
func (c *Client) GetAccountStatus(stripeAccountID string) (string, error) {
	acct, err := account.GetByID(stripeAccountID, nil)
	if err != nil {
		return "", err
	}
	if acct.DetailsSubmitted && acct.ChargesEnabled {
		return "active", nil
	}
	if acct.DetailsSubmitted {
		return "restricted", nil
	}
	return "pending", nil
}

// ValidateSecretKey checks that a secret key is usable.
func ValidateSecretKey(secretKey string) error {
	old := stripe.Key
	stripe.Key = secretKey
	defer func() { stripe.Key = old }()
	_, err := account.Get()
	return err
}

// EnsureCustomer creates a Stripe customer and returns their customer ID.
func EnsureCustomer(secretKey, email, name string) (string, error) {
	old := stripe.Key
	stripe.Key = secretKey
	defer func() { stripe.Key = old }()

	params := &stripe.CustomerParams{
		Email: stripe.String(email),
		Name:  stripe.String(name),
	}
	cust, err := customer.New(params)
	if err != nil {
		return "", err
	}
	return cust.ID, nil
}

// CreateSetupIntent returns a client_secret for saving a payment method.
func CreateSetupIntent(secretKey, stripeCustomerID string) (string, error) {
	old := stripe.Key
	stripe.Key = secretKey
	defer func() { stripe.Key = old }()

	params := &stripe.SetupIntentParams{
		Customer: stripe.String(stripeCustomerID),
		AutomaticPaymentMethods: &stripe.SetupIntentAutomaticPaymentMethodsParams{
			Enabled: stripe.Bool(true),
		},
	}
	si, err := setupintent.New(params)
	if err != nil {
		return "", err
	}
	return si.ClientSecret, nil
}

// ListPaymentMethods returns saved cards for a customer.
func ListPaymentMethods(secretKey, stripeCustomerID string) ([]PaymentMethod, error) {
	old := stripe.Key
	stripe.Key = secretKey
	defer func() { stripe.Key = old }()

	cust, err := customer.Get(stripeCustomerID, nil)
	if err != nil {
		return nil, err
	}
	var defaultID string
	if cust.InvoiceSettings != nil && cust.InvoiceSettings.DefaultPaymentMethod != nil {
		defaultID = cust.InvoiceSettings.DefaultPaymentMethod.ID
	}

	params := &stripe.PaymentMethodListParams{
		Customer: stripe.String(stripeCustomerID),
		Type:     stripe.String(string(stripe.PaymentMethodTypeCard)),
	}
	iter := paymentmethod.List(params)
	var out []PaymentMethod
	for iter.Next() {
		pm := iter.PaymentMethod()
		if pm.Card == nil {
			continue
		}
		out = append(out, PaymentMethod{
			ID:        pm.ID,
			Brand:     string(pm.Card.Brand),
			Last4:     pm.Card.Last4,
			ExpMonth:  pm.Card.ExpMonth,
			ExpYear:   pm.Card.ExpYear,
			IsDefault: pm.ID == defaultID,
		})
	}
	return out, iter.Err()
}

// DetachPaymentMethod removes a saved payment method from the customer.
func DetachPaymentMethod(secretKey, pmID string) error {
	old := stripe.Key
	stripe.Key = secretKey
	defer func() { stripe.Key = old }()
	_, err := paymentmethod.Detach(pmID, nil)
	return err
}

// CreateInvoicePaymentIntent creates a PaymentIntent for an invoice payment.
// When destinationAccountID is set (marketplace/platform path), funds transfer to Connect account.
func (c *Client) CreateInvoicePaymentIntent(amountCents int64, stripeCustomerID, invoicePublicID, buyerAccountPublicID string, destinationAccountID *string) (string, string, error) {
	params := &stripe.PaymentIntentParams{
		Amount:   stripe.Int64(amountCents),
		Currency: stripe.String("usd"),
		Customer: stripe.String(stripeCustomerID),
		AutomaticPaymentMethods: &stripe.PaymentIntentAutomaticPaymentMethodsParams{
			Enabled: stripe.Bool(true),
		},
		Metadata: map[string]string{
			"purpose":           "invoice_payment",
			"invoice_public_id": invoicePublicID,
			"buyer_public_id":   buyerAccountPublicID,
		},
	}
	if destinationAccountID != nil && *destinationAccountID != "" {
		params.TransferData = &stripe.PaymentIntentTransferDataParams{
			Destination: stripe.String(*destinationAccountID),
		}
		if c.platformFee > 0 {
			fee := int64(float64(amountCents) * c.platformFee)
			if fee > 0 {
				params.ApplicationFeeAmount = stripe.Int64(fee)
			}
		}
	}
	pi, err := paymentintent.New(params)
	if err != nil {
		return "", "", err
	}
	return pi.ID, pi.ClientSecret, nil
}

// CreatePublisherInvoicePaymentIntent creates a PI on a publisher's own Stripe account.
func CreatePublisherInvoicePaymentIntent(secretKey string, amountCents int64, stripeCustomerID, invoicePublicID, buyerAccountPublicID string) (string, string, error) {
	old := stripe.Key
	stripe.Key = secretKey
	defer func() { stripe.Key = old }()

	params := &stripe.PaymentIntentParams{
		Amount:   stripe.Int64(amountCents),
		Currency: stripe.String("usd"),
		Customer: stripe.String(stripeCustomerID),
		AutomaticPaymentMethods: &stripe.PaymentIntentAutomaticPaymentMethodsParams{
			Enabled: stripe.Bool(true),
		},
		Metadata: map[string]string{
			"purpose":           "invoice_payment",
			"invoice_public_id": invoicePublicID,
			"buyer_public_id":   buyerAccountPublicID,
		},
	}
	pi, err := paymentintent.New(params)
	if err != nil {
		return "", "", err
	}
	return pi.ID, pi.ClientSecret, nil
}

// CreateTopupIntent creates a PaymentIntent for balance top-up.
func CreateTopupIntent(secretKey string, amountCents int64, stripeCustomerID, buyerAccountPublicID string) (string, string, error) {
	old := stripe.Key
	stripe.Key = secretKey
	defer func() { stripe.Key = old }()

	params := &stripe.PaymentIntentParams{
		Amount:   stripe.Int64(amountCents),
		Currency: stripe.String("usd"),
		Customer: stripe.String(stripeCustomerID),
		AutomaticPaymentMethods: &stripe.PaymentIntentAutomaticPaymentMethodsParams{
			Enabled: stripe.Bool(true),
		},
		Metadata: map[string]string{
			"purpose":         "balance_topup",
			"buyer_public_id": buyerAccountPublicID,
		},
	}
	pi, err := paymentintent.New(params)
	if err != nil {
		return "", "", err
	}
	return pi.ID, pi.ClientSecret, nil
}

// GetPaymentIntent retrieves a PaymentIntent using the given secret key.
func GetPaymentIntent(secretKey, piID string) (*stripe.PaymentIntent, error) {
	old := stripe.Key
	stripe.Key = secretKey
	defer func() { stripe.Key = old }()
	return paymentintent.Get(piID, nil)
}

// Platform wrappers using the platform secret key.

func (c *Client) EnsureCustomer(email, name string) (string, error) {
	return EnsureCustomer(c.secretKey, email, name)
}

func (c *Client) CreateSetupIntent(stripeCustomerID string) (string, error) {
	return CreateSetupIntent(c.secretKey, stripeCustomerID)
}

func (c *Client) ListPaymentMethods(stripeCustomerID string) ([]PaymentMethod, error) {
	return ListPaymentMethods(c.secretKey, stripeCustomerID)
}

func (c *Client) DetachPaymentMethod(pmID string) error {
	return DetachPaymentMethod(c.secretKey, pmID)
}

func (c *Client) CreateTopupIntent(amountCents int64, stripeCustomerID, buyerAccountPublicID string) (string, string, error) {
	return CreateTopupIntent(c.secretKey, amountCents, stripeCustomerID, buyerAccountPublicID)
}

// NetTransferAmount returns the payout amount after deducting the platform fee.
func (c *Client) NetTransferAmount(amountCents int64) int64 {
	if amountCents <= 0 {
		return 0
	}
	if c.platformFee <= 0 {
		return amountCents
	}
	fee := int64(float64(amountCents) * c.platformFee)
	if fee >= amountCents {
		return 0
	}
	return amountCents - fee
}

// CreateTransfer moves funds from the platform balance to a connected account.
func (c *Client) CreateTransfer(amountCents int64, destinationAccountID, idempotencyKey string) (string, error) {
	if amountCents < 1 {
		return "", fmt.Errorf("transfer amount must be positive")
	}
	params := &stripe.TransferParams{
		Amount:      stripe.Int64(amountCents),
		Currency:    stripe.String("usd"),
		Destination: stripe.String(destinationAccountID),
	}
	params.SetIdempotencyKey(idempotencyKey)
	params.AddMetadata("purpose", "marketplace_payout_clear")
	tr, err := transfer.New(params)
	if err != nil {
		return "", err
	}
	return tr.ID, nil
}
