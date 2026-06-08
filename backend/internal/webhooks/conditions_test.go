package webhooks

import "testing"

func TestEvalPayloadConditions(t *testing.T) {
	flat := map[string]any{"status": "Booked", "first_name": "Test"}
	conds := []PayloadCondition{{Field: "status", Op: "eq", Value: []byte(`"Booked"`)}}
	if !evalPayloadConditions(conds, "and", flat) {
		t.Fatal("expected match")
	}
	if evalPayloadConditions(conds, "and", map[string]any{"status": "Cancelled"}) {
		t.Fatal("expected no match")
	}
	if !evalPayloadConditions(nil, "and", flat) {
		t.Fatal("empty conditions should always match")
	}
}
