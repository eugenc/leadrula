package providers

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestParseGHLConfig_requiresLocation(t *testing.T) {
	_, err := ParseGHLConfig(map[string]any{})
	if err == nil {
		t.Fatal("expected error for missing location_id")
	}
}

func TestParseGHLConfig_createContactMustBeTrue(t *testing.T) {
	_, err := ParseGHLConfig(map[string]any{
		"location_id":    "loc1",
		"create_contact": false,
	})
	if err == nil {
		t.Fatal("expected error when create_contact is false")
	}
}

func TestParseGHLConfig_opportunityRequiresStageMap(t *testing.T) {
	_, err := ParseGHLConfig(map[string]any{
		"location_id":        "loc1",
		"create_opportunity": true,
	})
	if err == nil {
		t.Fatal("expected error when opportunity enabled without stage map")
	}
}

func TestParseGHLConfigForTest_allowsOpportunityWithoutStageMap(t *testing.T) {
	cfg, err := ParseGHLConfigForTest(map[string]any{
		"location_id":        "loc1",
		"create_opportunity": true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.LocationID != "loc1" {
		t.Fatalf("location_id: got %q", cfg.LocationID)
	}
}

func TestParseGHLConfigForTest_webhookMode(t *testing.T) {
	cfg, err := ParseGHLConfigForTest(map[string]any{
		"delivery_mode": "webhook",
		"webhook_url":   "https://hooks.gohighlevel.com/workflow/abc",
	})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.DeliveryMode != "webhook" || cfg.WebhookURL == "" {
		t.Fatalf("unexpected config: %+v", cfg)
	}
}

func TestResolveGHLStage(t *testing.T) {
	entries := []GHLPipelineStageMapEntry{
		{LeadrulaPipelineID: 1, LeadrulaStageID: 5, GHLPipelineID: "p1", GHLPipelineStageID: "s1"},
	}
	pid, sid, err := resolveGHLStage(entries, 1, 5)
	if err != nil || pid != "p1" || sid != "s1" {
		t.Fatalf("resolveGHLStage: pid=%s sid=%s err=%v", pid, sid, err)
	}
	_, _, err = resolveGHLStage(entries, 2, 5)
	if err == nil {
		t.Fatal("expected unmapped stage error")
	}
}

func TestResolveLeadrulaStage(t *testing.T) {
	entries := []GHLPipelineStageMapEntry{
		{LeadrulaPipelineID: 1, LeadrulaStageID: 5, GHLPipelineID: "p1", GHLPipelineStageID: "s1"},
		{LeadrulaPipelineID: 2, LeadrulaStageID: 9, GHLPipelineID: "p1", GHLPipelineStageID: "s2"},
	}
	stageID, ok := ResolveLeadrulaStage(entries, 1, "p1", "s1")
	if !ok || stageID != 5 {
		t.Fatalf("got stageID=%d ok=%v", stageID, ok)
	}
	_, ok = ResolveLeadrulaStage(entries, 1, "p1", "s2")
	if ok {
		t.Fatal("expected no match for wrong stage on pipeline 1")
	}
	stageID, ok = ResolveLeadrulaStage(entries, 2, "p1", "s2")
	if !ok || stageID != 9 {
		t.Fatalf("pipeline 2: got stageID=%d ok=%v", stageID, ok)
	}
}

func TestParseGHLOpportunitySearch(t *testing.T) {
	body := []byte(`{"opportunities":[{"id":"opp-1","pipelineId":"pipe-1","pipelineStageId":"stage-sit"}]}`)
	ref, err := parseGHLOpportunitySearch(body)
	if err != nil {
		t.Fatal(err)
	}
	if ref.ID != "opp-1" || ref.PipelineID != "pipe-1" || ref.PipelineStageID != "stage-sit" {
		t.Fatalf("unexpected ref: %+v", ref)
	}
	ref, err = parseGHLOpportunitySearch([]byte(`{"opportunities":[]}`))
	if err != nil || ref.ID != "" {
		t.Fatalf("empty search: ref=%+v err=%v", ref, err)
	}
}

func TestGHLInboundPipelineStage(t *testing.T) {
	p, s := GHLInboundPipelineStage(map[string]any{
		"pipelineId":      "gp1",
		"pipelineStageId": "gs1",
	})
	if p != "gp1" || s != "gs1" {
		t.Fatalf("got pipeline=%q stage=%q", p, s)
	}
	p, s = GHLInboundPipelineStage(map[string]any{"pipeline_id": "gp2", "stageId": "gs2"})
	if p != "gp2" || s != "gs2" {
		t.Fatalf("snake_case: got pipeline=%q stage=%q", p, s)
	}
	if p, s := GHLInboundPipelineStage(map[string]any{"pipelineId": "gp1"}); p != "gp1" || s != "" {
		t.Fatalf("missing stage: pipeline=%q stage=%q", p, s)
	}
}

func TestGHLInboundPipelineStageName(t *testing.T) {
	if got := GHLInboundPipelineStageName(map[string]any{"pipleline_stage": "PTO"}); got != "PTO" {
		t.Fatalf("pipleline_stage: got %q", got)
	}
	if got := GHLInboundPipelineStageName(map[string]any{"pippleine_stage": "Installed"}); got != "Installed" {
		t.Fatalf("pippleine_stage: got %q", got)
	}
	if got := GHLInboundPipelineStageName(map[string]any{"stageName": "Sit"}); got != "Sit" {
		t.Fatalf("stageName: got %q", got)
	}
	if got := GHLInboundPipelineStageName(map[string]any{"opportunity.pipeline_stage_name": "Signed"}); got != "Signed" {
		t.Fatalf("opportunity.pipeline_stage_name: got %q", got)
	}
	if got := GHLInboundPipelineStageName(map[string]any{"pipelineStageId": "uuid"}); got != "" {
		t.Fatalf("expected empty when only stage id present, got %q", got)
	}
}

func TestNormalizeGHLInboundFlat_defaultPayload(t *testing.T) {
	flat := NormalizeGHLInboundFlat(map[string]any{
		"contactId":       "contact-1",
		"id":              "opportunity-9",
		"pipeline_id":     "pipe-1",
		"pipleline_stage": "Sit",
	})
	if got := ghlInboundContactID(flat); got != "contact-1" {
		t.Fatalf("contact id = %q, want contact-1", got)
	}
	p, s := GHLInboundPipelineStage(flat)
	if p != "pipe-1" || s != "" {
		t.Fatalf("pipeline=%q stage=%q", p, s)
	}
	if got := GHLInboundPipelineStageName(flat); got != "Sit" {
		t.Fatalf("stage name = %q", got)
	}
}

func TestNormalizeGHLInboundFlat_nestedOpportunity(t *testing.T) {
	flat := NormalizeGHLInboundFlat(map[string]any{
		"contactId":                    "contact-2",
		"opportunity.pipelineId":       "pipe-2",
		"opportunity.pipelineStageId":  "stage-2",
		"opportunity.pipeline_stage_name": "Signed",
	})
	p, s := GHLInboundPipelineStage(flat)
	if p != "pipe-2" || s != "stage-2" {
		t.Fatalf("pipeline=%q stage=%q", p, s)
	}
	if got := GHLInboundPipelineStageName(flat); got != "Signed" {
		t.Fatalf("stage name = %q", got)
	}
}

func TestPrepareGHLInboundFlat_customData(t *testing.T) {
	flat := PrepareGHLInboundFlat(map[string]any{
		"contact_id": "c1",
		"customData": map[string]any{
			"appointment_disposition": "Reschedule",
		},
	}, nil)
	if got := ghlFlatText(flat, "appointment_disposition"); got != "Reschedule" {
		t.Fatalf("appointment_disposition = %q", got)
	}
	if got := ghlFlatText(flat, "opportunity.appointment_disposition"); got != "Reschedule" {
		t.Fatalf("opportunity.appointment_disposition = %q", got)
	}
}

func TestPrepareGHLInboundFlat_customFieldsArray(t *testing.T) {
	flat := PrepareGHLInboundFlat(map[string]any{
		"customFields": []any{
			map[string]any{
				"key":          "opportunity.disposition",
				"field_value":  "Showed",
			},
		},
	}, nil)
	if got := ghlFlatText(flat, "opportunity.disposition"); got != "Showed" {
		t.Fatalf("opportunity.disposition = %q", got)
	}
}

func TestPrepareGHLInboundFlat_customFieldsDoesNotOverwrite(t *testing.T) {
	flat := PrepareGHLInboundFlat(map[string]any{
		"opportunity.disposition": "Existing",
		"customFields": []any{
			map[string]any{
				"key":         "opportunity.disposition",
				"field_value": "Showed",
			},
		},
	}, nil)
	if got := ghlFlatText(flat, "opportunity.disposition"); got != "Existing" {
		t.Fatalf("expected existing value preserved, got %q", got)
	}
}

func TestPrepareGHLInboundFlat_customDataJSONString(t *testing.T) {
	flat := PrepareGHLInboundFlat(map[string]any{
		"customData": `{"appointment_disposition":"Reschedule"}`,
	}, nil)
	if got := ghlFlatText(flat, "appointment_disposition"); got != "Reschedule" {
		t.Fatalf("appointment_disposition = %q", got)
	}
}

func TestPrepareGHLInboundFlat_nestedCustomData(t *testing.T) {
	flat := PrepareGHLInboundFlat(map[string]any{
		"customData": map[string]any{
			"opportunity": map[string]any{
				"appointment_disposition": "Reschedule",
			},
		},
	}, nil)
	if got := ghlFlatText(flat, "appointment_disposition"); got != "Reschedule" {
		t.Fatalf("appointment_disposition = %q", got)
	}
	if got := ghlFlatText(flat, "opportunity.appointment_disposition"); got != "Reschedule" {
		t.Fatalf("opportunity.appointment_disposition = %q", got)
	}
}

func TestPrepareGHLInboundFlat_customFieldsBareKeyMirrorsOpportunity(t *testing.T) {
	flat := PrepareGHLInboundFlat(map[string]any{
		"customFields": []any{
			map[string]any{
				"key":         "appointment_disposition",
				"field_value": "Showed",
			},
		},
	}, nil)
	if got := ghlFlatText(flat, "appointment_disposition"); got != "Showed" {
		t.Fatalf("appointment_disposition = %q", got)
	}
	if got := ghlFlatText(flat, "opportunity.appointment_disposition"); got != "Showed" {
		t.Fatalf("opportunity.appointment_disposition = %q", got)
	}
}

func TestPrepareGHLInboundFlat_displayNameAliases(t *testing.T) {
	flat := PrepareGHLInboundFlat(map[string]any{
		"Appointment Date & Time": "2026-08-07T12:00:00Z",
		"Recording Link":          "https://example.com/rec.wav",
	}, map[string]string{
		"Appointment Date & Time": "appointment_date_time",
		"Recording Link":          "appointment_recording_link",
	})
	if got := ghlFlatText(flat, "appointment_date_time"); got != "2026-08-07T12:00:00Z" {
		t.Fatalf("appointment_date_time = %q", got)
	}
	if got := ghlFlatText(flat, "appointment_recording_link"); got != "https://example.com/rec.wav" {
		t.Fatalf("appointment_recording_link = %q", got)
	}
}

func TestPrepareGHLInboundFlat_realWorkflowPayload(t *testing.T) {
	flat := PrepareGHLInboundFlat(map[string]any{
		"contact_id": "q94AAT7iTxJbbG563wqS",
		"customData": map[string]any{
			"appointment_disposition": "[2026-08-07 16:58] Reschedule soon",
		},
		"Appointment Date & Time": "2026-08-07T12:00",
		"Recording Link":          "https://d3njiazx9u20q.cloudfront.net/rec.wav",
	}, map[string]string{
		"Appointment Date & Time": "appointment_date_time",
		"Recording Link":          "appointment_recording_link",
	})
	if got := ghlFlatText(flat, "appointment_disposition"); got == "" {
		t.Fatal("expected appointment_disposition from customData")
	}
	if got := ghlFlatText(flat, "appointment_date_time"); got != "2026-08-07T12:00" {
		t.Fatalf("appointment_date_time = %q", got)
	}
	if got := ghlFlatText(flat, "appointment_recording_link"); got == "" {
		t.Fatal("expected appointment_recording_link from display name alias")
	}
}

func TestPrepareGHLInboundFlat_customDataAppointmentNotesSlug(t *testing.T) {
	noteBody := "[2026-08-07 16:58] Reschedule soon"
	flat := PrepareGHLInboundFlat(map[string]any{
		"customData": map[string]any{
			"Appointment Notes": noteBody,
		},
	}, nil)
	if got := ghlFlatText(flat, "Appointment Notes"); got != noteBody {
		t.Fatalf("Appointment Notes = %q", got)
	}
	if got := ghlFlatText(flat, "appointment_notes"); got != noteBody {
		t.Fatalf("appointment_notes = %q", got)
	}
}

func TestPrepareGHLInboundFlat_customDataSkipsEmptyValues(t *testing.T) {
	flat := PrepareGHLInboundFlat(map[string]any{
		"customData": map[string]any{
			"appointment_disposition":   "",
			"Appointment Recording Link": "",
			"Appointment Notes":         "Has content",
		},
	}, nil)
	if _, ok := flat["appointment_disposition"]; ok {
		t.Fatalf("empty appointment_disposition should not be promoted, flat=%v", flat["appointment_disposition"])
	}
	if _, ok := flat["Appointment Recording Link"]; ok {
		t.Fatal("empty Appointment Recording Link should not be promoted")
	}
	if got := ghlFlatText(flat, "appointment_notes"); got != "Has content" {
		t.Fatalf("appointment_notes = %q", got)
	}
}

func TestPrepareGHLInboundFlat_customDataNameToKeyAlias(t *testing.T) {
	flat := PrepareGHLInboundFlat(map[string]any{
		"customData": map[string]any{
			"Recording Link": "https://example.com/rec.wav",
		},
	}, map[string]string{
		"Recording Link": "appointment_recording_link",
	})
	if got := ghlFlatText(flat, "appointment_recording_link"); got != "https://example.com/rec.wav" {
		t.Fatalf("appointment_recording_link = %q", got)
	}
}

func TestGHLInboundNameToKeyFromConfig(t *testing.T) {
	aliases := GHLInboundNameToKeyFromConfig(map[string]any{
		"outbound_field_map": []map[string]any{
			{
				"dest_key":        "appointment_date_time",
				"ghl_field_name":  "Appointment Date & Time",
				"source_type":     "builtin",
				"builtin_field":   "action_at",
				"ghl_field_model": "opportunity",
			},
		},
	})
	if got := aliases["Appointment Date & Time"]; got != "appointment_date_time" {
		t.Fatalf("alias = %q", got)
	}
}

func TestGHLInboundMapsFromConfig_opportunityAliases(t *testing.T) {
	cfid := int64(42)
	name := "Appointment Disposition"
	config := map[string]any{
		"outbound_field_map": []map[string]any{
			{
				"dest_key":        "appointment_disposition",
				"source_type":     "custom",
				"custom_field_id": cfid,
				"ghl_field_model": "opportunity",
				"ghl_field_name":  name,
			},
		},
	}
	maps := GHLInboundMapsFromConfig(config)
	keys := map[string]bool{}
	for _, m := range maps {
		if m.TargetType == "custom" && m.CustomFieldID != nil && *m.CustomFieldID == cfid {
			keys[m.SourceKey] = true
		}
	}
	for _, want := range []string{"appointment_disposition", "opportunity.appointment_disposition", name} {
		if !keys[want] {
			t.Fatalf("missing inbound source key %q, got %v", want, keys)
		}
	}
}

func TestGHLInboundContactID_prefersContactIdOverRootID(t *testing.T) {
	got := ghlInboundContactID(map[string]any{
		"contactId": "contact-abc",
		"id":        "opportunity-xyz",
	})
	if got != "contact-abc" {
		t.Fatalf("got %q, want contact-abc", got)
	}
}

func TestValidateInboundStageSync(t *testing.T) {
	cfg := GHLConfig{
		InboundStageSyncEnabled:       true,
		InboundSyncLeadrulaPipelineID: 1,
		InboundSyncGHLPipelineID:      "p1",
		PipelineStageMap: []GHLPipelineStageMapEntry{
			{LeadrulaPipelineID: 1, LeadrulaStageID: 5, GHLPipelineID: "p1", GHLPipelineStageID: "s1"},
		},
	}
	if err := validateInboundStageSync(cfg); err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if !InboundStageSyncReady(cfg) {
		t.Fatal("expected ready")
	}
	cfg.InboundSyncGHLPipelineID = ""
	if err := validateInboundStageSync(cfg); err == nil {
		t.Fatal("expected ghl pipeline required")
	}
}

func TestValidateGHLConfigJSON_inboundStageSyncRequiresMap(t *testing.T) {
	_, err := ParseGHLConfig(map[string]any{
		"location_id":                     "loc1",
		"inbound_stage_sync_enabled":      true,
		"inbound_sync_leadrula_pipeline_id": 1,
		"inbound_sync_ghl_pipeline_id":      "p1",
		"pipeline_stage_map":              []any{},
	})
	if err == nil {
		t.Fatal("expected validation error for missing stage map")
	}
}

func TestResolveGHLFieldSourceValue_builtin(t *testing.T) {
	bf := "first_name"
	fs := GHLFieldSource{SourceType: "builtin", BuiltinField: &bf}
	if got := resolveGHLFieldSourceValue(fs, DeliveryPayload{FirstName: "Jane"}); got != "Jane" {
		t.Fatalf("got %q", got)
	}
}

func TestResolveGHLFieldSourceValue_static(t *testing.T) {
	v := "Consultation"
	fs := GHLFieldSource{SourceType: "static", StaticValue: &v}
	if got := resolveGHLFieldSourceValue(fs, DeliveryPayload{}); got != "Consultation" {
		t.Fatalf("got %q", got)
	}
}

func TestResolveGHLFieldValue_actionAt(t *testing.T) {
	bf := "action_at"
	entry := SunbaseFieldMapEntry{
		DestKey:      "appointment_date_time",
		SourceType:   "builtin",
		BuiltinField: &bf,
	}
	payload := DeliveryPayload{
		ActionAt: "2026-06-19T23:00:00Z",
		Config: map[string]any{
			"account_timezone": "America/New_York",
		},
	}
	got := resolveGHLFieldValue(entry, payload)
	want := "2026-06-19T19:00"
	if got != want {
		t.Fatalf("resolveGHLFieldValue() = %q, want %q", got, want)
	}

	fs := GHLFieldSource{SourceType: "builtin", BuiltinField: &bf}
	if raw := resolveGHLFieldSourceValue(fs, payload); raw != "2026-06-19T23:00:00Z" {
		t.Fatalf("resolveGHLFieldSourceValue() = %q, want raw RFC3339 for appointments", raw)
	}
}

func TestBuildGHLContactBody_defaults(t *testing.T) {
	cfg := GHLConfig{LocationID: "loc1", CreateContact: true}
	body := buildGHLContactBody(cfg, DeliveryPayload{
		FirstName: "John",
		LastName:  "Doe",
		Phone:     "555",
		Email:     "j@test.com",
	})
	if body["firstName"] != "John" || body["locationId"] != "loc1" {
		t.Fatalf("unexpected body: %v", body)
	}
}

func TestBuildGHLContactBody_phoneOnlyOmitsEmail(t *testing.T) {
	cfg := GHLConfig{LocationID: "loc1", CreateContact: true}
	body := buildGHLContactBody(cfg, DeliveryPayload{
		FirstName: "James",
		LastName:  "Gabriel",
		Phone:     "+16314187142",
	})
	if _, ok := body["email"]; ok {
		t.Fatalf("email should be omitted for phone-only lead, body: %v", body)
	}
	if body["phone"] != "+16314187142" {
		t.Fatalf("phone: got %v", body["phone"])
	}
	if body["locationId"] != "loc1" {
		t.Fatalf("locationId: got %v", body["locationId"])
	}
}

func TestParseGHLCredentials_PIT(t *testing.T) {
	token, err := ParseGHLCredentials([]byte(`{"private_integration_token":"pit-abc"}`))
	if err != nil || token != "pit-abc" {
		t.Fatalf("token=%q err=%v", token, err)
	}
}

func TestResolveGHLTitleTemplate_mixed(t *testing.T) {
	got := resolveGHLTitleTemplate("Lead: {{first_name}} {{last_name}}", DeliveryPayload{
		FirstName: "Jane",
		LastName:  "Doe",
	})
	if got != "Lead: Jane Doe" {
		t.Fatalf("got %q", got)
	}
}

func TestResolveGHLTitleTemplate_custom(t *testing.T) {
	got := resolveGHLTitleTemplate("{{custom:42}}", DeliveryPayload{
		CustomFields: map[string]any{"42": "Solar"},
	})
	if got != "Solar" {
		t.Fatalf("got %q", got)
	}
}

func TestParseGHLTitleTemplate_legacyAppointmentTitle(t *testing.T) {
	cfg, err := ParseGHLConfig(map[string]any{
		"location_id": "loc1",
		"appointment_title": map[string]any{
			"source_type":   "builtin",
			"builtin_field": "first_name",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.AppointmentTitleTemplate != "{{first_name}}" {
		t.Fatalf("got %q", cfg.AppointmentTitleTemplate)
	}
}

func TestParseGHLConfig_appointmentRequiresTitleTemplate(t *testing.T) {
	_, err := ParseGHLConfig(map[string]any{
		"location_id":        "loc1",
		"create_appointment": true,
		"calendar_id":        "cal1",
		"appointment_timezone": "America/New_York",
		"appointment_datetime": map[string]any{
			"source_type":   "builtin",
			"builtin_field": "action_at",
		},
		"appointment_title_template": "   ",
	})
	if err == nil {
		t.Fatal("expected validation error for empty appointment_title_template")
	}
}

func TestParseGHLConfig_webhookMode(t *testing.T) {
	cfg, err := ParseGHLConfig(map[string]any{
		"delivery_mode": "webhook",
		"webhook_url":   "https://hooks.gohighlevel.com/workflow/abc",
	})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.DeliveryMode != "webhook" || cfg.WebhookURL == "" {
		t.Fatalf("unexpected config: %+v", cfg)
	}
}

func TestParseGHLConfig_webhookRequiresURL(t *testing.T) {
	_, err := ParseGHLConfig(map[string]any{
		"delivery_mode": "webhook",
	})
	if err == nil {
		t.Fatal("expected error for missing webhook_url")
	}
}

func TestBuildGHLWebhookPayload(t *testing.T) {
	cfg := GHLConfig{
		DeliveryMode: "webhook",
		WebhookURL:   "https://example.com/hook",
		PipelineStageMap: []GHLPipelineStageMapEntry{
			{LeadrulaPipelineID: 1, LeadrulaStageID: 5, GHLPipelineID: "p1", GHLPipelineStageID: "s1"},
		},
	}
	body := buildGHLWebhookPayload(cfg, DeliveryPayload{
		LeadID:     "abc-uuid-123",
		FirstName:  "Jane",
		LastName:   "Doe",
		Phone:      "555",
		PipelineID: 1,
		StageID:    5,
	})
	if body["firstName"] != "Jane" {
		t.Fatalf("firstName: %v", body["firstName"])
	}
	if body["lead_id"] != "abc-uuid-123" {
		t.Fatalf("lead_id: %v", body["lead_id"])
	}
	if body["leadrula_pipeline_id"] != int64(1) {
		t.Fatalf("leadrula_pipeline_id: %v", body["leadrula_pipeline_id"])
	}
	if body["leadrula_stage_id"] != int64(5) {
		t.Fatalf("leadrula_stage_id: %v", body["leadrula_stage_id"])
	}
	if _, ok := body["ghl_pipeline_id"]; ok {
		t.Fatalf("ghl_pipeline_id should be omitted from webhook payload")
	}
	if _, ok := body["ghl_pipeline_stage_id"]; ok {
		t.Fatalf("ghl_pipeline_stage_id should be omitted from webhook payload")
	}
}

func TestParsePipelineStageMap_webhookMode(t *testing.T) {
	entries := parsePipelineStageMap([]map[string]any{
		{"leadrula_pipeline_id": 1, "leadrula_stage_id": 5},
		{"leadrula_pipeline_id": 2, "leadrula_stage_id": 0},
	}, "webhook")
	if len(entries) != 1 || entries[0].LeadrulaPipelineID != 1 || entries[0].LeadrulaStageID != 5 {
		t.Fatalf("unexpected webhook entries: %+v", entries)
	}
}

func TestMatchesGHLWebhookTrigger(t *testing.T) {
	entries := []GHLPipelineStageMapEntry{
		{LeadrulaPipelineID: 1, LeadrulaStageID: 5},
	}
	if !MatchesGHLWebhookTrigger(entries, 1, 5) {
		t.Fatal("expected match")
	}
	if MatchesGHLWebhookTrigger(entries, 2, 5) {
		t.Fatal("expected no match for wrong pipeline")
	}
	if MatchesGHLWebhookTrigger(nil, 1, 5) {
		t.Fatal("expected no match for empty map")
	}
}

func TestGhlWebhookTestPayload_standardContactFields(t *testing.T) {
	cfg := GHLConfig{
		DeliveryMode: "webhook",
		WebhookURL:   "https://example.com/hook",
		CreateContact: true,
	}
	body := buildGHLWebhookPayload(cfg, ghlWebhookTestPayload())

	want := map[string]string{
		"firstName":  "Leadrula",
		"lastName":   "Test",
		"phone":      "+15555550100",
		"email":      "test@leadrula.example",
		"address1":   "123 Test St",
		"city":       "Miami",
		"state":      "FL",
		"postalCode": "33101",
		"source":     "leadrula_test",
	}
	for k, v := range want {
		if body[k] != v {
			t.Fatalf("%s: got %v want %q", k, body[k], v)
		}
	}
	if _, ok := body["locationId"]; ok {
		t.Fatal("locationId should be omitted from webhook payload")
	}
	if _, ok := body["tags"]; ok {
		t.Fatal("tags should be omitted from webhook payload")
	}
	if body["lead_id"] != "00000000-0000-0000-0000-000000000001" {
		t.Fatalf("lead_id: got %v", body["lead_id"])
	}
}

func TestParseGHLConfig_appointmentDefaultsDatetime(t *testing.T) {
	cfg, err := ParseGHLConfig(map[string]any{
		"location_id":              "loc1",
		"create_appointment":       true,
		"calendar_id":              "cal1",
		"appointment_timezone":     "America/New_York",
		"appointment_title_template": "{{first_name}}",
	})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.AppointmentDatetime.SourceType != "builtin" || cfg.AppointmentDatetime.BuiltinField == nil || *cfg.AppointmentDatetime.BuiltinField != "action_at" {
		t.Fatalf("expected default action_at datetime, got %+v", cfg.AppointmentDatetime)
	}
}

func TestBuildGHLContactBody_customFields(t *testing.T) {
	fieldID := "cf-abc"
	fieldKey := "contact.roof_type"
	model := "contact"
	cfg := GHLConfig{
		LocationID:    "loc1",
		CreateContact: true,
		OutboundFieldMap: []SunbaseFieldMapEntry{
			{
				DestKey:          fieldKey,
				SourceType:       "static",
				StaticValue:      strPtr("Solar"),
				GHLCustomFieldID: &fieldID,
				GHLFieldModel:    &model,
			},
		},
	}
	body := buildGHLContactBody(cfg, DeliveryPayload{})
	raw, ok := body["customFields"].([]map[string]any)
	if !ok {
		t.Fatalf("expected customFields array, got %T: %v", body["customFields"], body["customFields"])
	}
	if len(raw) != 1 {
		t.Fatalf("expected 1 custom field, got %d", len(raw))
	}
	if raw[0]["id"] != fieldID || raw[0]["key"] != fieldKey || raw[0]["field_value"] != "Solar" {
		t.Fatalf("unexpected custom field: %v", raw[0])
	}
	if _, ok := body[fieldKey]; ok {
		t.Fatalf("custom field should not be top-level key")
	}
}

func TestBuildGHLContactBody_legacyKeyOnly(t *testing.T) {
	cfg := GHLConfig{
		LocationID:    "loc1",
		CreateContact: true,
		OutboundFieldMap: []SunbaseFieldMapEntry{
			{
				DestKey:     "contact.legacy_field",
				SourceType:  "static",
				StaticValue: strPtr("value"),
			},
		},
	}
	body := buildGHLContactBody(cfg, DeliveryPayload{})
	raw, ok := body["customFields"].([]map[string]any)
	if !ok || len(raw) != 1 {
		t.Fatalf("expected legacy key-only custom field, got %v", body["customFields"])
	}
	if raw[0]["key"] != "contact.legacy_field" {
		t.Fatalf("unexpected key: %v", raw[0]["key"])
	}
	if _, hasID := raw[0]["id"]; hasID {
		t.Fatal("legacy mapping should not include id")
	}
}

func TestGhlCustomFieldsPayload_opportunityOnly(t *testing.T) {
	fieldID := "opp-cf"
	fieldKey := "opportunity.disposition"
	model := "opportunity"
	contactModel := "contact"
	entries := []SunbaseFieldMapEntry{
		{
			DestKey:          fieldKey,
			SourceType:       "static",
			StaticValue:      strPtr("Showed"),
			GHLCustomFieldID: &fieldID,
			GHLFieldModel:    &model,
		},
		{
			DestKey:       "contact.notes",
			SourceType:    "static",
			StaticValue:   strPtr("note"),
			GHLFieldModel: &contactModel,
		},
	}
	oppFields := ghlCustomFieldsPayload(entries, DeliveryPayload{}, "opportunity")
	if len(oppFields) != 1 || oppFields[0]["key"] != fieldKey {
		t.Fatalf("unexpected opportunity fields: %v", oppFields)
	}
	contactFields := ghlCustomFieldsPayload(entries, DeliveryPayload{}, "contact")
	if len(contactFields) != 1 || contactFields[0]["key"] != "contact.notes" {
		t.Fatalf("unexpected contact fields: %v", contactFields)
	}
}

func TestParseAppointmentTimes(t *testing.T) {
	start, end, err := parseAppointmentTimes("2024-06-15T10:00:00", "America/New_York", 30*time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if start == "" || end == "" {
		t.Fatal("expected ISO times")
	}
}

func TestApplyGHLOpportunityStandardFields(t *testing.T) {
	v := "5000"
	bf := "cost"
	opp := map[string]any{"status": "open"}
	applyGHLOpportunityStandardFields(opp, GHLOpportunityStandardFields{
		MonetaryValue:  GHLFieldSource{SourceType: "static", StaticValue: &v},
		AssignedUserID: GHLFieldSource{SourceType: "static", StaticValue: strPtr("user-1")},
		Status:         GHLFieldSource{SourceType: "builtin", BuiltinField: &bf},
	}, DeliveryPayload{})
	if opp["monetaryValue"] != float64(5000) {
		t.Fatalf("monetaryValue: %v", opp["monetaryValue"])
	}
	if opp["assignedTo"] != "user-1" {
		t.Fatalf("assignedTo: %v", opp["assignedTo"])
	}
}

func TestGHLInboundMapsFromConfig_invertsOutbound(t *testing.T) {
	cfid := int64(42)
	config := map[string]any{
		"location_id": "loc1",
		"outbound_field_map": []map[string]any{
			{
				"dest_key":        "contact.solar_type",
				"source_type":     "custom",
				"custom_field_id": cfid,
				"ghl_field_model": "contact",
			},
			{
				"dest_key":     "contact.static_only",
				"source_type":  "static",
				"static_value": "x",
			},
		},
		"opportunity_standard_fields": map[string]any{
			"monetary_value": map[string]any{
				"source_type":     "custom",
				"custom_field_id": cfid,
			},
		},
	}
	maps := GHLInboundMapsFromConfig(config)
	if len(maps) < 2 {
		t.Fatalf("expected at least 2 inbound maps, got %d", len(maps))
	}
	foundSolar := false
	foundMonetary := false
	for _, m := range maps {
		if m.SourceKey == "contact.solar_type" && m.TargetType == "custom" && m.CustomFieldID != nil && *m.CustomFieldID == cfid {
			foundSolar = true
		}
		if m.SourceKey == "monetaryValue" && m.TargetType == "custom" {
			foundMonetary = true
		}
	}
	if !foundSolar || !foundMonetary {
		t.Fatalf("missing inverted maps: solar=%v monetary=%v maps=%+v", foundSolar, foundMonetary, maps)
	}
}

func TestGhlSkipOpportunityStage(t *testing.T) {
	if !ghlSkipOpportunityStage(map[string]any{"skip_opportunity_stage": true}) {
		t.Fatal("expected true for bool flag")
	}
	if !ghlSkipOpportunityStage(map[string]any{"skip_opportunity_stage": "true"}) {
		t.Fatal("expected true for string flag")
	}
	if ghlSkipOpportunityStage(map[string]any{"skip_opportunity_stage": false}) {
		t.Fatal("expected false")
	}
	if ghlSkipOpportunityStage(nil) {
		t.Fatal("expected false for nil config")
	}
}

func TestGHLProviderDeliver_webhookSkippedWhenStageSynced(t *testing.T) {
	p := &GHLProvider{}
	cfg := map[string]any{
		"delivery_mode":            "webhook",
		"webhook_url":              "https://example.com/hook",
		"location_id":              "loc1",
		"skip_opportunity_stage":   true,
		"pipeline_stage_map": []map[string]any{
			{"leadrula_pipeline_id": 1, "leadrula_stage_id": 5},
		},
	}
	_, err := p.Deliver(context.Background(), nil, DeliveryPayload{
		PipelineID: 1,
		StageID:    5,
		Config:     cfg,
	})
	if err == nil {
		t.Fatal("expected skip error when stage already synced")
	}
	if !IsDeliverySkipped(err) {
		t.Fatalf("expected DeliverySkippedError, got %v", err)
	}
}

func TestGHLProviderDeliver_webhookUnmappedStageSkipped(t *testing.T) {
	p := &GHLProvider{}
	cfg := map[string]any{
		"delivery_mode": "webhook",
		"webhook_url":   "https://example.com/hook",
		"location_id":   "loc1",
		"pipeline_stage_map": []map[string]any{
			{"leadrula_pipeline_id": 1, "leadrula_stage_id": 5},
		},
	}
	_, err := p.Deliver(context.Background(), nil, DeliveryPayload{
		PipelineID: 1,
		StageID:    99,
		Config:     cfg,
	})
	if err == nil {
		t.Fatal("expected skip error for unmapped stage")
	}
	if !IsDeliverySkipped(err) {
		t.Fatalf("expected DeliverySkippedError, got %v", err)
	}
}

func TestParseGHLConfig_syncContactUpdatesEnabled(t *testing.T) {
	cfg, err := ParseGHLConfig(map[string]any{
		"location_id":                  "loc1",
		"sync_contact_updates_enabled": true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.SyncContactUpdatesEnabled {
		t.Fatal("expected sync_contact_updates_enabled true")
	}
}

func TestGHLProviderDeliver_contactOnlySkipsOpportunityAndAppointment(t *testing.T) {
	cfg := map[string]any{
		"skip_opportunity_stage": true,
	}
	if !ghlSkipOpportunityStage(cfg) {
		t.Fatal("expected skip_opportunity_stage to be recognized")
	}
}

func TestBuildGHLContactBody_customStandardMapping(t *testing.T) {
	cfid := int64(77)
	cfg, err := ParseGHLConfig(map[string]any{
		"location_id": "loc1",
		"contact_standard_fields": map[string]any{
			"firstName": map[string]any{
				"source_type":     "custom",
				"custom_field_id": cfid,
			},
			"lastName": map[string]any{
				"source_type":   "builtin",
				"builtin_field": "last_name",
			},
			"phone": map[string]any{
				"source_type":   "builtin",
				"builtin_field": "phone",
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	body := buildGHLContactBody(cfg, DeliveryPayload{
		FirstName:    "ignored",
		LastName:     "Doe",
		Phone:        "555",
		CustomFields: map[string]any{"77": "MappedFirst"},
	})
	if body["firstName"] != "MappedFirst" {
		t.Fatalf("firstName = %v, want MappedFirst", body["firstName"])
	}
	if body["lastName"] != "Doe" || body["phone"] != "555" {
		t.Fatalf("unexpected body: %v", body)
	}
}

func TestParseGHLConfig_contactStandardFieldsRequired(t *testing.T) {
	_, err := ParseGHLConfig(map[string]any{
		"location_id": "loc1",
		"contact_standard_fields": map[string]any{
			"firstName": map[string]any{
				"source_type":   "builtin",
				"builtin_field": "first_name",
			},
		},
	})
	if err == nil || !strings.Contains(err.Error(), "contact_standard_fields.") || !strings.Contains(err.Error(), "is required") {
		t.Fatalf("expected required contact field error, got %v", err)
	}
}

func TestParseGHLConfig_contactStandardFieldsOptionalOmit(t *testing.T) {
	cfg, err := ParseGHLConfig(map[string]any{
		"location_id": "loc1",
		"contact_standard_fields": map[string]any{
			"firstName": map[string]any{"source_type": "builtin", "builtin_field": "first_name"},
			"lastName":  map[string]any{"source_type": "builtin", "builtin_field": "last_name"},
			"phone":     map[string]any{"source_type": "builtin", "builtin_field": "phone"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	body := buildGHLContactBody(cfg, DeliveryPayload{
		FirstName: "Jane",
		LastName:  "Doe",
		Phone:     "555",
		Email:     "j@test.com",
	})
	if _, ok := body["email"]; ok {
		t.Fatalf("email should be omitted when not mapped, body: %v", body)
	}
}

func TestGHLContactPayloadChanged_remappedFirstName(t *testing.T) {
	cfid := int64(77)
	cfg, err := ParseGHLConfig(map[string]any{
		"location_id": "loc1",
		"contact_standard_fields": map[string]any{
			"firstName": map[string]any{"source_type": "custom", "custom_field_id": cfid},
			"lastName":  map[string]any{"source_type": "builtin", "builtin_field": "last_name"},
			"phone":     map[string]any{"source_type": "builtin", "builtin_field": "phone"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	before, _ := json.Marshal(map[string]any{
		"first_name":     "Jane",
		"custom_fields": map[string]any{"77": "A"},
	})
	afterBuiltin, _ := json.Marshal(map[string]any{
		"first_name":     "Janet",
		"custom_fields": map[string]any{"77": "A"},
	})
	if GHLContactPayloadChanged(cfg, before, afterBuiltin) {
		t.Fatal("builtin first_name change should be ignored when firstName maps to custom field")
	}
	afterCustom, _ := json.Marshal(map[string]any{
		"first_name":     "Jane",
		"custom_fields": map[string]any{"77": "B"},
	})
	if !GHLContactPayloadChanged(cfg, before, afterCustom) {
		t.Fatal("custom mapped firstName change should be detected")
	}
}

func TestGHLInboundMapsFromConfig_contactStandardFields(t *testing.T) {
	cfid := int64(42)
	maps := GHLInboundMapsFromConfig(map[string]any{
		"contact_standard_fields": map[string]any{
			"firstName": map[string]any{"source_type": "custom", "custom_field_id": cfid},
		},
	})
	var found bool
	for _, m := range maps {
		if m.SourceKey == "firstName" && m.TargetType == "custom" && m.CustomFieldID != nil && *m.CustomFieldID == cfid {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected firstName inbound map to custom 42, got %+v", maps)
	}
}

func TestGhlExtractAppointmentEventID(t *testing.T) {
	tests := []struct {
		body string
		want string
	}{
		{`{"id":"evt-1"}`, "evt-1"},
		{`{"eventId":"evt-2"}`, "evt-2"},
		{`{"event":{"id":"evt-3"}}`, "evt-3"},
		{`{}`, ""},
	}
	for _, tc := range tests {
		if got := ghlExtractAppointmentEventID([]byte(tc.body)); got != tc.want {
			t.Fatalf("body %q: got %q want %q", tc.body, got, tc.want)
		}
	}
}

func TestGhlIsAppointmentNotFound(t *testing.T) {
	if !ghlIsAppointmentNotFound(DeliveryResult{HTTPStatus: 404, Raw: []byte(`{"message":"not found"}`)}) {
		t.Fatal("expected 404 to be not found")
	}
	if !ghlIsAppointmentNotFound(DeliveryResult{HTTPStatus: 400, Raw: []byte(`{"message":"Event not found"}`)}) {
		t.Fatal("expected 400 not found message")
	}
	if ghlIsAppointmentNotFound(DeliveryResult{HTTPStatus: 400, Raw: []byte(`{"message":"invalid time"}`)}) {
		t.Fatal("expected unrelated 400 to be false")
	}
}

func TestDeliveryPayloadActionAtChanged(t *testing.T) {
	base := []byte(`{"action_at":"2026-08-01T10:00:00-04:00","first_name":"A"}`)
	same := []byte(`{"action_at":"2026-08-01T10:00:00-04:00","first_name":"B"}`)
	changed := []byte(`{"action_at":"2026-08-02T14:00:00-04:00","first_name":"A"}`)
	missing := []byte(`{"first_name":"A"}`)

	if DeliveryPayloadActionAtChanged(base, same) {
		t.Fatal("expected same action_at to be unchanged")
	}
	if !DeliveryPayloadActionAtChanged(base, changed) {
		t.Fatal("expected action_at change")
	}
	if !DeliveryPayloadActionAtChanged(base, missing) {
		t.Fatal("expected missing action_at to count as changed")
	}
}
