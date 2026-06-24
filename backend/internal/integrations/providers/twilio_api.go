package providers

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// TwilioPhoneNumber is a voice-capable incoming number on a Twilio account.
type TwilioPhoneNumber struct {
	SID           string  `json:"sid"`
	PhoneNumber   string  `json:"phone_number"`
	FriendlyName  string  `json:"friendly_name"`
	NumberType    string  `json:"number_type,omitempty"`
	InUseBySource *string `json:"in_use_by_source,omitempty"`
	InUseActive   bool    `json:"in_use_active,omitempty"`
}

// TwilioAvailablePhoneNumber is a purchasable phone number from Twilio search.
type TwilioAvailablePhoneNumber struct {
	PhoneNumber string `json:"phone_number"`
	FriendlyName string `json:"friendly_name,omitempty"`
	Locality    string `json:"locality,omitempty"`
	Region      string `json:"region,omitempty"`
	NumberType  string `json:"number_type"`
}

type twilioListResponse struct {
	IncomingPhoneNumbers []struct {
		SID          string `json:"sid"`
		PhoneNumber  string `json:"phone_number"`
		FriendlyName string `json:"friendly_name"`
		Capabilities struct {
			Voice bool `json:"voice"`
		} `json:"capabilities"`
	} `json:"incoming_phone_numbers"`
}

// ListVoiceNumbers returns voice-capable incoming phone numbers for the account.
func ListVoiceNumbers(ctx context.Context, accountSID, authToken string) ([]TwilioPhoneNumber, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		fmt.Sprintf("https://api.twilio.com/2010-04-01/Accounts/%s/IncomingPhoneNumbers.json", accountSID), nil)
	if err != nil {
		return nil, err
	}
	req.SetBasicAuth(accountSID, authToken)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("twilio request failed: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("twilio list numbers: %s", twilioErrMsg(body))
	}
	var parsed twilioListResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, fmt.Errorf("twilio list numbers: invalid response")
	}
	out := make([]TwilioPhoneNumber, 0, len(parsed.IncomingPhoneNumbers))
	for _, n := range parsed.IncomingPhoneNumbers {
		if !n.Capabilities.Voice {
			continue
		}
		out = append(out, TwilioPhoneNumber{
			SID:          n.SID,
			PhoneNumber:  n.PhoneNumber,
			FriendlyName: n.FriendlyName,
		})
	}
	return out, nil
}

type twilioAvailableSearchResponse struct {
	AvailablePhoneNumbers []struct {
		PhoneNumber  string `json:"phone_number"`
		FriendlyName string `json:"friendly_name"`
		Locality     string `json:"locality"`
		Region       string `json:"region"`
	} `json:"available_phone_numbers"`
}

// SearchAvailableNumbers finds purchasable voice numbers (Local or TollFree) in a country.
func SearchAvailableNumbers(ctx context.Context, accountSID, authToken, country, numberType string, params url.Values) ([]TwilioAvailablePhoneNumber, error) {
	if params == nil {
		params = url.Values{}
	}
	params.Set("VoiceEnabled", "true")
	if params.Get("Limit") == "" {
		params.Set("Limit", "20")
	}
	u := fmt.Sprintf("https://api.twilio.com/2010-04-01/Accounts/%s/AvailablePhoneNumbers/%s/%s.json?%s",
		accountSID, country, numberType, params.Encode())
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	req.SetBasicAuth(accountSID, authToken)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("twilio search numbers: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("twilio search numbers: %s", twilioErrMsg(body))
	}
	var parsed twilioAvailableSearchResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, fmt.Errorf("twilio search numbers: invalid response")
	}
	out := make([]TwilioAvailablePhoneNumber, 0, len(parsed.AvailablePhoneNumbers))
	typeLabel := strings.ToLower(numberType)
	for _, n := range parsed.AvailablePhoneNumbers {
		out = append(out, TwilioAvailablePhoneNumber{
			PhoneNumber:  n.PhoneNumber,
			FriendlyName: n.FriendlyName,
			Locality:     n.Locality,
			Region:       n.Region,
			NumberType:   typeLabel,
		})
	}
	return out, nil
}

// PurchasePhoneNumber buys an incoming phone number on the account.
func PurchasePhoneNumber(ctx context.Context, accountSID, authToken, phoneNumberE164 string) (TwilioPhoneNumber, error) {
	form := url.Values{"PhoneNumber": {phoneNumberE164}}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		fmt.Sprintf("https://api.twilio.com/2010-04-01/Accounts/%s/IncomingPhoneNumbers.json", accountSID),
		strings.NewReader(form.Encode()))
	if err != nil {
		return TwilioPhoneNumber{}, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth(accountSID, authToken)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return TwilioPhoneNumber{}, fmt.Errorf("twilio purchase number: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 8192))
	if resp.StatusCode >= 400 {
		return TwilioPhoneNumber{}, fmt.Errorf("twilio purchase number: %s", twilioErrMsg(body))
	}
	var n struct {
		SID          string `json:"sid"`
		PhoneNumber  string `json:"phone_number"`
		FriendlyName string `json:"friendly_name"`
	}
	if err := json.Unmarshal(body, &n); err != nil {
		return TwilioPhoneNumber{}, fmt.Errorf("twilio purchase number: invalid response")
	}
	return TwilioPhoneNumber{
		SID:          n.SID,
		PhoneNumber:  n.PhoneNumber,
		FriendlyName: n.FriendlyName,
	}, nil
}

// ReleasePhoneNumber removes an incoming phone number from the account.
func ReleasePhoneNumber(ctx context.Context, accountSID, authToken, phoneNumberSID string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete,
		fmt.Sprintf("https://api.twilio.com/2010-04-01/Accounts/%s/IncomingPhoneNumbers/%s.json", accountSID, phoneNumberSID), nil)
	if err != nil {
		return err
	}
	req.SetBasicAuth(accountSID, authToken)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("twilio release number: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 8192))
	if resp.StatusCode >= 400 {
		return fmt.Errorf("twilio release number: %s", twilioErrMsg(body))
	}
	return nil
}

// MonthlyPriceEstimate returns the monthly price for a number type in a country (USD).
func MonthlyPriceEstimate(ctx context.Context, accountSID, authToken, country, numberType string) (*float64, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		fmt.Sprintf("https://pricing.twilio.com/v1/PhoneNumbers/Countries/%s", country), nil)
	if err != nil {
		return nil, err
	}
	req.SetBasicAuth(accountSID, authToken)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("twilio pricing: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 8192))
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("twilio pricing: %s", twilioErrMsg(body))
	}
	var parsed struct {
		PhoneNumberPrices []struct {
			NumberType    string `json:"number_type"`
			CurrentPrice  string `json:"current_price"`
		} `json:"phone_number_prices"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, fmt.Errorf("twilio pricing: invalid response")
	}
	want := strings.ToLower(numberType)
	if want == "tollfree" {
		want = "toll free"
	}
	for _, p := range parsed.PhoneNumberPrices {
		if strings.EqualFold(strings.ReplaceAll(p.NumberType, "_", " "), want) ||
			strings.EqualFold(p.NumberType, numberType) {
			var price float64
			if _, err := fmt.Sscanf(p.CurrentPrice, "%f", &price); err == nil {
				return &price, nil
			}
		}
	}
	return nil, nil
}

// SetVoiceWebhook configures inbound voice for an IncomingPhoneNumber resource.
func SetVoiceWebhook(ctx context.Context, accountSID, authToken, phoneNumberSID, voiceURL string) error {
	return updateIncomingNumber(ctx, accountSID, authToken, phoneNumberSID, url.Values{
		"VoiceUrl":    {voiceURL},
		"VoiceMethod": {"POST"},
	})
}

// ClearVoiceWebhook removes inbound voice handling from an IncomingPhoneNumber.
func ClearVoiceWebhook(ctx context.Context, accountSID, authToken, phoneNumberSID string) error {
	return updateIncomingNumber(ctx, accountSID, authToken, phoneNumberSID, url.Values{
		"VoiceUrl":    {""},
		"VoiceMethod": {""},
	})
}

func updateIncomingNumber(ctx context.Context, accountSID, authToken, phoneNumberSID string, form url.Values) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		fmt.Sprintf("https://api.twilio.com/2010-04-01/Accounts/%s/IncomingPhoneNumbers/%s.json", accountSID, phoneNumberSID),
		strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth(accountSID, authToken)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("twilio update number: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 8192))
	if resp.StatusCode >= 400 {
		return fmt.Errorf("twilio update number: %s", twilioErrMsg(body))
	}
	return nil
}

func twilioErrMsg(body []byte) string {
	var errResp struct {
		Message string `json:"message"`
	}
	if json.Unmarshal(body, &errResp) == nil && errResp.Message != "" {
		return errResp.Message
	}
	return strings.TrimSpace(string(body))
}
