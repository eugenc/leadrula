package providers

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/echayko/leadrula/backend/internal/customfields"
)

var ghlTitlePlaceholderRe = regexp.MustCompile(`\{\{([^}]+)\}\}`)

const ghlAPIVersion = "2021-07-28"

type GHLFieldSource struct {
	SourceType    string  `json:"source_type"`
	BuiltinField  *string `json:"builtin_field,omitempty"`
	CustomFieldID *int64  `json:"custom_field_id,omitempty"`
	StaticValue   *string `json:"static_value,omitempty"`
}

type GHLPipelineStageMapEntry struct {
	LeadrulaPipelineID int64  `json:"leadrula_pipeline_id"`
	LeadrulaStageID    int64  `json:"leadrula_stage_id"`
	GHLPipelineID      string `json:"ghl_pipeline_id"`
	GHLPipelineStageID string `json:"ghl_pipeline_stage_id"`
}

type GHLConfig struct {
	DeliveryMode             string
	WebhookURL               string
	LocationID               string
	CreateContact            bool
	CreateOpportunity        bool
	CreateAppointment        bool
	CalendarID               string
	AppointmentTimezone      string
	AppointmentDatetime      GHLFieldSource
	AppointmentTitleTemplate string
	OpportunityTitleTemplate string
	AppointmentNotes         *GHLFieldSource
	PipelineStageMap         []GHLPipelineStageMapEntry
	OutboundFieldMap         []SunbaseFieldMapEntry
}

func ParseGHLCredentials(credentials []byte) (token string, err error) {
	var creds struct {
		PrivateIntegrationToken string `json:"private_integration_token"`
		APIKey                  string `json:"api_key"`
	}
	if err = json.Unmarshal(credentials, &creds); err != nil {
		return "", fmt.Errorf("invalid ghl credentials: %w", err)
	}
	token = strings.TrimSpace(creds.PrivateIntegrationToken)
	if token == "" {
		token = strings.TrimSpace(creds.APIKey)
	}
	if token == "" {
		return "", fmt.Errorf("private_integration_token is required")
	}
	return token, nil
}

func ParseGHLConfig(config map[string]any) (GHLConfig, error) {
	out := GHLConfig{CreateContact: true}
	if config == nil {
		config = map[string]any{}
	}
	out.DeliveryMode = parseGHLDeliveryMode(config)
	if s, ok := config["webhook_url"].(string); ok {
		out.WebhookURL = strings.TrimSpace(s)
	}
	if loc, ok := config["location_id"].(string); ok {
		out.LocationID = strings.TrimSpace(loc)
	}
	if v, ok := config["create_contact"].(bool); ok {
		out.CreateContact = v
	}
	if !out.CreateContact {
		return out, fmt.Errorf("create_contact must be true")
	}
	if v, ok := config["create_opportunity"].(bool); ok {
		out.CreateOpportunity = v
	}
	if v, ok := config["create_appointment"].(bool); ok {
		out.CreateAppointment = v
	}
	if s, ok := config["calendar_id"].(string); ok {
		out.CalendarID = strings.TrimSpace(s)
	}
	if s, ok := config["appointment_timezone"].(string); ok {
		out.AppointmentTimezone = strings.TrimSpace(s)
	}
	out.AppointmentDatetime = parseGHLFieldSource(config["appointment_datetime"])
	out.AppointmentTitleTemplate = parseGHLTitleTemplate(config, "appointment_title_template", "appointment_title", "{{first_name}}")
	out.OpportunityTitleTemplate = parseGHLTitleTemplate(config, "opportunity_title_template", "", "{{first_name}} {{last_name}}")
	if notes := parseGHLFieldSourcePtr(config["appointment_notes"]); notes != nil {
		out.AppointmentNotes = notes
	}
	out.PipelineStageMap = parsePipelineStageMap(config["pipeline_stage_map"], out.DeliveryMode)
	out.OutboundFieldMap = outboundFieldMapFromConfig(config)

	if out.DeliveryMode == "webhook" {
		if out.WebhookURL == "" {
			return out, fmt.Errorf("webhook_url is required in webhook delivery mode")
		}
		if !ghlWebhookURLValid(out.WebhookURL) {
			return out, fmt.Errorf("webhook_url must be a valid http or https URL")
		}
		return out, nil
	}

	if out.LocationID == "" {
		return out, fmt.Errorf("location_id is required in config")
	}

	if out.CreateAppointment && !ghlFieldSourceSet(out.AppointmentDatetime) {
		out.AppointmentDatetime = defaultGHLAppointmentDatetime()
	}

	if out.CreateOpportunity && len(out.PipelineStageMap) == 0 {
		return out, fmt.Errorf("pipeline_stage_map required when create_opportunity is enabled")
	}
	if out.CreateAppointment {
		if out.CalendarID == "" {
			return out, fmt.Errorf("calendar_id required when create_appointment is enabled")
		}
		if out.AppointmentTimezone == "" {
			return out, fmt.Errorf("appointment_timezone required when create_appointment is enabled")
		}
		if !ghlFieldSourceSet(out.AppointmentDatetime) {
			return out, fmt.Errorf("appointment_datetime required when create_appointment is enabled")
		}
		if strings.TrimSpace(out.AppointmentTitleTemplate) == "" {
			return out, fmt.Errorf("appointment_title_template required when create_appointment is enabled")
		}
	}
	return out, nil
}

// ParseGHLConfigForTest validates only what credential testing needs (location or webhook URL).
// It does not require pipeline_stage_map, calendar_id, or appointment fields.
func ParseGHLConfigForTest(config map[string]any) (GHLConfig, error) {
	out := GHLConfig{CreateContact: true}
	if config == nil {
		config = map[string]any{}
	}
	out.DeliveryMode = parseGHLDeliveryMode(config)
	if s, ok := config["webhook_url"].(string); ok {
		out.WebhookURL = strings.TrimSpace(s)
	}
	if loc, ok := config["location_id"].(string); ok {
		out.LocationID = strings.TrimSpace(loc)
	}

	if out.DeliveryMode == "webhook" {
		if out.WebhookURL == "" {
			return out, fmt.Errorf("webhook_url is required in webhook delivery mode")
		}
		if !ghlWebhookURLValid(out.WebhookURL) {
			return out, fmt.Errorf("webhook_url must be a valid http or https URL")
		}
		return out, nil
	}

	if out.LocationID == "" {
		return out, fmt.Errorf("location_id is required in config")
	}
	return out, nil
}

func parseGHLTitleTemplate(config map[string]any, templateKey, legacyKey, defaultTemplate string) string {
	if raw, ok := config[templateKey]; ok {
		if s, ok := raw.(string); ok {
			return strings.TrimSpace(s)
		}
		return ""
	}
	if legacyKey != "" {
		if legacy := ghlFieldSourceToTemplate(parseGHLFieldSource(config[legacyKey])); legacy != "" {
			return legacy
		}
	}
	return defaultTemplate
}

func ghlFieldSourceToTemplate(fs GHLFieldSource) string {
	switch fs.SourceType {
	case "static":
		if fs.StaticValue != nil {
			return *fs.StaticValue
		}
	case "builtin":
		if fs.BuiltinField != nil && strings.TrimSpace(*fs.BuiltinField) != "" {
			return "{{" + strings.TrimSpace(*fs.BuiltinField) + "}}"
		}
	case "custom":
		if fs.CustomFieldID != nil && *fs.CustomFieldID > 0 {
			return fmt.Sprintf("{{custom:%d}}", *fs.CustomFieldID)
		}
	}
	return ""
}

func parseGHLFieldSourcePtr(raw any) *GHLFieldSource {
	fs := parseGHLFieldSource(raw)
	if !ghlFieldSourceSet(fs) {
		return nil
	}
	return &fs
}

func parseGHLFieldSource(raw any) GHLFieldSource {
	if raw == nil {
		return GHLFieldSource{}
	}
	b, err := json.Marshal(raw)
	if err != nil {
		return GHLFieldSource{}
	}
	var fs GHLFieldSource
	_ = json.Unmarshal(b, &fs)
	return fs
}

func defaultGHLAppointmentDatetime() GHLFieldSource {
	bf := "action_at"
	return GHLFieldSource{SourceType: "builtin", BuiltinField: &bf}
}

func ghlFieldSourceSet(fs GHLFieldSource) bool {
	switch fs.SourceType {
	case "static":
		return fs.StaticValue != nil && strings.TrimSpace(*fs.StaticValue) != ""
	case "builtin":
		return fs.BuiltinField != nil && strings.TrimSpace(*fs.BuiltinField) != ""
	case "custom":
		return fs.CustomFieldID != nil && *fs.CustomFieldID > 0
	default:
		return false
	}
}

func parsePipelineStageMap(raw any, deliveryMode string) []GHLPipelineStageMapEntry {
	if raw == nil {
		return nil
	}
	b, err := json.Marshal(raw)
	if err != nil {
		return nil
	}
	var entries []GHLPipelineStageMapEntry
	if json.Unmarshal(b, &entries) != nil {
		return nil
	}
	webhookMode := deliveryMode == "webhook"
	seen := map[string]bool{}
	out := make([]GHLPipelineStageMapEntry, 0, len(entries))
	for _, e := range entries {
		if e.LeadrulaPipelineID == 0 || e.LeadrulaStageID == 0 {
			continue
		}
		if !webhookMode {
			if strings.TrimSpace(e.GHLPipelineID) == "" || strings.TrimSpace(e.GHLPipelineStageID) == "" {
				continue
			}
		}
		key := fmt.Sprintf("%d:%d", e.LeadrulaPipelineID, e.LeadrulaStageID)
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, e)
	}
	return out
}

// MatchesGHLWebhookTrigger reports whether the lead stage is configured as a webhook outbound trigger.
func MatchesGHLWebhookTrigger(entries []GHLPipelineStageMapEntry, pipelineID, stageID int64) bool {
	if pipelineID == 0 || stageID == 0 || len(entries) == 0 {
		return false
	}
	for _, e := range entries {
		if e.LeadrulaPipelineID == pipelineID && e.LeadrulaStageID == stageID {
			return true
		}
	}
	return false
}

func resolveGHLStage(mapEntries []GHLPipelineStageMapEntry, pipelineID, stageID int64) (string, string, error) {
	if pipelineID == 0 || stageID == 0 {
		return "", "", fmt.Errorf("lead pipeline_id and stage_id required for opportunity push")
	}
	for _, e := range mapEntries {
		if e.LeadrulaPipelineID == pipelineID && e.LeadrulaStageID == stageID {
			return e.GHLPipelineID, e.GHLPipelineStageID, nil
		}
	}
	return "", "", fmt.Errorf("no GHL stage mapped for Leadrula pipeline %d stage %d", pipelineID, stageID)
}

func buildGHLContactBody(cfg GHLConfig, payload DeliveryPayload) map[string]any {
	contact := ghlStandardContactFields(cfg, payload)
	if fields := ghlCustomFieldsPayload(cfg.OutboundFieldMap, payload, "contact"); len(fields) > 0 {
		contact["customFields"] = fields
	}
	return contact
}

func ghlStandardContactFields(cfg GHLConfig, payload DeliveryPayload) map[string]any {
	return map[string]any{
		"firstName":  payload.FirstName,
		"lastName":   payload.LastName,
		"phone":      payload.Phone,
		"email":      payload.Email,
		"address1":   payload.Address,
		"city":       payload.City,
		"state":      payload.State,
		"postalCode": payload.Zip,
		"source":     payload.Source,
		"locationId": cfg.LocationID,
		"tags":       []string{"leadrula"},
	}
}

func ghlFieldModel(e SunbaseFieldMapEntry) string {
	if e.GHLFieldModel != nil {
		if m := strings.TrimSpace(*e.GHLFieldModel); m != "" {
			return m
		}
	}
	return "contact"
}

func ghlCustomFieldsPayload(entries []SunbaseFieldMapEntry, payload DeliveryPayload, model string) []map[string]any {
	var out []map[string]any
	for _, e := range entries {
		if e.DestKey == "" || ghlFieldModel(e) != model {
			continue
		}
		v := resolveGHLFieldValue(e, payload)
		if v == "" {
			continue
		}
		item := map[string]any{
			"key":          e.DestKey,
			"field_value":  v,
		}
		if e.GHLCustomFieldID != nil && strings.TrimSpace(*e.GHLCustomFieldID) != "" {
			item["id"] = strings.TrimSpace(*e.GHLCustomFieldID)
		}
		out = append(out, item)
	}
	return out
}

func resolveGHLFieldValue(e SunbaseFieldMapEntry, payload DeliveryPayload) string {
	return resolveGHLFieldSourceValue(GHLFieldSource{
		SourceType:    e.SourceType,
		BuiltinField:  e.BuiltinField,
		CustomFieldID: e.CustomFieldID,
		StaticValue:   e.StaticValue,
	}, payload)
}

func resolveGHLFieldSourceValue(fs GHLFieldSource, payload DeliveryPayload) string {
	switch fs.SourceType {
	case "static":
		if fs.StaticValue != nil {
			return *fs.StaticValue
		}
	case "builtin":
		if fs.BuiltinField == nil {
			return ""
		}
		switch *fs.BuiltinField {
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
			return payload.ActionAt
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
		if fs.CustomFieldID != nil && payload.CustomFields != nil {
			key := strconv.FormatInt(*fs.CustomFieldID, 10)
			if v, ok := payload.CustomFields[key]; ok {
				s := valueToQueryString(v)
				if ftype := ghlCustomFieldType(payload.Config, key); ftype == "date" || ftype == "datetime" {
					s = customfields.FormatForSunbaseExport(ftype, s)
				}
				return s
			}
		}
	}
	return ""
}

func ghlCustomFieldType(config map[string]any, fieldID string) string {
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

func resolveGHLTitleTemplate(template string, payload DeliveryPayload) string {
	template = strings.TrimSpace(template)
	if template == "" {
		return ""
	}
	result := ghlTitlePlaceholderRe.ReplaceAllStringFunc(template, func(match string) string {
		token := strings.TrimSpace(match[2 : len(match)-2])
		if strings.HasPrefix(token, "custom:") {
			idStr := strings.TrimSpace(strings.TrimPrefix(token, "custom:"))
			id, err := strconv.ParseInt(idStr, 10, 64)
			if err != nil || id <= 0 {
				return ""
			}
			return resolveGHLFieldSourceValue(GHLFieldSource{
				SourceType:    "custom",
				CustomFieldID: &id,
			}, payload)
		}
		bf := token
		return resolveGHLFieldSourceValue(GHLFieldSource{
			SourceType:   "builtin",
			BuiltinField: &bf,
		}, payload)
	})
	return strings.TrimSpace(result)
}

func defaultAppointmentTitle(payload DeliveryPayload) string {
	name := strings.TrimSpace(payload.FirstName + " " + payload.LastName)
	if name != "" {
		return name
	}
	return "Appointment"
}

func defaultOpportunityTitle(payload DeliveryPayload) string {
	name := strings.TrimSpace(payload.FirstName + " " + payload.LastName)
	if name != "" {
		return name
	}
	return "Opportunity"
}

func parseAppointmentTimes(datetimeStr, timezone string) (startISO, endISO string, err error) {
	datetimeStr = strings.TrimSpace(datetimeStr)
	timezone = strings.TrimSpace(timezone)
	if datetimeStr == "" {
		return "", "", fmt.Errorf("appointment datetime is empty")
	}
	if timezone == "" {
		timezone = "America/New_York"
	}
	loc, err := time.LoadLocation(timezone)
	if err != nil {
		return "", "", fmt.Errorf("invalid appointment_timezone: %w", err)
	}
	var t time.Time
	layouts := []string{
		time.RFC3339,
		"2006-01-02T15:04:05",
		"2006-01-02 15:04:05",
		"2006-01-02",
	}
	for _, layout := range layouts {
		if parsed, e := time.ParseInLocation(layout, datetimeStr, loc); e == nil {
			t = parsed
			break
		}
	}
	if t.IsZero() {
		if parsed, e := time.Parse(time.RFC3339, datetimeStr); e == nil {
			t = parsed.In(loc)
		}
	}
	if t.IsZero() {
		return "", "", fmt.Errorf("could not parse appointment datetime %q", datetimeStr)
	}
	start := t.In(loc)
	end := start.Add(30 * time.Minute)
	return start.Format(time.RFC3339), end.Format(time.RFC3339), nil
}

func DefaultGHLConnectionConfig(locationID string) map[string]any {
	return map[string]any{
		"delivery_mode":        "api",
		"location_id":          locationID,
		"create_contact":      true,
		"create_opportunity":  false,
		"create_appointment":  false,
		"appointment_timezone":       "America/New_York",
		"appointment_datetime": map[string]any{
			"source_type":   "builtin",
			"builtin_field": "action_at",
		},
		"appointment_title_template": "{{first_name}}",
		"opportunity_title_template": "{{first_name}} {{last_name}}",
		"pipeline_stage_map":  []any{},
		"outbound_field_map":  []any{},
	}
}
