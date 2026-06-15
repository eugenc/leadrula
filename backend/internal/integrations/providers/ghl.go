package providers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

const ghlBaseURL = "https://rest.gohighlevel.com/v1"

type GHLProvider struct{}

func (p *GHLProvider) Slug() string { return "ghl" }

func (p *GHLProvider) Deliver(ctx context.Context, credentials []byte, payload DeliveryPayload) (*DeliveryResult, error) {
	var creds struct {
		APIKey string `json:"api_key"`
	}
	if err := json.Unmarshal(credentials, &creds); err != nil {
		return nil, fmt.Errorf("invalid ghl credentials: %w", err)
	}
	locationID, _ := payload.Config["location_id"].(string)
	if locationID == "" {
		return nil, fmt.Errorf("location_id required in delivery config")
	}
	contact := map[string]any{
		"firstName":  payload.FirstName,
		"lastName":   payload.LastName,
		"phone":      payload.Phone,
		"email":      payload.Email,
		"address1":   payload.Address,
		"city":       payload.City,
		"state":      payload.State,
		"postalCode": payload.Zip,
		"source":     payload.Source,
		"locationId": locationID,
		"tags":       []string{"leadrula"},
	}
	for k, v := range payload.CustomFields {
		contact[k] = v
	}
	body, _ := json.Marshal(contact)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, ghlBaseURL+"/contacts/", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+creds.APIKey)
	req.Header.Set("Content-Type", "application/json")
	result, err := executeHTTP(req, body, AnyMapToMapped(contact))
	if err != nil {
		return result, fmt.Errorf("ghl returned %d", result.HTTPStatus)
	}
	var ghlResult struct {
		Contact struct {
			ID string `json:"id"`
		} `json:"contact"`
	}
	_ = json.Unmarshal(result.Raw, &ghlResult)
	pipelineID, hasPipeline := payload.Config["pipeline_id"].(string)
	stageID, hasStage := payload.Config["stage_id"].(string)
	if hasPipeline && hasStage && ghlResult.Contact.ID != "" {
		_ = createGHLOpportunity(ctx, creds.APIKey, locationID, ghlResult.Contact.ID, pipelineID, stageID, payload)
	}
	result.ExternalID = ghlResult.Contact.ID
	return result, nil
}

func createGHLOpportunity(ctx context.Context, apiKey, locationID, contactID, pipelineID, stageID string, payload DeliveryPayload) error {
	opp := map[string]any{
		"pipelineId":      pipelineID,
		"locationId":        locationID,
		"name":              payload.FirstName + " " + payload.LastName,
		"pipelineStageId":   stageID,
		"contactId":         contactID,
		"status":            "open",
		"source":            payload.Source,
	}
	body, _ := json.Marshal(opp)
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, ghlBaseURL+"/opportunities/", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	resp.Body.Close()
	return nil
}

func (p *GHLProvider) ValidateCredentials(ctx context.Context, credentials []byte, config map[string]any) error {
	var creds struct {
		APIKey string `json:"api_key"`
	}
	if err := json.Unmarshal(credentials, &creds); err != nil || creds.APIKey == "" {
		return fmt.Errorf("api_key is required")
	}
	if _, ok := config["location_id"]; !ok {
		return fmt.Errorf("location_id is required in config")
	}
	return nil
}
