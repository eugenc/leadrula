package providers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"
)

const pipedriveBase = "https://api.pipedrive.com/v1"

type PipedriveProvider struct{}

func (p *PipedriveProvider) Slug() string { return "pipedrive" }

func (p *PipedriveProvider) Deliver(ctx context.Context, credentials []byte, payload DeliveryPayload) (*DeliveryResult, error) {
	token, err := oauthAccessToken(credentials)
	if err != nil {
		return nil, err
	}
	name := payload.FirstName + " " + payload.LastName
	if name == " " {
		name = payload.Phone
	}
	personBody := map[string]any{
		"name":  name,
		"phone": []map[string]string{{"value": payload.Phone}},
		"email": []map[string]string{{"value": payload.Email}},
	}
	body, _ := json.Marshal(personBody)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, pipedriveBase+"/persons", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var pr struct {
		Data struct {
			ID int `json:"id"`
		} `json:"data"`
		Success bool `json:"success"`
	}
	raw, _ := json.Marshal(&pr)
	_ = json.NewDecoder(resp.Body).Decode(&pr)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 || !pr.Success {
		return nil, fmt.Errorf("pipedrive returned %d", resp.StatusCode)
	}
	extID := strconv.Itoa(pr.Data.ID)
	if pipelineID, ok := payload.Config["pipeline_id"]; ok && extID != "0" {
		_ = createPipedriveDeal(ctx, token, pr.Data.ID, pipelineID, payload)
	}
	return &DeliveryResult{ExternalID: extID, Raw: raw}, nil
}

func createPipedriveDeal(ctx context.Context, token string, personID int, pipelineID any, payload DeliveryPayload) error {
	title := payload.FirstName + " " + payload.LastName
	deal := map[string]any{
		"title":     title,
		"person_id": personID,
	}
	if pid, ok := pipelineID.(float64); ok {
		deal["pipeline_id"] = int(pid)
	} else if ps, ok := pipelineID.(string); ok {
		deal["pipeline_id"] = ps
	}
	if stageID, ok := payload.Config["stage_id"]; ok {
		deal["stage_id"] = stageID
	}
	body, _ := json.Marshal(deal)
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, pipedriveBase+"/deals", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	resp.Body.Close()
	return nil
}

func (p *PipedriveProvider) ValidateCredentials(ctx context.Context, credentials []byte, config map[string]any) error {
	_, err := oauthAccessToken(credentials)
	return err
}
