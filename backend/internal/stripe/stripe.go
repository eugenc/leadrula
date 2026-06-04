// Package stripe wraps the Stripe SDK for Connect onboarding and PaymentIntents.
package stripe

import (
	"github.com/stripe/stripe-go/v78"
	"github.com/stripe/stripe-go/v78/account"
	"github.com/stripe/stripe-go/v78/accountlink"
	"github.com/stripe/stripe-go/v78/customer"
	"github.com/stripe/stripe-go/v78/paymentintent"
	"github.com/stripe/stripe-go/v78/paymentmethod"
	"github.com/stripe/stripe-go/v78/setupintent"
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
	secretKey   string
	platformFee float64
	appBaseURL  string
}

// New initialises the Stripe client and sets the global API key.
func New(secretKey, appBaseURL string, platformFee float64) *Client {
	stripe.Key = secretKey
	return &Client{secretKey: secretKey, platformFee: platformFee, appBaseURL: appBaseURL}
}

// Enabled reports whether Stripe API calls can be made.
func (c *Client) Enabled() bool {
	return c != nil && c.secretKey != ""
}

// CreateConnectAccount creates a new Express account for a publisher.
func (c *Client) CreateConnectAccount(email string) (string, error) {
	params := &stripe.AccountParams{
		Type:  stripe.String(string(stripe.AccountTypeExpress)),
		Email: stripe.String(email),
		Capabilities: &stripe.AccountCapabilitiesParams{
			Transfers: &stripe.AccountCapabilitiesTransfersParams{
				Requested: stripe.Bool(true),
			},
		},
	}
	acct, err := account.New(params)
	if err != nil {
		return "", err
	}
	return acct.ID, nil
}

// CreateOnboardingLink generates a Stripe-hosted onboarding URL.
func (c *Client) CreateOnboardingLink(stripeAccountID, returnURL, refreshURL string) (string, error) {
	params := &stripe.AccountLinkParams{
		Account:    stripe.String(stripeAccountID),
		RefreshURL: stripe.String(refreshURL),
		ReturnURL:  stripe.String(returnURL),
		Type:       stripe.String("account_onboarding"),
	}
	link, err := accountlink.New(params)
	if err != nil {
		return "", err
	}
	return link.URL, nil
}

// GetAccountStatus returns active, restricted, or pending.
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

// EnsureCustomer creates a Stripe customer and returns their customer ID.
func (c *Client) EnsureCustomer(email, name string) (string, error) {
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
func (c *Client) CreateSetupIntent(stripeCustomerID string) (string, error) {
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
func (c *Client) ListPaymentMethods(stripeCustomerID string) ([]PaymentMethod, error) {
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
func (c *Client) DetachPaymentMethod(pmID string) error {
	_, err := paymentmethod.Detach(pmID, nil)
	return err
}

// CreateTopupIntent creates a PaymentIntent for balance top-up.
func (c *Client) CreateTopupIntent(amountCents int64, stripeCustomerID, buyerAccountPublicID string) (string, string, error) {
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
