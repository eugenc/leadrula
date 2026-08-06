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
	CRMPipelineID      string `json:"crm_pipeline_id,omitempty"`
	CRMStageID         string `json:"crm_stage_id,omitempty"`
	CRMStageName       string `json:"crm_stage_name,omitempty"`
	GHLPipelineID      string `json:"ghl_pipeline_id,omitempty"`
	GHLPipelineStageID string `json:"ghl_pipeline_stage_id,omitempty"`
	GHLStageName       string `json:"ghl_stage_name,omitempty"`
}

type GHLOpportunityStandardFields struct {
	MonetaryValue  GHLFieldSource
	AssignedUserID GHLFieldSource
	Status         GHLFieldSource
}

type GHLAppointmentStandardFields struct {
	Description         GHLFieldSource
	Address             GHLFieldSource
	DurationMinutes     int
	AssignedUserID      GHLFieldSource
	MeetingLocationType GHLFieldSource
}

type GHLContactStandardFields struct {
	FirstName  GHLFieldSource
	LastName   GHLFieldSource
	Phone      GHLFieldSource
	Email      GHLFieldSource
	Address1   GHLFieldSource
	City       GHLFieldSource
	State      GHLFieldSource
	PostalCode GHLFieldSource
	Source     GHLFieldSource
}

type ghlContactStandardFieldSpec struct {
	ghlKey         string
	configKey      string
	required       bool
	defaultBuiltin string
}

var ghlContactStandardFieldSpecs = []ghlContactStandardFieldSpec{
	{ghlKey: "firstName", configKey: "firstName", required: true, defaultBuiltin: "first_name"},
	{ghlKey: "lastName", configKey: "lastName", required: true, defaultBuiltin: "last_name"},
	{ghlKey: "phone", configKey: "phone", required: true, defaultBuiltin: "phone"},
	{ghlKey: "email", configKey: "email", defaultBuiltin: "email"},
	{ghlKey: "address1", configKey: "address1", defaultBuiltin: "address"},
	{ghlKey: "city", configKey: "city", defaultBuiltin: "city"},
	{ghlKey: "state", configKey: "state", defaultBuiltin: "state"},
	{ghlKey: "postalCode", configKey: "postalCode", defaultBuiltin: "zip"},
	{ghlKey: "source", configKey: "source", defaultBuiltin: "source"},
}

type GHLConfig struct {
	DeliveryMode                string
	WebhookURL                  string
	LocationID                  string
	CreateContact               bool
	CreateOpportunity           bool
	CreateAppointment           bool
	CalendarID                  string
	AppointmentTimezone         string
	AppointmentDatetime         GHLFieldSource
	AppointmentTitleTemplate    string
	OpportunityTitleTemplate    string
	AppointmentNotes            *GHLFieldSource
	PipelineStageMap            []GHLPipelineStageMapEntry
	OutboundFieldMap            []SunbaseFieldMapEntry
	OpportunityStandardFields   GHLOpportunityStandardFields
	AppointmentStandardFields   GHLAppointmentStandardFields
	ContactStandardFields       GHLContactStandardFields
	ContactStandardFieldsConfigured bool
	InboundStageSyncEnabled       bool
	InboundSyncLeadrulaPipelineID int64
	InboundSyncGHLPipelineID      string
	SyncContactUpdatesEnabled     bool
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
	out.OutboundFieldMap = ghlOutboundFieldMapFromConfig(config)
	out.OpportunityStandardFields = parseGHLOpportunityStandardFields(config["opportunity_standard_fields"])
	out.AppointmentStandardFields = parseGHLAppointmentStandardFields(config["appointment_standard_fields"])
	if _, ok := config["contact_standard_fields"]; ok {
		out.ContactStandardFieldsConfigured = true
	}
	out.ContactStandardFields = parseGHLContactStandardFields(config["contact_standard_fields"])
	if v, ok := config["inbound_stage_sync_enabled"].(bool); ok {
		out.InboundStageSyncEnabled = v
	}
	out.InboundSyncLeadrulaPipelineID = ghlInt64FromAny(config["inbound_sync_leadrula_pipeline_id"])
	if s, ok := config["inbound_sync_ghl_pipeline_id"].(string); ok {
		out.InboundSyncGHLPipelineID = strings.TrimSpace(s)
	}
	if v, ok := config["sync_contact_updates_enabled"].(bool); ok {
		out.SyncContactUpdatesEnabled = v
	}
	if err := validateInboundStageSync(out); err != nil {
		return out, err
	}
	if err := validateGHLContactStandardFields(out); err != nil {
		return out, err
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

// PipelineStageMapFromConfig reads pipeline_stage_map from a GHL connection config map.
func PipelineStageMapFromConfig(config map[string]any) []GHLPipelineStageMapEntry {
	if config == nil {
		return nil
	}
	return parsePipelineStageMap(config["pipeline_stage_map"], ParseGHLDeliveryModeFromConfig(config))
}

// MergePipelineStageMapEntries appends entries without clobbering existing leadrula pipeline/stage pairs.
func MergePipelineStageMapEntries(existing, add []GHLPipelineStageMapEntry) []GHLPipelineStageMapEntry {
	seen := map[string]bool{}
	out := make([]GHLPipelineStageMapEntry, 0, len(existing)+len(add))
	for _, e := range existing {
		key := fmt.Sprintf("%d:%d", e.LeadrulaPipelineID, e.LeadrulaStageID)
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, e)
	}
	for _, e := range add {
		if e.LeadrulaPipelineID == 0 || e.LeadrulaStageID == 0 {
			continue
		}
		if entryCRMPipelineID(mapEntry(e)) == "" || entryCRMStageID(mapEntry(e)) == "" {
			continue
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
			if entryCRMPipelineID(mapEntry(e)) == "" || entryCRMStageID(mapEntry(e)) == "" {
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

func ghlOutboundFieldMapFromConfig(config map[string]any) []SunbaseFieldMapEntry {
	if config == nil {
		return nil
	}
	raw, ok := config["outbound_field_map"]
	if !ok || raw == nil {
		return nil
	}
	b, err := json.Marshal(raw)
	if err != nil {
		return nil
	}
	var entries []SunbaseFieldMapEntry
	if json.Unmarshal(b, &entries) != nil {
		return nil
	}
	return entries
}

func parseGHLOpportunityStandardFields(raw any) GHLOpportunityStandardFields {
	m := parseGHLStandardFieldsMap(raw)
	out := GHLOpportunityStandardFields{}
	if fs, ok := m["monetary_value"]; ok {
		out.MonetaryValue = fs
	}
	if fs, ok := m["assigned_user_id"]; ok {
		out.AssignedUserID = fs
	}
	if fs, ok := m["status"]; ok {
		out.Status = fs
	}
	return out
}

func parseGHLAppointmentStandardFields(raw any) GHLAppointmentStandardFields {
	out := GHLAppointmentStandardFields{DurationMinutes: 30}
	if raw == nil {
		return out
	}
	m, ok := raw.(map[string]any)
	if !ok {
		b, err := json.Marshal(raw)
		if err != nil {
			return out
		}
		if json.Unmarshal(b, &m) != nil {
			return out
		}
	}
	if v := intFromAny(m["duration_minutes"]); v > 0 {
		out.DurationMinutes = v
	}
	fields := parseGHLStandardFieldsMap(raw)
	if fs, ok := fields["description"]; ok {
		out.Description = fs
	}
	if fs, ok := fields["address"]; ok {
		out.Address = fs
	}
	if fs, ok := fields["assigned_user_id"]; ok {
		out.AssignedUserID = fs
	}
	if fs, ok := fields["meeting_location_type"]; ok {
		out.MeetingLocationType = fs
	}
	return out
}

func parseGHLContactStandardFields(raw any) GHLContactStandardFields {
	fields := parseGHLStandardFieldsMap(raw)
	return ghlContactStandardFieldsFromMap(fields)
}

func ghlContactStandardFieldsFromMap(fields map[string]GHLFieldSource) GHLContactStandardFields {
	out := GHLContactStandardFields{}
	for _, spec := range ghlContactStandardFieldSpecs {
		if fs, ok := fields[spec.configKey]; ok {
			setGHLContactStandardField(&out, spec.configKey, fs)
		}
	}
	return out
}

func setGHLContactStandardField(out *GHLContactStandardFields, key string, fs GHLFieldSource) {
	switch key {
	case "firstName":
		out.FirstName = fs
	case "lastName":
		out.LastName = fs
	case "phone":
		out.Phone = fs
	case "email":
		out.Email = fs
	case "address1":
		out.Address1 = fs
	case "city":
		out.City = fs
	case "state":
		out.State = fs
	case "postalCode":
		out.PostalCode = fs
	case "source":
		out.Source = fs
	}
}

func (f GHLContactStandardFields) field(key string) GHLFieldSource {
	switch key {
	case "firstName":
		return f.FirstName
	case "lastName":
		return f.LastName
	case "phone":
		return f.Phone
	case "email":
		return f.Email
	case "address1":
		return f.Address1
	case "city":
		return f.City
	case "state":
		return f.State
	case "postalCode":
		return f.PostalCode
	case "source":
		return f.Source
	default:
		return GHLFieldSource{}
	}
}

func defaultGHLContactStandardFields() GHLContactStandardFields {
	out := GHLContactStandardFields{}
	for _, spec := range ghlContactStandardFieldSpecs {
		bf := spec.defaultBuiltin
		setGHLContactStandardField(&out, spec.configKey, GHLFieldSource{
			SourceType:   "builtin",
			BuiltinField: &bf,
		})
	}
	return out
}

func defaultGHLContactStandardFieldsMap() map[string]any {
	out := map[string]any{}
	for _, spec := range ghlContactStandardFieldSpecs {
		out[spec.configKey] = map[string]any{
			"source_type":   "builtin",
			"builtin_field": spec.defaultBuiltin,
		}
	}
	return out
}

func resolveGHLContactStandardField(cfg GHLConfig, spec ghlContactStandardFieldSpec) (GHLFieldSource, bool) {
	defaults := defaultGHLContactStandardFields()
	if !cfg.ContactStandardFieldsConfigured {
		return defaults.field(spec.configKey), true
	}
	fs := cfg.ContactStandardFields.field(spec.configKey)
	if ghlFieldSourceSet(fs) {
		return fs, true
	}
	if spec.required {
		return defaults.field(spec.configKey), true
	}
	return GHLFieldSource{}, false
}

func validateGHLContactStandardFields(cfg GHLConfig) error {
	if !cfg.ContactStandardFieldsConfigured {
		return nil
	}
	for _, spec := range ghlContactStandardFieldSpecs {
		if !spec.required {
			continue
		}
		if !ghlFieldSourceSet(cfg.ContactStandardFields.field(spec.configKey)) {
			return fmt.Errorf("contact_standard_fields.%s is required", spec.configKey)
		}
	}
	return nil
}

func ghlContactFieldSourceTrackedKeys(fs GHLFieldSource, builtins map[string]bool, customIDs map[string]bool) {
	switch fs.SourceType {
	case "builtin":
		if fs.BuiltinField != nil && strings.TrimSpace(*fs.BuiltinField) != "" {
			builtins[strings.TrimSpace(*fs.BuiltinField)] = true
		}
	case "custom":
		if fs.CustomFieldID != nil && *fs.CustomFieldID > 0 {
			customIDs[strconv.FormatInt(*fs.CustomFieldID, 10)] = true
		}
	}
}

// GHLContactTrackedPayloadKeys returns lead payload keys that affect GHL contact delivery for a connection.
func GHLContactTrackedPayloadKeys(cfg GHLConfig) (builtins map[string]bool, customIDs map[string]bool) {
	builtins = map[string]bool{}
	customIDs = map[string]bool{}
	for _, spec := range ghlContactStandardFieldSpecs {
		fs, include := resolveGHLContactStandardField(cfg, spec)
		if !include {
			continue
		}
		ghlContactFieldSourceTrackedKeys(fs, builtins, customIDs)
	}
	for _, e := range cfg.OutboundFieldMap {
		if ghlFieldModel(e) != "contact" {
			continue
		}
		ghlContactFieldSourceTrackedKeys(GHLFieldSource{
			SourceType:    e.SourceType,
			BuiltinField:  e.BuiltinField,
			CustomFieldID: e.CustomFieldID,
			StaticValue:   e.StaticValue,
		}, builtins, customIDs)
	}
	return builtins, customIDs
}

// GHLContactPayloadSlice extracts contact-relevant delivery payload keys for change detection.
func GHLContactPayloadSlice(cfg GHLConfig, payload []byte) map[string]any {
	out := map[string]any{}
	if len(payload) == 0 {
		return out
	}
	var m map[string]any
	if json.Unmarshal(payload, &m) != nil {
		return out
	}
	builtins, customIDs := GHLContactTrackedPayloadKeys(cfg)
	for k := range builtins {
		if v, ok := m[k]; ok {
			out[k] = v
		}
	}
	if len(customIDs) == 0 {
		return out
	}
	cf, ok := m["custom_fields"].(map[string]any)
	if !ok {
		return out
	}
	filtered := map[string]any{}
	for id, v := range cf {
		if customIDs[id] {
			filtered[id] = v
		}
	}
	if len(filtered) > 0 {
		out["custom_fields"] = filtered
	}
	return out
}

// GHLContactPayloadChanged reports whether contact-relevant delivery fields differ for a connection.
func GHLContactPayloadChanged(cfg GHLConfig, before, after []byte) bool {
	a := GHLContactPayloadSlice(cfg, before)
	b := GHLContactPayloadSlice(cfg, after)
	ja, _ := json.Marshal(a)
	jb, _ := json.Marshal(b)
	return string(ja) != string(jb)
}

func intFromAny(v any) int {
	switch x := v.(type) {
	case float64:
		return int(x)
	case int:
		return x
	case int64:
		return int(x)
	case json.Number:
		n, _ := x.Int64()
		return int(n)
	default:
		return 0
	}
}

func applyGHLOpportunityStandardFields(opp map[string]any, std GHLOpportunityStandardFields, payload DeliveryPayload) {
	if v := strings.TrimSpace(resolveGHLFieldSourceValue(std.MonetaryValue, payload)); v != "" {
		if n, err := strconv.ParseFloat(v, 64); err == nil {
			opp["monetaryValue"] = n
		}
	}
	if v := strings.TrimSpace(resolveGHLFieldSourceValue(std.AssignedUserID, payload)); v != "" {
		opp["assignedTo"] = v
	}
	if v := strings.TrimSpace(resolveGHLFieldSourceValue(std.Status, payload)); v != "" {
		opp["status"] = v
	}
}

func applyGHLAppointmentStandardFields(event map[string]any, std GHLAppointmentStandardFields, payload DeliveryPayload) {
	if v := strings.TrimSpace(resolveGHLFieldSourceValue(std.Description, payload)); v != "" {
		event["description"] = v
	}
	if v := strings.TrimSpace(resolveGHLFieldSourceValue(std.Address, payload)); v != "" {
		event["address"] = v
	}
	if v := strings.TrimSpace(resolveGHLFieldSourceValue(std.MeetingLocationType, payload)); v != "" {
		event["meetingLocationType"] = v
	}
}

func appointmentDurationMinutes(std GHLAppointmentStandardFields) time.Duration {
	mins := std.DurationMinutes
	if mins <= 0 {
		mins = 30
	}
	return time.Duration(mins) * time.Minute
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

// ResolveGHLStage maps a Leadrula pipeline/stage to GHL pipeline and stage IDs.
func ResolveGHLStage(mapEntries []GHLPipelineStageMapEntry, pipelineID, stageID int64) (string, string, error) {
	return resolveGHLStage(mapEntries, pipelineID, stageID)
}

func validateInboundStageSync(cfg GHLConfig) error {
	if !cfg.InboundStageSyncEnabled {
		return nil
	}
	if cfg.InboundSyncLeadrulaPipelineID <= 0 {
		return fmt.Errorf("inbound_sync_leadrula_pipeline_id required when inbound stage sync is enabled")
	}
	if strings.TrimSpace(cfg.InboundSyncGHLPipelineID) == "" {
		return fmt.Errorf("inbound_sync_ghl_pipeline_id required when inbound stage sync is enabled")
	}
	if !hasCompleteInboundStageMap(cfg.PipelineStageMap, cfg.InboundSyncLeadrulaPipelineID) {
		return fmt.Errorf("configure stage mappings for the selected pipeline")
	}
	return nil
}

func hasCompleteInboundStageMap(entries []GHLPipelineStageMapEntry, lrPipelineID int64) bool {
	return hasCompleteCRMInboundStageMap(mapEntries(entries), lrPipelineID)
}

// InboundStageSyncReady reports whether inbound GHL stage auto-sync should run.
func InboundStageSyncReady(cfg GHLConfig) bool {
	return CRMInboundStageSyncReady(ParseInboundStageSyncFromGHL(cfg))
}

func ParseInboundStageSyncFromGHL(cfg GHLConfig) InboundStageSyncConfig {
	entries := make([]CRMPipelineStageMapEntry, 0, len(cfg.PipelineStageMap))
	for _, e := range cfg.PipelineStageMap {
		entries = append(entries, mapEntry(e))
	}
	crmPipelineID := cfg.InboundSyncGHLPipelineID
	return InboundStageSyncConfig{
		Enabled:            cfg.InboundStageSyncEnabled,
		LeadrulaPipelineID: cfg.InboundSyncLeadrulaPipelineID,
		CRMPipelineID:      crmPipelineID,
		PipelineStageMap:   entries,
	}
}

func ResolveLeadrulaStage(entries []GHLPipelineStageMapEntry, lrPipelineID int64, ghlPipelineID, ghlStageID string) (stageID int64, ok bool) {
	return ResolveCRMLeadrulaStage(mapEntries(entries), lrPipelineID, ghlPipelineID, ghlStageID)
}

func mapEntry(e GHLPipelineStageMapEntry) CRMPipelineStageMapEntry {
	return CRMPipelineStageMapEntry{
		LeadrulaPipelineID: e.LeadrulaPipelineID,
		LeadrulaStageID:    e.LeadrulaStageID,
		CRMPipelineID:      e.CRMPipelineID,
		CRMStageID:         e.CRMStageID,
		CRMStageName:       e.CRMStageName,
		GHLPipelineID:      e.GHLPipelineID,
		GHLPipelineStageID: e.GHLPipelineStageID,
		GHLStageName:       e.GHLStageName,
	}
}

func mapEntries(entries []GHLPipelineStageMapEntry) []CRMPipelineStageMapEntry {
	out := make([]CRMPipelineStageMapEntry, 0, len(entries))
	for _, e := range entries {
		out = append(out, mapEntry(e))
	}
	return out
}

var ghlInboundPipelineIDKeys = []string{
	"pipelineId", "pipeline_id",
	"opportunity.pipelineId", "opportunity.pipeline_id",
}
var ghlInboundStageIDKeys = []string{
	"pipelineStageId", "pipeline_stage_id", "stageId",
	"opportunity.pipelineStageId", "opportunity.pipeline_stage_id",
}
var ghlInboundStageNameKeys = []string{
	"pipeline_stage", "pipleline_stage", "pippleine_stage", "stage_name", "stageName",
	"opportunity.pipeline_stage_name", "opportunity.stageName",
}

func ghlFlatText(flat map[string]any, keys ...string) string {
	for _, key := range keys {
		if v, ok := flat[key]; ok {
			if s := strings.TrimSpace(toGHLText(v)); s != "" && s != "null" {
				return s
			}
		}
	}
	return ""
}

func copyGHLFlatKeyIfEmpty(out map[string]any, dest string, sources ...string) {
	if ghlFlatText(out, dest) != "" {
		return
	}
	for _, src := range sources {
		if s := ghlFlatText(out, src); s != "" {
			out[dest] = out[src]
			return
		}
	}
}

// NormalizeGHLInboundFlat promotes nested GHL default webhook keys to the flat keys stage sync expects.
func NormalizeGHLInboundFlat(flat map[string]any) map[string]any {
	if flat == nil {
		return map[string]any{}
	}
	out := make(map[string]any, len(flat)+8)
	for k, v := range flat {
		out[k] = v
	}
	copyGHLFlatKeyIfEmpty(out, "contactId", "contact_id", "contact.id", "contact.contactId", "contact.contact_id")
	copyGHLFlatKeyIfEmpty(out, "contact_id", "contactId", "contact.contact_id", "contact.id", "contact.contactId")
	copyGHLFlatKeyIfEmpty(out, "pipelineId", "pipeline_id", "opportunity.pipelineId", "opportunity.pipeline_id")
	copyGHLFlatKeyIfEmpty(out, "pipeline_id", "pipelineId", "opportunity.pipeline_id", "opportunity.pipelineId")
	copyGHLFlatKeyIfEmpty(out, "pipelineStageId", "pipeline_stage_id", "stageId", "opportunity.pipelineStageId", "opportunity.pipeline_stage_id")
	copyGHLFlatKeyIfEmpty(out, "pipleline_stage", "pipeline_stage", "pippleine_stage", "stage_name", "stageName", "opportunity.pipeline_stage_name", "opportunity.stageName")
	return out
}

// PrepareGHLInboundFlat normalizes GHL inbound payloads for field mapping and stage sync.
func PrepareGHLInboundFlat(flat map[string]any) map[string]any {
	out := NormalizeGHLInboundFlat(flat)

	if customData, ok := out["customData"].(map[string]any); ok {
		for k, v := range customData {
			if ghlFlatText(out, k) == "" {
				out[k] = v
			}
			oppKey := "opportunity." + k
			if ghlFlatText(out, oppKey) == "" {
				out[oppKey] = v
			}
		}
	}

	expandGHLCustomFieldsArray(out, out["customFields"])
	expandGHLCustomFieldsArray(out, out["opportunity.customFields"])

	return out
}

func expandGHLCustomFieldsArray(out map[string]any, raw any) {
	items, ok := raw.([]any)
	if !ok {
		return
	}
	for _, item := range items {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		key := strings.TrimSpace(toGHLText(m["key"]))
		if key == "" {
			continue
		}
		val := m["field_value"]
		if val == nil {
			val = m["value"]
		}
		if ghlFlatText(out, key) == "" {
			out[key] = val
		}
	}
}

// GHLInboundPipelineStageName reads a human-readable stage name from GHL default or custom webhook fields.
func GHLInboundPipelineStageName(flat map[string]any) string {
	return ghlFlatText(flat, ghlInboundStageNameKeys...)
}

// GHLInboundPipelineStage reads GHL pipeline and stage IDs from a flattened webhook payload.
func GHLInboundPipelineStage(flat map[string]any) (pipelineID, stageID string) {
	return ghlFlatText(flat, ghlInboundPipelineIDKeys...), ghlFlatText(flat, ghlInboundStageIDKeys...)
}

func toGHLText(v any) string {
	switch x := v.(type) {
	case string:
		return x
	case float64:
		if x == float64(int64(x)) {
			return strconv.FormatInt(int64(x), 10)
		}
		return strconv.FormatFloat(x, 'f', -1, 64)
	default:
		return fmt.Sprint(v)
	}
}

func ghlInt64FromAny(v any) int64 {
	switch x := v.(type) {
	case float64:
		return int64(x)
	case int64:
		return x
	case int:
		return int64(x)
	case json.Number:
		n, _ := x.Int64()
		return n
	case string:
		n, _ := strconv.ParseInt(strings.TrimSpace(x), 10, 64)
		return n
	default:
		return 0
	}
}

func buildGHLContactBody(cfg GHLConfig, payload DeliveryPayload) map[string]any {
	contact := ghlStandardContactFields(cfg, payload)
	if fields := ghlCustomFieldsPayload(cfg.OutboundFieldMap, payload, "contact"); len(fields) > 0 {
		contact["customFields"] = fields
	}
	return contact
}

func ghlStandardContactFields(cfg GHLConfig, payload DeliveryPayload) map[string]any {
	contact := map[string]any{
		"locationId": cfg.LocationID,
		"tags":       []string{"leadrula"},
	}
	for _, spec := range ghlContactStandardFieldSpecs {
		fs, include := resolveGHLContactStandardField(cfg, spec)
		if !include {
			continue
		}
		v := resolveGHLFieldSourceValue(fs, payload)
		if fs.SourceType == "builtin" && fs.BuiltinField != nil && *fs.BuiltinField == "action_at" && v != "" {
			v = customfields.FormatForSunbaseExportInTimezone("datetime", v, sunbaseAccountTimezone(payload.Config))
		}
		setGHLContactField(contact, spec.ghlKey, v)
	}
	return contact
}

func setGHLContactField(contact map[string]any, key, value string) {
	if v := strings.TrimSpace(value); v != "" {
		contact[key] = v
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
	v := resolveGHLFieldSourceValue(GHLFieldSource{
		SourceType:    e.SourceType,
		BuiltinField:  e.BuiltinField,
		CustomFieldID: e.CustomFieldID,
		StaticValue:   e.StaticValue,
	}, payload)
	if v == "" {
		return ""
	}
	if e.SourceType == "builtin" && e.BuiltinField != nil && *e.BuiltinField == "action_at" {
		return customfields.FormatForSunbaseExportInTimezone("datetime", v, sunbaseAccountTimezone(payload.Config))
	}
	return v
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

func parseAppointmentTimes(datetimeStr, timezone string, duration time.Duration) (startISO, endISO string, err error) {
	datetimeStr = strings.TrimSpace(datetimeStr)
	timezone = strings.TrimSpace(timezone)
	if datetimeStr == "" {
		return "", "", fmt.Errorf("appointment datetime is empty")
	}
	if timezone == "" {
		timezone = "America/New_York"
	}
	if duration <= 0 {
		duration = 30 * time.Minute
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
	end := start.Add(duration)
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
		"contact_standard_fields":    defaultGHLContactStandardFieldsMap(),
		"pipeline_stage_map":         []any{},
		"outbound_field_map":  []any{},
		"inbound_stage_sync_enabled":   false,
		"sync_contact_updates_enabled": false,
	}
}
