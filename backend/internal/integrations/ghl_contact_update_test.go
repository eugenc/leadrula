package integrations

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/echayko/leadrula/backend/internal/integrations/providers"
)

func TestGhlContactUpdateShouldEnqueue(t *testing.T) {
	enabled, _ := json.Marshal(map[string]any{
		"location_id":                  "loc1",
		"sync_contact_updates_enabled": true,
	})
	if !ghlContactUpdateShouldEnqueue(enabled) {
		t.Fatal("expected true when enabled in api mode")
	}
	disabled, _ := json.Marshal(map[string]any{
		"location_id":                  "loc1",
		"sync_contact_updates_enabled": false,
	})
	if ghlContactUpdateShouldEnqueue(disabled) {
		t.Fatal("expected false when disabled")
	}
	webhook, _ := json.Marshal(map[string]any{
		"location_id":                  "loc1",
		"delivery_mode":                "webhook",
		"webhook_url":                    "https://example.com/hook",
		"sync_contact_updates_enabled": true,
	})
	if ghlContactUpdateShouldEnqueue(webhook) {
		t.Fatal("expected false for webhook mode")
	}
}

func TestGHLContactPayloadChangedForConnection_standardFields(t *testing.T) {
	connConfig, _ := json.Marshal(map[string]any{"location_id": "loc1"})
	before, _ := json.Marshal(map[string]any{"first_name": "Jane", "phone": "555"})
	after, _ := json.Marshal(map[string]any{"first_name": "Jane", "phone": "556"})
	if GHLContactPayloadChangedForConnection(connConfig, before, before) {
		t.Fatal("expected no change for identical payloads")
	}
	if !GHLContactPayloadChangedForConnection(connConfig, before, after) {
		t.Fatal("expected phone change")
	}
	stageOnly, _ := json.Marshal(map[string]any{"first_name": "Jane", "phone": "555", "stage_id": 99})
	if GHLContactPayloadChangedForConnection(connConfig, before, stageOnly) {
		t.Fatal("stage-only change should not count as contact change")
	}
}

func TestGHLContactPayloadChangedForConnection_contactCustomFieldsOnly(t *testing.T) {
	cfid := int64(42)
	oppCF := int64(99)
	connConfig, _ := json.Marshal(map[string]any{
		"location_id": "loc1",
		"outbound_field_map": []map[string]any{
			{
				"dest_key":        "contact.solar",
				"source_type":     "custom",
				"custom_field_id": cfid,
				"ghl_field_model": "contact",
			},
			{
				"dest_key":        "opportunity.amount",
				"source_type":     "custom",
				"custom_field_id": oppCF,
				"ghl_field_model": "opportunity",
			},
		},
	})
	before, _ := json.Marshal(map[string]any{
		"first_name":     "Jane",
		"custom_fields": map[string]any{"42": "a", "99": "1"},
	})
	afterOppOnly, _ := json.Marshal(map[string]any{
		"first_name":     "Jane",
		"custom_fields": map[string]any{"42": "a", "99": "2"},
	})
	if GHLContactPayloadChangedForConnection(connConfig, before, afterOppOnly) {
		t.Fatal("opportunity-mapped custom field change should not trigger contact diff")
	}
	afterContact, _ := json.Marshal(map[string]any{
		"first_name":     "Jane",
		"custom_fields": map[string]any{"42": "b", "99": "1"},
	})
	if !GHLContactPayloadChangedForConnection(connConfig, before, afterContact) {
		t.Fatal("contact-mapped custom field change should be detected")
	}
}

func TestPayloadHasSkipOpportunityStage(t *testing.T) {
	raw := setSkipOpportunityStage([]byte(`{"first_name":"Jane"}`))
	if !payloadHasSkipOpportunityStage(raw) {
		t.Fatal("expected true after setSkipOpportunityStage")
	}
	if payloadHasSkipOpportunityStage([]byte(`{"first_name":"Jane"}`)) {
		t.Fatal("expected false without flag")
	}
}

func TestPlanGHLOutboundDeliver_preFlaggedContactUpdate(t *testing.T) {
	s := &Service{}
	enqueued := setSkipOpportunityStage([]byte(`{"first_name":"Jane"}`))
	plan := s.planGHLOutboundDeliver(context.Background(), 0, "", nil, enqueued, enqueued)
	if plan.Action != ghlOutboundDeliverContactOnly {
		t.Fatalf("action = %q, want %q", plan.Action, ghlOutboundDeliverContactOnly)
	}
}

func TestMergeGHLConfigDefaults_syncContactUpdates(t *testing.T) {
	cfg := providers.MergeGHLConfigDefaults(map[string]any{"location_id": "loc1"})
	if cfg["sync_contact_updates_enabled"] != false {
		t.Fatalf("default sync_contact_updates_enabled = %v, want false", cfg["sync_contact_updates_enabled"])
	}
}
