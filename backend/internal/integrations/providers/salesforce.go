package providers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
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
		"FirstName": payload.FirstName,
		"LastName":  lastName,
		"Phone":     payload.Phone,
		"Email":     payload.Email,
		"Street":    payload.Address,
		"City":      payload.City,
		"State":     payload.State,
		"PostalCode": payload.Zip,
		"Company":   payload.Source,
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
	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var result struct {
		ID      string `json:"id"`
		Success bool   `json:"success"`
	}
	raw, _ := json.Marshal(&result)
	_ = json.NewDecoder(resp.Body).Decode(&result)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 || !result.Success {
		return nil, fmt.Errorf("salesforce returned %d", resp.StatusCode)
	}
	return &DeliveryResult{ExternalID: result.ID, Raw: raw}, nil
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
