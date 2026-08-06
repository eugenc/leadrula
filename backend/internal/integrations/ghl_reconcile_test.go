package integrations

import (
	"encoding/json"
	"testing"

	"github.com/echayko/leadrula/backend/internal/integrations/providers"
)

func TestGHLDeliveryContactPayloadChanged(t *testing.T) {
	cfg, err := providers.ParseGHLConfig(providers.MergeGHLConfigDefaults(map[string]any{"location_id": "loc1"}))
	if err != nil {
		t.Fatal(err)
	}
	base := map[string]any{
		"first_name":  "Jane",
		"last_name":   "Doe",
		"phone":       "555",
		"pipeline_id": float64(1),
		"stage_id":    float64(2),
	}
	baseJSON, _ := json.Marshal(base)

	changedPhone := map[string]any{
		"first_name":  "Jane",
		"last_name":   "Doe",
		"phone":       "556",
		"pipeline_id": float64(1),
		"stage_id":    float64(3),
	}
	changedPhoneJSON, _ := json.Marshal(changedPhone)

	if providers.GHLContactPayloadChanged(cfg, baseJSON, baseJSON) {
		t.Fatal("expected identical payloads to be unchanged")
	}
	if !providers.GHLContactPayloadChanged(cfg, baseJSON, changedPhoneJSON) {
		t.Fatal("expected phone change to be detected")
	}
	stageOnly := map[string]any{
		"first_name":  "Jane",
		"last_name":   "Doe",
		"phone":       "555",
		"pipeline_id": float64(1),
		"stage_id":    float64(99),
	}
	stageOnlyJSON, _ := json.Marshal(stageOnly)
	if providers.GHLContactPayloadChanged(cfg, baseJSON, stageOnlyJSON) {
		t.Fatal("stage-only change should be ignored for contact diff")
	}
}

func TestGHLContactPayloadSlice_ignoresStage(t *testing.T) {
	cfg, err := providers.ParseGHLConfig(providers.MergeGHLConfigDefaults(map[string]any{"location_id": "loc1"}))
	if err != nil {
		t.Fatal(err)
	}
	raw := []byte(`{"first_name":"A","stage_id":1,"custom_fields":{"42":"x"}}`)
	slice := providers.GHLContactPayloadSlice(cfg, raw)
	if _, ok := slice["stage_id"]; ok {
		t.Fatal("stage_id should not be in contact slice")
	}
	if slice["first_name"] != "A" {
		t.Fatalf("first_name = %v", slice["first_name"])
	}
	if _, ok := slice["custom_fields"]; ok {
		t.Fatal("unmapped custom_fields should not be included")
	}
}

func TestSetSkipOpportunityStage(t *testing.T) {
	out := setSkipOpportunityStage([]byte(`{"first_name":"Jane"}`))
	var m map[string]any
	if err := json.Unmarshal(out, &m); err != nil {
		t.Fatal(err)
	}
	cfg, ok := m["_config"].(map[string]any)
	if !ok || cfg["skip_opportunity_stage"] != true {
		t.Fatalf("_config = %v", m["_config"])
	}
}
