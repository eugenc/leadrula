package providers

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/echayko/leadrula/backend/internal/customfields"
)

const DefaultSunbaseEndpoint = "https://server4.sunbasedata.com/sunbase/portal/api/lead_post.jsp"

var (
	sunbaseSchemaErr      = regexp.MustCompile(`(?i)schema_name`)
	sunbaseCustIDRe       = regexp.MustCompile(`cust-[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}`)
	sunbaseUUIDRe         = regexp.MustCompile(`[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}`)
	sunbaseHexIDRe        = regexp.MustCompile(`[0-9a-f]{32}`)
	sunbaseInsertedLeadRe = regexp.MustCompile(`(?i)successfully inserted lead\s+\S+\s+([0-9a-f]{32})`)
)

type SunbaseFieldMapEntry struct {
	DestKey          string  `json:"dest_key"`
	SourceType       string  `json:"source_type"`
	BuiltinField     *string `json:"builtin_field,omitempty"`
	CustomFieldID    *int64  `json:"custom_field_id,omitempty"`
	StaticValue      *string `json:"static_value,omitempty"`
	GHLCustomFieldID *string `json:"ghl_custom_field_id,omitempty"`
	GHLFieldModel    *string `json:"ghl_field_model,omitempty"`
	GHLMapSection    *string `json:"ghl_map_section,omitempty"`
}

type SunbaseProvider struct{}

func (p *SunbaseProvider) Slug() string { return "sunbase" }

func (p *SunbaseProvider) Deliver(ctx context.Context, credentials []byte, payload DeliveryPayload) (*DeliveryResult, error) {
	schemaName, endpointURL, err := parseSunbaseCreds(credentials, payload.Config)
	if err != nil {
		return nil, err
	}
	fieldMap := outboundFieldMapFromConfig(payload.Config)
	params, skipped := buildSunbaseParams(schemaName, fieldMap, payload)
	mapped := URLValuesToMapped(params)
	fullURL, err := sunbaseAppendQueryParams(endpointURL, params)
	if err != nil {
		return nil, err
	}
	return doSunbaseRequest(ctx, http.MethodPost, fullURL, mapped, skipped)
}

func (p *SunbaseProvider) ValidateCredentials(ctx context.Context, credentials []byte, config map[string]any) error {
	_, _, err := parseSunbaseCreds(credentials, config)
	return err
}

func (p *SunbaseProvider) TestConnection(ctx context.Context, credentials []byte, config map[string]any) error {
	schemaName, endpointURL, err := parseSunbaseCreds(credentials, config)
	if err != nil {
		return err
	}
	params := url.Values{}
	params.Set("schema_name", schemaName)
	params.Set("last_name", "ConnectionTest")
	params.Set("first_name", "Leadrula")
	mapped := URLValuesToMapped(params)
	fullURL, err := sunbaseAppendQueryParams(endpointURL, params)
	if err != nil {
		return err
	}
	_, err = doSunbaseRequest(ctx, http.MethodPost, fullURL, mapped, nil)
	return err
}

func parseSunbaseCreds(credentials []byte, config map[string]any) (schemaName, endpointURL string, err error) {
	var creds struct {
		SchemaName string `json:"schema_name"`
	}
	if err = json.Unmarshal(credentials, &creds); err != nil || strings.TrimSpace(creds.SchemaName) == "" {
		return "", "", fmt.Errorf("schema_name is required")
	}
	endpointURL = DefaultSunbaseEndpoint
	if config != nil {
		if u, ok := config["endpoint_url"].(string); ok && strings.TrimSpace(u) != "" {
			endpointURL = strings.TrimSpace(u)
		}
	}
	return creds.SchemaName, endpointURL, nil
}

func outboundFieldMapFromConfig(config map[string]any) []SunbaseFieldMapEntry {
	if config == nil {
		return DefaultSunbaseOutboundFieldMap("")
	}
	raw, ok := config["outbound_field_map"]
	if !ok {
		return DefaultSunbaseOutboundFieldMap("")
	}
	b, err := json.Marshal(raw)
	if err != nil {
		return DefaultSunbaseOutboundFieldMap("")
	}
	var entries []SunbaseFieldMapEntry
	if json.Unmarshal(b, &entries) != nil || len(entries) == 0 {
		return DefaultSunbaseOutboundFieldMap("")
	}
	return entries
}

func DefaultSunbaseOutboundFieldMap(schemaName string) []SunbaseFieldMapEntry {
	s := schemaName
	return []SunbaseFieldMapEntry{
		{DestKey: "schema_name", SourceType: "static", StaticValue: &s},
		{DestKey: "last_name", SourceType: "builtin", BuiltinField: strPtr("last_name")},
		{DestKey: "first_name", SourceType: "builtin", BuiltinField: strPtr("first_name")},
		{DestKey: "address1", SourceType: "builtin", BuiltinField: strPtr("address")},
		{DestKey: "city", SourceType: "builtin", BuiltinField: strPtr("city")},
		{DestKey: "state", SourceType: "builtin", BuiltinField: strPtr("state")},
		{DestKey: "zip_code", SourceType: "builtin", BuiltinField: strPtr("zip")},
		{DestKey: "email", SourceType: "builtin", BuiltinField: strPtr("email")},
		{DestKey: "phone", SourceType: "builtin", BuiltinField: strPtr("phone")},
		{DestKey: "lead_source", SourceType: "builtin", BuiltinField: strPtr("source")},
		{DestKey: "lead_other", SourceType: "builtin", BuiltinField: strPtr("external_id")},
	}
}

func strPtr(s string) *string { return &s }

func buildSunbaseParams(schemaName string, entries []SunbaseFieldMapEntry, payload DeliveryPayload) (url.Values, map[string]string) {
	vals := url.Values{}
	skipped := map[string]string{}
	for _, e := range entries {
		if e.DestKey == "" {
			continue
		}
		v := resolveSunbaseFieldValue(e, payload)
		if v == "" && e.DestKey == "schema_name" {
			v = schemaName
		}
		if v != "" {
			vals.Set(e.DestKey, v)
			continue
		}
		if e.DestKey == "last_name" && payload.LastName != "" {
			vals.Set("last_name", payload.LastName)
			continue
		}
		skipped[e.DestKey] = sunbaseSkipReason(e, payload)
	}
	if vals.Get("schema_name") == "" {
		vals.Set("schema_name", schemaName)
		delete(skipped, "schema_name")
	}
	if vals.Get("last_name") == "" && payload.LastName != "" {
		vals.Set("last_name", payload.LastName)
		delete(skipped, "last_name")
	}
	if vals.Get("lead_id") == "" && strings.TrimSpace(payload.LeadID) != "" {
		vals.Set("lead_id", payload.LeadID)
	}
	return vals, skipped
}

func sunbaseSkipReason(e SunbaseFieldMapEntry, payload DeliveryPayload) string {
	if e.SourceType == "custom" && e.CustomFieldID != nil {
		key := fmt.Sprintf("%d", *e.CustomFieldID)
		if payload.CustomFields == nil {
			return "missing custom field value"
		}
		if _, ok := payload.CustomFields[key]; !ok {
			return "missing custom field value"
		}
	}
	return "empty"
}

func resolveSunbaseFieldValue(e SunbaseFieldMapEntry, payload DeliveryPayload) string {
	switch e.SourceType {
	case "static":
		if e.StaticValue != nil {
			return *e.StaticValue
		}
	case "builtin":
		if e.BuiltinField == nil {
			return ""
		}
		switch *e.BuiltinField {
		case "first_name":
			return payload.FirstName
		case "last_name":
			return payload.LastName
		case "phone":
			return payload.Phone
		case "email":
			return payload.Email
		case "address":
			return payload.Address
		case "city":
			return payload.City
		case "state":
			return payload.State
		case "zip":
			return payload.Zip
		case "source":
			return payload.Source
		case "action_at":
			if payload.ActionAt == "" {
				return ""
			}
			return customfields.FormatForSunbaseExportInTimezone("datetime", payload.ActionAt, sunbaseAccountTimezone(payload.Config))
		case "lead_id", "public_id":
			return payload.LeadID
		case "external_id":
			if payload.Config != nil {
				if v, ok := payload.Config["external_id"].(string); ok {
					return v
				}
			}
		}
	case "custom":
		if e.CustomFieldID != nil && payload.CustomFields != nil {
			key := fmt.Sprintf("%d", *e.CustomFieldID)
			if v, ok := payload.CustomFields[key]; ok {
				s := valueToQueryString(v)
				if ftype := sunbaseCustomFieldType(payload.Config, key); ftype == "date" || ftype == "datetime" {
					s = customfields.FormatForSunbaseExport(ftype, s)
				}
				return s
			}
		}
	}
	return ""
}

func sunbaseAccountTimezone(config map[string]any) string {
	if config == nil {
		return ""
	}
	if v, ok := config["account_timezone"].(string); ok {
		return v
	}
	return ""
}

func sunbaseCustomFieldType(config map[string]any, fieldID string) string {
	if config == nil {
		return ""
	}
	raw, ok := config["custom_field_types"]
	if !ok {
		return ""
	}
	if m, ok := raw.(map[string]string); ok {
		return m[fieldID]
	}
	m, ok := raw.(map[string]any)
	if !ok {
		return ""
	}
	v, ok := m[fieldID]
	if !ok {
		return ""
	}
	s, _ := v.(string)
	return s
}

func valueToQueryString(v any) string {
	switch t := v.(type) {
	case string:
		return t
	case float64:
		return fmt.Sprintf("%g", t)
	case bool:
		if t {
			return "true"
		}
		return "false"
	default:
		b, _ := json.Marshal(t)
		return strings.Trim(string(b), `"`)
	}
}

func sunbaseAppendQueryParams(baseURL string, params url.Values) (string, error) {
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

func doSunbaseRequest(ctx context.Context, method, fullURL string, mapped, skipped map[string]string) (*DeliveryResult, error) {
	req, err := http.NewRequestWithContext(ctx, method, fullURL, nil)
	if err != nil {
		return nil, err
	}
	reqLog := BuildDeliveryRequestLog(mapped, req, nil, skipped)
	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return &DeliveryResult{Request: reqLog, HTTPStatus: 0}, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	raw := body
	text := string(body)
	result := &DeliveryResult{Raw: raw, Request: reqLog, HTTPStatus: resp.StatusCode}
	if sunbaseSchemaErr.MatchString(text) && strings.Contains(strings.ToLower(text), "not specified") {
		return result, fmt.Errorf("invalid schema_name")
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return result, fmt.Errorf("sunbase returned %d: %s", resp.StatusCode, sunbaseBodySnippet(text))
	}
	if errMsg := sunbaseErrorFromBody(text); errMsg != "" {
		return result, fmt.Errorf("sunbase: %s", errMsg)
	}
	extID := parseSunbaseExternalID(body)
	if extID == "" && sunbaseBodyLooksLikeError(text) {
		return result, fmt.Errorf("sunbase: %s", sunbaseBodySnippet(text))
	}
	result.ExternalID = extID
	return result, nil
}

func sunbaseBodySnippet(text string) string {
	snippet := strings.TrimSpace(text)
	if len(snippet) > 200 {
		snippet = snippet[:200]
	}
	return snippet
}

func sunbaseErrorFromBody(text string) string {
	lower := strings.ToLower(text)
	if strings.Contains(lower, "unable to find site") {
		return sunbaseBodySnippet(text)
	}
	return ""
}

func sunbaseBodyLooksLikeError(text string) bool {
	lower := strings.ToLower(strings.TrimSpace(text))
	if lower == "" {
		return false
	}
	for _, phrase := range []string{"unable to find", "not found", "invalid", "error", "failed", "denied"} {
		if strings.Contains(lower, phrase) {
			return true
		}
	}
	return false
}

func parseSunbaseExternalID(body []byte) string {
	if len(body) == 0 {
		return ""
	}
	var obj map[string]any
	if json.Unmarshal(body, &obj) == nil {
		for _, key := range []string{"uuid", "id"} {
			if v, ok := obj[key]; ok {
				if s := strings.TrimSpace(anyToString(v)); isSunbaseCustomerID(s) {
					return s
				}
			}
		}
	}
	text := string(body)
	if m := sunbaseCustIDRe.FindString(text); m != "" {
		return m
	}
	if m := sunbaseUUIDRe.FindString(text); m != "" {
		return m
	}
	if m := sunbaseInsertedLeadRe.FindStringSubmatch(text); len(m) > 1 {
		return m[1]
	}
	if strings.Contains(strings.ToLower(text), "successfully inserted") {
		if m := sunbaseHexIDRe.FindString(text); m != "" {
			return m
		}
	}
	return ""
}

func isSunbaseCustomerID(s string) bool {
	if s == "" {
		return false
	}
	if strings.HasPrefix(strings.ToLower(s), "cust-") {
		return sunbaseCustIDRe.MatchString(s)
	}
	if sunbaseUUIDRe.MatchString(s) {
		return true
	}
	return sunbaseHexIDRe.MatchString(s)
}

// NormalizeSunbaseExternalID returns the canonical 32-char hex ID when the input is a valid SunBase customer ID.
func NormalizeSunbaseExternalID(raw string) string {
	s := strings.TrimSpace(raw)
	if s == "" {
		return ""
	}
	if hex := sunbaseIDToHex(s); hex != "" {
		return hex
	}
	return s
}

// SunbaseExternalIDCandidates returns lookup variants for the same SunBase customer ID.
func SunbaseExternalIDCandidates(raw string) []string {
	s := strings.TrimSpace(raw)
	if s == "" {
		return nil
	}
	seen := map[string]struct{}{}
	var out []string
	add := func(v string) {
		v = strings.TrimSpace(v)
		if v == "" {
			return
		}
		if _, ok := seen[v]; ok {
			return
		}
		seen[v] = struct{}{}
		out = append(out, v)
	}
	add(s)
	if !isSunbaseCustomerID(s) {
		return out
	}
	if hex := sunbaseIDToHex(s); hex != "" {
		add(hex)
		add(sunbaseHexToCust(hex))
		add(sunbaseHexToDashed(hex))
	}
	return out
}

func sunbaseIDToHex(s string) string {
	lower := strings.ToLower(strings.TrimSpace(s))
	if strings.HasPrefix(lower, "cust-") {
		lower = strings.TrimPrefix(lower, "cust-")
	}
	compact := strings.ReplaceAll(lower, "-", "")
	if sunbaseHexIDRe.MatchString(compact) {
		return compact
	}
	return ""
}

func sunbaseHexToCust(hex string) string {
	hex = strings.ToLower(strings.ReplaceAll(hex, "-", ""))
	if len(hex) != 32 || !sunbaseHexIDRe.MatchString(hex) {
		return ""
	}
	return fmt.Sprintf("cust-%s-%s-%s-%s-%s", hex[0:8], hex[8:12], hex[12:16], hex[16:20], hex[20:32])
}

func sunbaseHexToDashed(hex string) string {
	hex = strings.ToLower(strings.ReplaceAll(hex, "-", ""))
	if len(hex) != 32 || !sunbaseHexIDRe.MatchString(hex) {
		return ""
	}
	return fmt.Sprintf("%s-%s-%s-%s-%s", hex[0:8], hex[8:12], hex[12:16], hex[16:20], hex[20:32])
}

func anyToString(v any) string {
	switch x := v.(type) {
	case string:
		return x
	case float64:
		if x == float64(int64(x)) {
			return fmt.Sprintf("%d", int64(x))
		}
		return fmt.Sprintf("%g", x)
	default:
		b, _ := json.Marshal(v)
		return strings.Trim(string(b), `"`)
	}
}
