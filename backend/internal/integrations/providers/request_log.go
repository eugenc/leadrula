package providers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type HTTPRequestLog struct {
	Method  string            `json:"method"`
	URL     string            `json:"url"`
	Headers map[string]string `json:"headers,omitempty"`
	Body    json.RawMessage   `json:"body,omitempty"`
}

// DeliveryRequestLog is stored in integration_delivery_logs.request_body.
type DeliveryRequestLog struct {
	Mapped  map[string]string `json:"mapped"`
	Skipped map[string]string `json:"skipped,omitempty"`
	HTTP    HTTPRequestLog    `json:"http"`
}

var sensitiveHeaderNames = map[string]bool{
	"authorization":        true,
	"cookie":               true,
	"x-leadrula-signature": true,
}

func BuildDeliveryRequestLog(mapped map[string]string, req *http.Request, body []byte, skipped map[string]string) []byte {
	if mapped == nil {
		mapped = map[string]string{}
	}
	log := DeliveryRequestLog{
		Mapped: mapped,
		HTTP: HTTPRequestLog{
			Method: req.Method,
			URL:    req.URL.String(),
		},
	}
	if len(skipped) > 0 {
		log.Skipped = skipped
	}
	if len(body) > 0 {
		log.HTTP.Body = json.RawMessage(body)
	}
	if headers := safeHeaders(req); len(headers) > 0 {
		log.HTTP.Headers = headers
	}
	b, _ := json.Marshal(log)
	return b
}

func safeHeaders(req *http.Request) map[string]string {
	out := map[string]string{}
	for k, vs := range req.Header {
		if sensitiveHeaderNames[strings.ToLower(k)] {
			continue
		}
		if len(vs) > 0 {
			out[k] = vs[0]
		}
	}
	return out
}

func URLValuesToMapped(vals url.Values) map[string]string {
	out := make(map[string]string, len(vals))
	for k, vs := range vals {
		if len(vs) > 0 {
			out[k] = vs[len(vs)-1]
		}
	}
	return out
}

func StringMapToMapped(m map[string]string) map[string]string {
	if m == nil {
		return map[string]string{}
	}
	out := make(map[string]string, len(m))
	for k, v := range m {
		if v != "" {
			out[k] = v
		}
	}
	return out
}

func AnyMapToMapped(m map[string]any) map[string]string {
	out := make(map[string]string, len(m))
	for k, v := range m {
		s := valueToString(v)
		if s != "" {
			out[k] = s
		}
	}
	return out
}

func executeHTTP(req *http.Request, body []byte, mapped map[string]string) (*DeliveryResult, error) {
	reqLog := BuildDeliveryRequestLog(mapped, req, body, nil)
	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return &DeliveryResult{Request: reqLog, HTTPStatus: 0}, err
	}
	defer resp.Body.Close()
	buf := make([]byte, 4096)
	n, _ := resp.Body.Read(buf)
	raw := buf[:n]
	result := &DeliveryResult{
		Raw:        raw,
		Request:    reqLog,
		HTTPStatus: resp.StatusCode,
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return result, fmt.Errorf("http returned %d", resp.StatusCode)
	}
	return result, nil
}
