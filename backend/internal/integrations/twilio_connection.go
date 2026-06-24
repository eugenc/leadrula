package integrations

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"strings"

	"github.com/echayko/leadrula/backend/internal/integrations/providers"
	"github.com/echayko/leadrula/backend/pkg/httpx"
	"github.com/jackc/pgx/v5"
)

var e164Phone = regexp.MustCompile(`^\+[1-9]\d{1,14}$`)

func (s *Service) twilioCreds(ctx context.Context, accountID, connectionID int64) (accountSID, authToken string, err error) {
	conn, err := s.GetConnection(ctx, accountID, connectionID)
	if err != nil {
		return "", "", err
	}
	if conn.ProviderSlug != "twilio" {
		return "", "", httpx.Validation("not a twilio connection")
	}
	creds, err := s.DecryptedCredentials(ctx, accountID, connectionID)
	if err != nil {
		return "", "", err
	}
	accountSID = creds["account_sid"]
	authToken = creds["auth_token"]
	if accountSID == "" || authToken == "" {
		return "", "", httpx.Validation("twilio connection is missing account_sid or auth_token")
	}
	return accountSID, authToken, nil
}

type twilioNumberInUse struct {
	SourceName string
	IsActive   bool
}

func (s *Service) twilioNumbersInUse(ctx context.Context, publisherID, connectionID int64) (bySID map[string]twilioNumberInUse, byPhone map[string]twilioNumberInUse, err error) {
	rows, err := s.pool.Query(ctx,
		`SELECT name, twilio_sid, tracking_number, is_active
		 FROM routing_sources
		 WHERE publisher_id = $1 AND type = 'call' AND integration_connection_id = $2
		   AND (twilio_sid IS NOT NULL OR tracking_number IS NOT NULL)`,
		publisherID, connectionID)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()
	bySID = map[string]twilioNumberInUse{}
	byPhone = map[string]twilioNumberInUse{}
	for rows.Next() {
		var name string
		var sid, phone *string
		var active bool
		if err := rows.Scan(&name, &sid, &phone, &active); err != nil {
			return nil, nil, err
		}
		entry := twilioNumberInUse{SourceName: name, IsActive: active}
		if sid != nil && strings.TrimSpace(*sid) != "" {
			bySID[strings.TrimSpace(*sid)] = entry
		}
		if phone != nil && strings.TrimSpace(*phone) != "" {
			byPhone[strings.TrimSpace(*phone)] = entry
		}
	}
	return bySID, byPhone, rows.Err()
}

func classifyTwilioNumberType(e164 string) string {
	digits := strings.TrimPrefix(e164, "+1")
	if len(digits) >= 3 {
		prefix := digits[:3]
		switch prefix {
		case "800", "888", "877", "866", "855", "844", "833":
			return "tollfree"
		}
	}
	return "local"
}

// ListTwilioPhoneNumbers returns voice-capable numbers on a Twilio connection with in-use metadata.
func (s *Service) ListTwilioPhoneNumbers(ctx context.Context, accountID, connectionID int64) ([]providers.TwilioPhoneNumber, error) {
	accountSID, authToken, err := s.twilioCreds(ctx, accountID, connectionID)
	if err != nil {
		return nil, err
	}
	numbers, err := providers.ListVoiceNumbers(ctx, accountSID, authToken)
	if err != nil {
		return nil, fmt.Errorf("list twilio numbers: %w", err)
	}
	bySID, byPhone, err := s.twilioNumbersInUse(ctx, accountID, connectionID)
	if err != nil {
		return nil, err
	}
	out := make([]providers.TwilioPhoneNumber, 0, len(numbers))
	for _, n := range numbers {
		item := n
		item.NumberType = classifyTwilioNumberType(n.PhoneNumber)
		if use, ok := bySID[n.SID]; ok {
			name := use.SourceName
			item.InUseBySource = &name
			item.InUseActive = use.IsActive
		} else if use, ok := byPhone[n.PhoneNumber]; ok {
			name := use.SourceName
			item.InUseBySource = &name
			item.InUseActive = use.IsActive
		}
		out = append(out, item)
	}
	return out, nil
}

// SearchTwilioAvailableNumbers searches purchasable voice numbers.
func (s *Service) SearchTwilioAvailableNumbers(ctx context.Context, accountID, connectionID int64, numberType, areaCode, prefix string) ([]providers.TwilioAvailablePhoneNumber, error) {
	accountSID, authToken, err := s.twilioCreds(ctx, accountID, connectionID)
	if err != nil {
		return nil, err
	}
	params := url.Values{}
	switch strings.ToLower(numberType) {
	case "local":
		if len(areaCode) != 3 {
			return nil, httpx.Validation("area_code must be a 3-digit US area code")
		}
		params.Set("AreaCode", areaCode)
		numberType = "Local"
	case "tollfree":
		if prefix == "" {
			return nil, httpx.Validation("prefix is required for toll-free search")
		}
		params.Set("Contains", prefix)
		numberType = "TollFree"
	default:
		return nil, httpx.Validation("type must be local or tollfree")
	}
	numbers, err := providers.SearchAvailableNumbers(ctx, accountSID, authToken, "US", numberType, params)
	if err != nil {
		return nil, err
	}
	return numbers, nil
}

// TwilioMonthlyPrice returns an estimated monthly price in USD for a number type.
func (s *Service) TwilioMonthlyPrice(ctx context.Context, accountID, connectionID int64, numberType string) (*float64, error) {
	accountSID, authToken, err := s.twilioCreds(ctx, accountID, connectionID)
	if err != nil {
		return nil, err
	}
	priceType := "local"
	if strings.EqualFold(numberType, "tollfree") {
		priceType = "toll free"
	}
	return providers.MonthlyPriceEstimate(ctx, accountSID, authToken, "US", priceType)
}

// PurchaseTwilioPhoneNumber buys a phone number on the Twilio account.
func (s *Service) PurchaseTwilioPhoneNumber(ctx context.Context, accountID, connectionID int64, phoneNumber string) (providers.TwilioPhoneNumber, error) {
	phoneNumber = strings.TrimSpace(phoneNumber)
	if !e164Phone.MatchString(phoneNumber) {
		return providers.TwilioPhoneNumber{}, httpx.Validation("phone_number must be E.164 format")
	}
	accountSID, authToken, err := s.twilioCreds(ctx, accountID, connectionID)
	if err != nil {
		return providers.TwilioPhoneNumber{}, err
	}
	purchased, err := providers.PurchasePhoneNumber(ctx, accountSID, authToken, phoneNumber)
	if err != nil {
		return providers.TwilioPhoneNumber{}, httpx.Validation(err.Error())
	}
	purchased.NumberType = classifyTwilioNumberType(purchased.PhoneNumber)
	return purchased, nil
}

// ReleaseTwilioPhoneNumber removes a phone number from Twilio when not in use by an active call source.
func (s *Service) ReleaseTwilioPhoneNumber(ctx context.Context, accountID, connectionID int64, phoneNumberSID string) error {
	phoneNumberSID = strings.TrimSpace(phoneNumberSID)
	if phoneNumberSID == "" {
		return httpx.Validation("phone number sid is required")
	}
	accountSID, authToken, err := s.twilioCreds(ctx, accountID, connectionID)
	if err != nil {
		return err
	}
	bySID, _, err := s.twilioNumbersInUse(ctx, accountID, connectionID)
	if err != nil {
		return err
	}
	if use, ok := bySID[phoneNumberSID]; ok && use.IsActive {
		return httpx.Validation(fmt.Sprintf("number is in use by active call source %q", use.SourceName))
	}
	// Clear webhook before release in case it was configured via a call source.
	_ = providers.ClearVoiceWebhook(ctx, accountSID, authToken, phoneNumberSID)
	if err := providers.ReleasePhoneNumber(ctx, accountSID, authToken, phoneNumberSID); err != nil {
		return httpx.Validation(err.Error())
	}
	return nil
}

// CanDeleteTwilioConnection reports whether the connection can be disconnected.
func (s *Service) CanDeleteTwilioConnection(ctx context.Context, accountID, connectionID int64) error {
	conn, err := s.GetConnection(ctx, accountID, connectionID)
	if err != nil {
		return err
	}
	if conn.ProviderSlug != "twilio" {
		return nil
	}
	var sourceName string
	err = s.pool.QueryRow(ctx,
		`SELECT name FROM routing_sources
		 WHERE publisher_id = $1 AND type = 'call' AND integration_connection_id = $2 AND is_active
		 LIMIT 1`,
		accountID, connectionID).Scan(&sourceName)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	return httpx.Validation(fmt.Sprintf("disconnect blocked: active call source %q uses this connection", sourceName))
}

// SetTwilioVoiceWebhook configures inbound voice for a Twilio phone number SID.
func (s *Service) SetTwilioVoiceWebhook(ctx context.Context, accountID, connectionID int64, phoneNumberSID, voiceURL string) error {
	accountSID, authToken, err := s.twilioCreds(ctx, accountID, connectionID)
	if err != nil {
		return err
	}
	if err := providers.SetVoiceWebhook(ctx, accountSID, authToken, phoneNumberSID, voiceURL); err != nil {
		return fmt.Errorf("configure twilio voice webhook: %w", err)
	}
	return nil
}

// ClearTwilioVoiceWebhook removes inbound voice handling from a Twilio phone number SID.
func (s *Service) ClearTwilioVoiceWebhook(ctx context.Context, accountID, connectionID int64, phoneNumberSID string) error {
	accountSID, authToken, err := s.twilioCreds(ctx, accountID, connectionID)
	if err != nil {
		return err
	}
	if err := providers.ClearVoiceWebhook(ctx, accountSID, authToken, phoneNumberSID); err != nil {
		return fmt.Errorf("clear twilio voice webhook: %w", err)
	}
	return nil
}
