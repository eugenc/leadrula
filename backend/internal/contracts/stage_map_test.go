package contracts

import "testing"

func TestBuildDeliveryStageMaps_entryAndReturnRoutes(t *testing.T) {
	rules := []returnRuleStage{
		{buyerStageID: 50, returnStageID: 500},
		{buyerStageID: 60, returnStageID: 600},
	}
	maps := buildDeliveryStageMaps(10, 100, rules)
	if maps[10] != 100 {
		t.Fatalf("entry map = %d want 100", maps[10])
	}
	if maps[50] != 500 || maps[60] != 600 {
		t.Fatalf("return route maps = %v", maps)
	}
	if len(maps) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(maps))
	}
}

func TestBuildDeliveryStageMaps_mismatchedPipelinesOk(t *testing.T) {
	// Buyer and publisher pipelines can differ; only explicit delivery + return routes are mapped.
	maps := buildDeliveryStageMaps(42, 99, nil)
	if len(maps) != 1 || maps[42] != 99 {
		t.Fatalf("maps = %v want single entry 42→99", maps)
	}
}

func TestBuildDeliveryStageMaps_skipsZeroIDs(t *testing.T) {
	maps := buildDeliveryStageMaps(0, 100, []returnRuleStage{{buyerStageID: 0, returnStageID: 500}})
	if len(maps) != 0 {
		t.Fatalf("expected empty maps, got %v", maps)
	}
}

func TestBuildDeliveryStageMaps_returnRuleOverridesSameBuyerStage(t *testing.T) {
	rules := []returnRuleStage{{buyerStageID: 10, returnStageID: 200}}
	maps := buildDeliveryStageMaps(10, 100, rules)
	if maps[10] != 200 {
		t.Fatalf("return route should win for same buyer stage: %v", maps)
	}
}
