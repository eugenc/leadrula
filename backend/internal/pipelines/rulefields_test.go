package pipelines

import (
	"encoding/json"
	"testing"
	"time"
)

func TestResolveActionValue_fromFieldDateCopy(t *testing.T) {
	appt := time.Date(2024, 6, 15, 14, 30, 0, 0, time.UTC)
	ec := &ruleEvalContext{
		customByKey: map[string]customFieldDef{
			"appt_time_date": {id: 1, kind: kindDate, ftype: "datetime"},
		},
		customByID: map[int64]json.RawMessage{
			1: json.RawMessage(`"2024-06-15T14:30:00Z"`),
		},
	}

	raw := json.RawMessage(`{"from_field":"custom:appt_time_date"}`)
	resolved, skip, err := resolveActionValue(ec, "lead", "action_at", raw)
	if err != nil {
		t.Fatalf("resolveActionValue: %v", err)
	}
	if skip {
		t.Fatal("expected skip=false")
	}

	tm, err := resolveActionAtValue(resolved)
	if err != nil {
		t.Fatalf("resolveActionAtValue: %v", err)
	}
	if tm == nil || !tm.Equal(appt) {
		t.Fatalf("got time %v, want %v", tm, appt)
	}
}

func TestResolveActionValue_emptySourceSkips(t *testing.T) {
	ec := &ruleEvalContext{
		customByKey: map[string]customFieldDef{
			"appt_time_date": {id: 1, kind: kindDate, ftype: "datetime"},
		},
		customByID: map[int64]json.RawMessage{},
	}

	raw := json.RawMessage(`{"from_field":"custom:appt_time_date"}`)
	_, skip, err := resolveActionValue(ec, "lead", "action_at", raw)
	if err != nil {
		t.Fatalf("resolveActionValue: %v", err)
	}
	if !skip {
		t.Fatal("expected skip=true for empty source")
	}
}

func TestResolveActionValue_literalUnchanged(t *testing.T) {
	ec := &ruleEvalContext{}
	raw := json.RawMessage(`{"mode":"today"}`)
	resolved, skip, err := resolveActionValue(ec, "lead", "action_at", raw)
	if err != nil {
		t.Fatalf("resolveActionValue: %v", err)
	}
	if skip {
		t.Fatal("expected skip=false")
	}
	if string(resolved) != string(raw) {
		t.Fatalf("got %s, want %s", resolved, raw)
	}

	tm, err := resolveActionDateValue(resolved)
	if err != nil {
		t.Fatalf("resolveActionDateValue: %v", err)
	}
	if tm == nil {
		t.Fatal("expected today time")
	}
}

func TestValidateFromFieldRef_kindMismatch(t *testing.T) {
	customByKey := map[string]customFieldDef{
		"appt_time_date": {id: 1, kind: kindDate, ftype: "datetime"},
	}
	err := validateFromFieldRef(customByKey, "lead", "first_name", "custom:appt_time_date")
	if err == nil {
		t.Fatal("expected kind mismatch error")
	}
}

func TestValidateFromFieldRef_sameField(t *testing.T) {
	customByKey := map[string]customFieldDef{
		"appt_time_date": {id: 1, kind: kindDate, ftype: "datetime"},
	}
	err := validateFromFieldRef(customByKey, "lead", "custom:appt_time_date", "custom:appt_time_date")
	if err == nil {
		t.Fatal("expected same-field error")
	}
}

func TestValidateFromFieldRef_ok(t *testing.T) {
	customByKey := map[string]customFieldDef{
		"appt_time_date": {id: 1, kind: kindDate, ftype: "datetime"},
	}
	err := validateFromFieldRef(customByKey, "lead", "action_at", "custom:appt_time_date")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
