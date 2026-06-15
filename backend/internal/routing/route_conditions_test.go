package routing

import (
	"encoding/json"
	"testing"
)

func TestEvalRouteConditions_empty(t *testing.T) {
	if !evalRouteConditions(nil, "and", nil, nil) {
		t.Fatal("empty conditions should match")
	}
}

func TestEvalPayloadCondition_eq(t *testing.T) {
	flat := map[string]any{"lead_source": "qualified"}
	c := RouteCondition{Domain: "payload", Field: "lead_source", Op: "eq", Value: json.RawMessage(`"qualified"`)}
	if !evalPayloadCondition(c, flat) {
		t.Fatal("expected match")
	}
	if evalPayloadCondition(c, map[string]any{"lead_source": "other"}) {
		t.Fatal("expected no match")
	}
}

func TestEvalRouteConditions_andOr(t *testing.T) {
	flat := map[string]any{"a": "1", "b": "2"}
	conds := []RouteCondition{
		{Domain: "payload", Field: "a", Op: "eq", Value: json.RawMessage(`"1"`)},
		{Domain: "payload", Field: "b", Op: "eq", Value: json.RawMessage(`"2"`)},
	}
	if !evalRouteConditions(conds, "and", nil, flat) {
		t.Fatal("expected and match")
	}
	if !evalRouteConditions(conds, "or", nil, map[string]any{"a": "x", "b": "2"}) {
		t.Fatal("expected partial or match")
	}
	if evalRouteConditions(conds, "and", nil, map[string]any{"a": "1", "b": "x"}) {
		t.Fatal("expected and miss")
	}
}

func TestPickMatchingBranch_priority(t *testing.T) {
	branches, _ := json.Marshal([]RouteBranch{
		{Position: 0, ConditionLogic: "and", Conditions: json.RawMessage(`[{"domain":"payload","field":"x","op":"eq","value":"no"}]`), Destination: "pipeline", Delivery: "leads_pipeline"},
		{Position: 1, Conditions: json.RawMessage(`[]`), Destination: "webhook", Delivery: "leads", DestWebhookID: ptrInt64(5)},
	})
	rt := &Route{Branches: branches}
	branch, pos, err := PickMatchingBranch(t.Context(), nil, 1, 1, rt, map[string]any{"x": "no"})
	if err != nil {
		t.Fatal(err)
	}
	if branch == nil || branch.Destination != "pipeline" || pos != 0 {
		t.Fatalf("branch = %+v pos=%d, want pipeline branch 0", branch, pos)
	}
}

func TestPickMatchingBranch_noMatch(t *testing.T) {
	branches, _ := json.Marshal([]RouteBranch{
		{Position: 0, ConditionLogic: "and", Conditions: json.RawMessage(`[{"domain":"payload","field":"x","op":"eq","value":"yes"}]`), Destination: "pipeline", Delivery: "leads"},
	})
	rt := &Route{Branches: branches}
	branch, _, err := PickMatchingBranch(t.Context(), nil, 1, 1, rt, map[string]any{"x": "no"})
	if err != nil {
		t.Fatal(err)
	}
	if branch != nil {
		t.Fatalf("branch = %+v, want nil", branch)
	}
}

func TestPickMatchingBranch_payloadRouting(t *testing.T) {
	branches, _ := json.Marshal([]RouteBranch{
		{Position: 0, Conditions: json.RawMessage(`[{"domain":"payload","field":"source","op":"eq","value":"a"}]`), Destination: "pipeline", Delivery: "leads_pipeline", TargetPipelineID: ptrInt64(1), TargetStageID: ptrInt64(2)},
		{Position: 1, Conditions: json.RawMessage(`[{"domain":"payload","field":"source","op":"eq","value":"b"}]`), Destination: "pipeline", Delivery: "leads_pipeline", TargetPipelineID: ptrInt64(1), TargetStageID: ptrInt64(3)},
		{Position: 2, Conditions: json.RawMessage(`[{"domain":"payload","field":"source","op":"eq","value":"c"}]`), Destination: "webhook", Delivery: "leads", DestWebhookID: ptrInt64(9)},
	})
	rt := &Route{Branches: branches}
	branch, _, err := PickMatchingBranch(t.Context(), nil, 1, 1, rt, map[string]any{"source": "c"})
	if err != nil {
		t.Fatal(err)
	}
	if branch == nil || branch.Destination != "webhook" || *branch.DestWebhookID != 9 {
		t.Fatalf("branch = %+v, want webhook dest 9", branch)
	}
}

func TestRouteForApply_overlaysBranch(t *testing.T) {
	rt := &Route{ID: 1, Destination: "pipeline", Delivery: "leads"}
	branch := &RouteBranch{Position: 2, Destination: "webhook", Delivery: "leads", DestWebhookID: ptrInt64(7)}
	out := RouteForApply(rt, branch)
	if out.Destination != "webhook" || out.MatchedBranchPosition != 2 || *out.DestWebhookID != 7 {
		t.Fatalf("out = %+v", out)
	}
}

func ptrInt64(v int64) *int64 { return &v }
