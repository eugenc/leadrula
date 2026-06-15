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
	"net/url"
	"strconv"
	"strings"
)

type WebhookProvider struct{}

func (p *WebhookProvider) Slug() string { return "webhook" }

func (p *WebhookProvider) Deliver(ctx context.Context, credentials []byte, payload DeliveryPayload) (*DeliveryResult, error) {
	raw, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	return DeliverWebhook(ctx, credentials, raw)
}

// DeliverWebhook sends an outbound webhook using raw rendered JSON bytes.
func DeliverWebhook(ctx context.Context, credentials, rawJSON []byte) (*DeliveryResult, error) {
	var creds struct {
		URL     string            `json:"url"`
		Secret  string            `json:"secret"`
		Format  string            `json:"format"`
		Method  string            `json:"method"`
		Headers map[string]string `json:"headers"`
	}
	if err := json.Unmarshal(credentials, &creds); err != nil {
		return nil, fmt.Errorf("invalid webhook credentials: %w", err)
	}
	if creds.URL == "" {
		return nil, fmt.Errorf("url is required")
	}
	format := creds.Format
	if format == "" {
		format = "json"
	}
	method := strings.ToUpper(creds.Method)
	if method == "" {
		method = http.MethodPost
	}

	useQuery := method == http.MethodGet || format == "url"
	if useQuery {
		return deliverQueryParams(ctx, creds.URL, method, creds.Secret, creds.Headers, rawJSON)
	}
	return deliverJSONBody(ctx, creds.URL, creds.Secret, creds.Headers, rawJSON)
}

func deliverJSONBody(ctx context.Context, baseURL, secret string, headers map[string]string, rawJSON []byte) (*DeliveryResult, error) {
	mapped, err := jsonPayloadToMapped(rawJSON)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL, bytes.NewReader(rawJSON))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	if secret != "" {
		req.Header.Set("X-Leadrula-Signature", hmacSHA256(rawJSON, secret))
	}
	result, err := executeHTTP(req, rawJSON, mapped)
	if err != nil {
		return result, fmt.Errorf("webhook returned %d", result.HTTPStatus)
	}
	return result, nil
}

func deliverQueryParams(ctx context.Context, baseURL, method, secret string, headers map[string]string, rawJSON []byte) (*DeliveryResult, error) {
	params, err := payloadToQueryValues(rawJSON)
	if err != nil {
		return nil, err
	}
	mapped := URLValuesToMapped(params)
	fullURL, err := appendQueryParams(baseURL, params)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, method, fullURL, nil)
	if err != nil {
		return nil, err
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	if secret != "" {
		req.Header.Set("X-Leadrula-Signature", hmacSHA256([]byte(fullURL), secret))
	}
	result, err := executeHTTP(req, nil, mapped)
	if err != nil {
		return result, fmt.Errorf("webhook returned %d", result.HTTPStatus)
	}
	return result, nil
}

func jsonPayloadToMapped(rawJSON []byte) (map[string]string, error) {
	var raw map[string]any
	if err := json.Unmarshal(rawJSON, &raw); err != nil {
		return nil, fmt.Errorf("invalid payload json: %w", err)
	}
	return AnyMapToMapped(raw), nil
}

func payloadToQueryValues(rawJSON []byte) (url.Values, error) {
	var raw map[string]any
	if err := json.Unmarshal(rawJSON, &raw); err != nil {
		return nil, fmt.Errorf("invalid payload json: %w", err)
	}
	vals := url.Values{}
	for k, v := range raw {
		s := valueToString(v)
		if s != "" {
			vals.Set(k, s)
		}
	}
	return vals, nil
}

func valueToString(v any) string {
	switch t := v.(type) {
	case nil:
		return ""
	case string:
		return t
	case float64:
		if t == float64(int64(t)) {
			return strconv.FormatInt(int64(t), 10)
		}
		return strconv.FormatFloat(t, 'f', -1, 64)
	case bool:
		return strconv.FormatBool(t)
	default:
		b, _ := json.Marshal(t)
		return string(b)
	}
}

func appendQueryParams(baseURL string, params url.Values) (string, error) {
	u, err := url.Parse(baseURL)
	if err != nil {
		return "", err
	}
	q := u.Query()
	for k, vs := range params {
		for _, v := range vs {
			q.Add(k, v)
		}
	}
	u.RawQuery = q.Encode()
	return u.String(), nil
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
