package providers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

const hubspotBase = "https://api.hubapi.com"

type HubSpotProvider struct{}

func (p *HubSpotProvider) Slug() string { return "hubspot" }

func (p *HubSpotProvider) Deliver(ctx context.Context, credentials []byte, payload DeliveryPayload) (*DeliveryResult, error) {
	token, err := oauthAccessToken(credentials)
	if err != nil {
		return nil, err
	}
	props := map[string]string{
		"firstname": payload.FirstName,
		"lastname":  payload.LastName,
		"phone":     payload.Phone,
		"email":     payload.Email,
		"address":   payload.Address,
		"city":      payload.City,
		"state":     payload.State,
		"zip":       payload.Zip,
	}
	if ls, ok := payload.Config["lifecyclestage"].(string); ok && ls != "" {
		props["lifecyclestage"] = ls
	}
	body, _ := json.Marshal(map[string]any{"properties": props})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, hubspotBase+"/crm/v3/objects/contacts", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var result struct {
		ID string `json:"id"`
	}
	raw, _ := json.Marshal(&result)
	_ = json.NewDecoder(resp.Body).Decode(&result)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("hubspot returned %d", resp.StatusCode)
	}
	return &DeliveryResult{ExternalID: result.ID, Raw: raw}, nil
}

func (p *HubSpotProvider) ValidateCredentials(ctx context.Context, credentials []byte, config map[string]any) error {
	_, err := oauthAccessToken(credentials)
	return err
}
