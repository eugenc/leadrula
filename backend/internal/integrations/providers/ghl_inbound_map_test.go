package providers

import "testing"

func TestGHLInboundMapsFromConfig_empty(t *testing.T) {
	if maps := GHLInboundMapsFromConfig(nil); len(maps) != 0 {
		t.Fatalf("expected empty, got %v", maps)
	}
}

func TestInvertStandardFields_skipsStatic(t *testing.T) {
	maps := invertStandardFields(map[string]any{
		"status": map[string]any{
			"source_type":  "static",
			"static_value": "won",
		},
	}, ghlOpportunityStandardInbound)
	if len(maps) != 0 {
		t.Fatalf("static standard fields should not invert, got %v", maps)
	}
}
