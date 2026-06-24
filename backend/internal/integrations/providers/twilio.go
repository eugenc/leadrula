package providers

import (
	"context"
	"encoding/json"
	"fmt"
)

// TwilioProvider stores a publisher's Twilio account credentials so call sources
// can provision numbers, validate webhooks, and fetch recordings. It is not a
// lead-delivery provider.
type TwilioProvider struct{}

func (p *TwilioProvider) Slug() string { return "twilio" }

func (p *TwilioProvider) Deliver(ctx context.Context, credentials []byte, payload DeliveryPayload) (*DeliveryResult, error) {
	return nil, fmt.Errorf("twilio is not a delivery provider")
}

func (p *TwilioProvider) ValidateCredentials(ctx context.Context, credentials []byte, config map[string]any) error {
	var creds struct {
		AccountSID string `json:"account_sid"`
		AuthToken  string `json:"auth_token"`
	}
	if err := json.Unmarshal(credentials, &creds); err != nil {
		return fmt.Errorf("invalid twilio credentials")
	}
	if creds.AccountSID == "" || creds.AuthToken == "" {
		return fmt.Errorf("account_sid and auth_token are required")
	}
	return nil
}
