package routing

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/echayko/leadrula/backend/internal/integrations/providers"
	"github.com/echayko/leadrula/backend/pkg/httpx"
	"github.com/jackc/pgx/v5"
)

// CallWebhookCreds decrypts integration credentials for call source webhook sync.
type CallWebhookCreds interface {
	DecryptedCredentials(ctx context.Context, accountID, connectionID int64) (map[string]string, error)
}

// SetCallWebhooks wires Twilio voice webhook sync for call-type sources.
func (s *Service) SetCallWebhooks(creds CallWebhookCreds, webhookBaseURL string) {
	s.callWebhookCreds = creds
	s.webhookBaseURL = strings.TrimRight(strings.TrimSpace(webhookBaseURL), "/")
}

func (s *Service) GetSource(ctx context.Context, publisherID, id int64) (*Source, error) {
	src, err := scanSource(s.pool.QueryRow(ctx,
		`SELECT `+sourceCols+` FROM routing_sources WHERE id=$1 AND publisher_id=$2`,
		id, publisherID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, httpx.NotFound("source not found")
		}
		return nil, err
	}
	return src, nil
}

func (s *Service) validateCallSourceParams(ctx context.Context, publisherID int64, call *CallSourceParams) error {
	if call.IntegrationConnectionID == nil || *call.IntegrationConnectionID == 0 {
		return httpx.Validation("integration_connection_id is required for call sources")
	}
	if call.TwilioSID == nil || strings.TrimSpace(*call.TwilioSID) == "" {
		return httpx.Validation("twilio_sid is required for call sources")
	}
	if call.TrackingNumber == nil || strings.TrimSpace(*call.TrackingNumber) == "" {
		return httpx.Validation("tracking_number is required for call sources")
	}
	return s.ensureTwilioConnection(ctx, publisherID, *call.IntegrationConnectionID)
}

func (s *Service) ensureTwilioConnection(ctx context.Context, publisherID, connectionID int64) error {
	var slug string
	err := s.pool.QueryRow(ctx,
		`SELECT p.slug
		 FROM integration_connections c
		 JOIN integration_providers p ON p.id = c.provider_id
		 WHERE c.id = $1 AND c.account_id = $2`,
		connectionID, publisherID).Scan(&slug)
	if errors.Is(err, pgx.ErrNoRows) {
		return httpx.NotFound("twilio connection not found")
	}
	if err != nil {
		return err
	}
	if slug != "twilio" {
		return httpx.Validation("integration connection must be a twilio connection")
	}
	return nil
}

func (s *Service) twilioVoiceURL() string {
	return s.webhookBaseURL + "/webhooks/twilio/voice"
}

func (s *Service) syncCreateCallWebhooks(ctx context.Context, publisherID int64, call *CallSourceParams) error {
	if s.callWebhookCreds == nil || s.webhookBaseURL == "" {
		return httpx.Validation("call webhook sync is not configured")
	}
	if err := s.validateCallSourceParams(ctx, publisherID, call); err != nil {
		return err
	}
	creds, err := s.callWebhookCreds.DecryptedCredentials(ctx, publisherID, *call.IntegrationConnectionID)
	if err != nil {
		return fmt.Errorf("twilio credentials: %w", err)
	}
	accountSID := creds["account_sid"]
	authToken := creds["auth_token"]
	if accountSID == "" || authToken == "" {
		return httpx.Validation("twilio connection is missing account_sid or auth_token")
	}
	if err := providers.SetVoiceWebhook(ctx, accountSID, authToken, strings.TrimSpace(*call.TwilioSID), s.twilioVoiceURL()); err != nil {
		return httpx.Validation("failed to configure twilio voice webhook: " + err.Error())
	}
	return nil
}

func (s *Service) syncUpdateCallWebhooks(ctx context.Context, publisherID int64, old *Source, call *CallSourceParams) error {
	if old.Type != "call" {
		return nil
	}
	if s.callWebhookCreds == nil || s.webhookBaseURL == "" {
		return httpx.Validation("call webhook sync is not configured")
	}

	newConnID := old.IntegrationConnectionID
	if call.IntegrationConnectionID != nil {
		newConnID = call.IntegrationConnectionID
	}
	newTracking := old.TrackingNumber
	if call.TrackingNumber != nil {
		trimmed := strings.TrimSpace(*call.TrackingNumber)
		newTracking = &trimmed
	}
	newTwilioSID := old.TwilioSID
	if call.TwilioSID != nil {
		trimmed := strings.TrimSpace(*call.TwilioSID)
		newTwilioSID = &trimmed
	}

	merged := &CallSourceParams{
		IntegrationConnectionID: newConnID,
		TrackingNumber:          newTracking,
		TwilioSID:               newTwilioSID,
	}
	if err := s.validateCallSourceParams(ctx, publisherID, merged); err != nil {
		return err
	}

	oldConnID := int64(0)
	if old.IntegrationConnectionID != nil {
		oldConnID = *old.IntegrationConnectionID
	}
	newConnIDVal := int64(0)
	if newConnID != nil {
		newConnIDVal = *newConnID
	}
	oldSID := ""
	if old.TwilioSID != nil {
		oldSID = strings.TrimSpace(*old.TwilioSID)
	}
	newSID := ""
	if newTwilioSID != nil {
		newSID = strings.TrimSpace(*newTwilioSID)
	}

	if oldSID != "" && oldConnID != 0 && (oldSID != newSID || oldConnID != newConnIDVal) {
		if err := s.clearTwilioVoiceWebhook(ctx, publisherID, oldConnID, oldSID); err != nil {
			return httpx.Validation("failed to clear previous twilio voice webhook: " + err.Error())
		}
	}

	if newSID != "" && newConnIDVal != 0 {
		creds, err := s.callWebhookCreds.DecryptedCredentials(ctx, publisherID, newConnIDVal)
		if err != nil {
			return fmt.Errorf("twilio credentials: %w", err)
		}
		accountSID := creds["account_sid"]
		authToken := creds["auth_token"]
		if accountSID == "" || authToken == "" {
			return httpx.Validation("twilio connection is missing account_sid or auth_token")
		}
		if err := providers.SetVoiceWebhook(ctx, accountSID, authToken, newSID, s.twilioVoiceURL()); err != nil {
			return httpx.Validation("failed to configure twilio voice webhook: " + err.Error())
		}
	}
	return nil
}

func (s *Service) clearTwilioVoiceWebhook(ctx context.Context, publisherID, connectionID int64, phoneNumberSID string) error {
	creds, err := s.callWebhookCreds.DecryptedCredentials(ctx, publisherID, connectionID)
	if err != nil {
		return err
	}
	accountSID := creds["account_sid"]
	authToken := creds["auth_token"]
	if accountSID == "" || authToken == "" {
		return httpx.Validation("twilio connection is missing account_sid or auth_token")
	}
	return providers.ClearVoiceWebhook(ctx, accountSID, authToken, phoneNumberSID)
}
