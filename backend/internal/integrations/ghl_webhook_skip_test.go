package integrations

import (
	"encoding/json"
	"testing"
)

func TestGhlWebhookSkipReason_apiMode(t *testing.T) {
	cfg := json.RawMessage(`{"delivery_mode":"api","location_id":"loc1"}`)
	if reason := ghlWebhookSkipReason("ghl", cfg, 1, 5); reason != "" {
		t.Fatalf("api mode should not skip, got %q", reason)
	}
}

func TestGhlStageMoveShouldEnqueue_apiModeMapped(t *testing.T) {
	cfg := json.RawMessage(`{
		"delivery_mode":"api",
		"location_id":"loc1",
		"create_opportunity":true,
		"pipeline_stage_map":[{"leadrula_pipeline_id":1,"leadrula_stage_id":5,"ghl_pipeline_id":"p1","ghl_pipeline_stage_id":"s1"}]
	}`)
	if !ghlStageMoveShouldEnqueue(cfg, 1, 5) {
		t.Fatal("expected api mode enqueue for mapped stage with create_opportunity")
	}
}

func TestGhlStageMoveShouldEnqueue_apiModeUnmapped(t *testing.T) {
	cfg := json.RawMessage(`{
		"delivery_mode":"api",
		"location_id":"loc1",
		"create_opportunity":true,
		"pipeline_stage_map":[{"leadrula_pipeline_id":1,"leadrula_stage_id":5}]
	}`)
	if ghlStageMoveShouldEnqueue(cfg, 1, 99) {
		t.Fatal("expected no enqueue for unmapped stage")
	}
}

func TestGhlStageMoveShouldEnqueue_apiModeNoOpportunity(t *testing.T) {
	cfg := json.RawMessage(`{
		"delivery_mode":"api",
		"location_id":"loc1",
		"create_opportunity":false,
		"pipeline_stage_map":[{"leadrula_pipeline_id":1,"leadrula_stage_id":5}]
	}`)
	if ghlStageMoveShouldEnqueue(cfg, 1, 5) {
		t.Fatal("expected no enqueue when create_opportunity is false")
	}
}

func TestGhlWebhookSkipReason_webhookUnmappedStage(t *testing.T) {
	cfg := json.RawMessage(`{
		"delivery_mode":"webhook",
		"webhook_url":"https://example.com/hook",
		"pipeline_stage_map":[{"leadrula_pipeline_id":1,"leadrula_stage_id":5}]
	}`)
	if reason := ghlWebhookSkipReason("ghl", cfg, 1, 99); reason == "" {
		t.Fatal("expected skip reason for unmapped stage")
	}
}

func TestGhlWebhookSkipReason_webhookMappedStage(t *testing.T) {
	cfg := json.RawMessage(`{
		"delivery_mode":"webhook",
		"webhook_url":"https://example.com/hook",
		"pipeline_stage_map":[{"leadrula_pipeline_id":1,"leadrula_stage_id":5}]
	}`)
	if reason := ghlWebhookSkipReason("ghl", cfg, 1, 5); reason != "" {
		t.Fatalf("mapped stage should enqueue, got %q", reason)
	}
}
