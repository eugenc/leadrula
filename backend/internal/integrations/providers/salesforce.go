package providers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

type SalesforceProvider struct{}

func (p *SalesforceProvider) Slug() string { return "salesforce" }

func (p *SalesforceProvider) Deliver(ctx context.Context, credentials []byte, payload DeliveryPayload) (*DeliveryResult, error) {
	var creds struct {
		AccessToken string `json:"access_token"`
		InstanceURL string `json:"instance_url"`
	}
	if err := json.Unmarshal(credentials, &creds); err != nil {
		return nil, fmt.Errorf("invalid salesforce credentials: %w", err)
	}
	if creds.AccessToken == "" {
		return nil, fmt.Errorf("access_token required")
	}
	instanceURL := creds.InstanceURL
	if instanceURL == "" {
		if u, ok := payload.Config["instance_url"].(string); ok {
			instanceURL = u
		}
	}
	if instanceURL == "" {
		return nil, fmt.Errorf("instance_url required")
	}
	instanceURL = strings.TrimRight(instanceURL, "/")
	lastName := payload.LastName
	if lastName == "" {
		lastName = "Lead"
	}
	lead := map[string]any{
		"FirstName":  payload.FirstName,
		"LastName":   lastName,
		"Phone":      payload.Phone,
		"Email":      payload.Email,
		"Street":     payload.Address,
		"City":       payload.City,
		"State":      payload.State,
		"PostalCode": payload.Zip,
		"Company":    payload.Source,
	}
	if rt, ok := payload.Config["record_type_id"].(string); ok && rt != "" {
		lead["RecordTypeId"] = rt
	}
	body, _ := json.Marshal(lead)
	url := instanceURL + "/services/data/v59.0/sobjects/Lead/"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+creds.AccessToken)
	req.Header.Set("Content-Type", "application/json")
	result, err := executeHTTP(req, body, AnyMapToMapped(lead))
	if err != nil {
		return result, fmt.Errorf("salesforce returned %d", result.HTTPStatus)
	}
	var sf struct {
		ID      string `json:"id"`
		Success bool   `json:"success"`
	}
	_ = json.Unmarshal(result.Raw, &sf)
	if !sf.Success {
		return result, fmt.Errorf("salesforce returned %d", result.HTTPStatus)
	}
	result.ExternalID = sf.ID
	return result, nil
}

func (p *SalesforceProvider) ValidateCredentials(ctx context.Context, credentials []byte, config map[string]any) error {
	var creds struct {
		AccessToken string `json:"access_token"`
		InstanceURL string `json:"instance_url"`
	}
	if err := json.Unmarshal(credentials, &creds); err != nil || creds.AccessToken == "" {
		return fmt.Errorf("access_token required")
	}
	return nil
}
