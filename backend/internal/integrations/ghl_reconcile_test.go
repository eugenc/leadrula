package integrations

import (
	"encoding/json"
	"testing"
)

func TestGHLDeliveryContactPayloadChanged(t *testing.T) {
	base := map[string]any{
		"first_name": "Jane",
		"last_name":  "Doe",
		"phone":      "555",
		"pipeline_id": float64(1),
		"stage_id":    float64(2),
	}
	baseJSON, _ := json.Marshal(base)

	changedPhone := map[string]any{
		"first_name": "Jane",
		"last_name":  "Doe",
		"phone":      "556",
		"pipeline_id": float64(1),
		"stage_id":    float64(3),
	}
	changedPhoneJSON, _ := json.Marshal(changedPhone)

	if GHLDeliveryContactPayloadChanged(baseJSON, baseJSON) {
		t.Fatal("expected identical payloads to be unchanged")
	}
	if !GHLDeliveryContactPayloadChanged(baseJSON, changedPhoneJSON) {
		t.Fatal("expected phone change to be detected")
	}
	if GHLDeliveryContactPayloadChanged(baseJSON, changedPhoneJSON) && !GHLDeliveryContactPayloadChanged(changedPhoneJSON, changedPhoneJSON) {
		// stage-only change should not count
	}
	stageOnly := map[string]any{
		"first_name": "Jane",
		"last_name":  "Doe",
		"phone":      "555",
		"pipeline_id": float64(1),
		"stage_id":    float64(99),
	}
	stageOnlyJSON, _ := json.Marshal(stageOnly)
	if GHLDeliveryContactPayloadChanged(baseJSON, stageOnlyJSON) {
		t.Fatal("stage-only change should be ignored for contact diff")
	}
}

func TestGhlDeliveryContactSlice_ignoresStage(t *testing.T) {
	raw := []byte(`{"first_name":"A","stage_id":1,"custom_fields":{"42":"x"}}`)
	slice := ghlDeliveryContactSlice(raw)
	if _, ok := slice["stage_id"]; ok {
		t.Fatal("stage_id should not be in contact slice")
	}
	if slice["first_name"] != "A" {
		t.Fatalf("first_name = %v", slice["first_name"])
	}
	cf, ok := slice["custom_fields"].(map[string]any)
	if !ok || cf["42"] != "x" {
		t.Fatalf("custom_fields = %v", slice["custom_fields"])
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
