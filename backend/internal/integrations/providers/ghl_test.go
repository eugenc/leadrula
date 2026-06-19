package providers

import (
	"testing"
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
	if body["ghl_pipeline_id"] != "p1" || body["ghl_pipeline_stage_id"] != "s1" {
		t.Fatalf("ghl ids: %v %v", body["ghl_pipeline_id"], body["ghl_pipeline_stage_id"])
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

func TestParseAppointmentTimes(t *testing.T) {
	start, end, err := parseAppointmentTimes("2024-06-15T10:00:00", "America/New_York")
	if err != nil {
		t.Fatal(err)
	}
	if start == "" || end == "" {
		t.Fatal("expected ISO times")
	}
}
