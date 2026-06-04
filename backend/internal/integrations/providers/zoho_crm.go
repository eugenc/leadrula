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

type ZohoCRMProvider struct{}

func (p *ZohoCRMProvider) Slug() string { return "zoho_crm" }

func (p *ZohoCRMProvider) Deliver(ctx context.Context, credentials []byte, payload DeliveryPayload) (*DeliveryResult, error) {
	token, err := oauthAccessToken(credentials)
	if err != nil {
		return nil, err
	}
	domain, _ := payload.Config["api_domain"].(string)
	if domain == "" {
		domain = "com"
	}
	base := "https://www.zohoapis." + strings.TrimPrefix(domain, ".")
	lastName := payload.LastName
	if lastName == "" {
		lastName = payload.Phone
	}
	if lastName == "" {
		lastName = "Lead"
	}
	record := map[string]any{
		"Last_Name":   lastName,
		"First_Name":  payload.FirstName,
		"Phone":       payload.Phone,
		"Email":       payload.Email,
		"Street":      payload.Address,
		"City":        payload.City,
		"State":       payload.State,
		"Zip_Code":    payload.Zip,
		"Lead_Source": payload.Source,
	}
	body, _ := json.Marshal(map[string]any{"data": []map[string]any{record}})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, base+"/crm/v6/Leads", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Zoho-oauthtoken "+token)
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var result struct {
		Data []struct {
			Details struct {
				ID string `json:"id"`
			} `json:"details"`
			Status string `json:"status"`
		} `json:"data"`
	}
	raw, _ := json.Marshal(&result)
	_ = json.NewDecoder(resp.Body).Decode(&result)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("zoho returned %d", resp.StatusCode)
	}
	extID := ""
	if len(result.Data) > 0 {
		extID = result.Data[0].Details.ID
	}
	return &DeliveryResult{ExternalID: extID, Raw: raw}, nil
}

func (p *ZohoCRMProvider) ValidateCredentials(ctx context.Context, credentials []byte, config map[string]any) error {
	if _, err := oauthAccessToken(credentials); err != nil {
		return err
	}
	if d, _ := config["api_domain"].(string); d == "" {
		return fmt.Errorf("api_domain is required in config")
	}
	return nil
}
