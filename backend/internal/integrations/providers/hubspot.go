package providers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
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
	result, err := executeHTTP(req, body, StringMapToMapped(props))
	if err != nil {
		return result, fmt.Errorf("hubspot returned %d", result.HTTPStatus)
	}
	var hs struct {
		ID string `json:"id"`
	}
	_ = json.Unmarshal(result.Raw, &hs)
	result.ExternalID = hs.ID
	return result, nil
}

func (p *HubSpotProvider) ValidateCredentials(ctx context.Context, credentials []byte, config map[string]any) error {
	_, err := oauthAccessToken(credentials)
	return err
}
