package providers

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type WebhookProvider struct{}

func (p *WebhookProvider) Slug() string { return "webhook" }

func (p *WebhookProvider) Deliver(ctx context.Context, credentials []byte, payload DeliveryPayload) (*DeliveryResult, error) {
	var creds struct {
		URL     string            `json:"url"`
		Secret  string            `json:"secret"`
		Headers map[string]string `json:"headers"`
	}
	if err := json.Unmarshal(credentials, &creds); err != nil {
		return nil, fmt.Errorf("invalid webhook credentials: %w", err)
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, creds.URL, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	for k, v := range creds.Headers {
		req.Header.Set(k, v)
	}
	if creds.Secret != "" {
		req.Header.Set("X-Leadrula-Signature", hmacSHA256(body, creds.Secret))
	}
	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	buf := make([]byte, 4096)
	n, _ := resp.Body.Read(buf)
	raw := buf[:n]
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("webhook returned %d", resp.StatusCode)
	}
	return &DeliveryResult{Raw: raw}, nil
}

func (p *WebhookProvider) ValidateCredentials(ctx context.Context, credentials []byte, config map[string]any) error {
	var creds struct {
		URL string `json:"url"`
	}
	if err := json.Unmarshal(credentials, &creds); err != nil || creds.URL == "" {
		return fmt.Errorf("url is required")
	}
	return nil
}

func hmacSHA256(body []byte, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	return hex.EncodeToString(mac.Sum(nil))
}
