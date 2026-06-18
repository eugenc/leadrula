package webhooks

import "testing"

func TestAppendWebhookLeadFilter_leadID(t *testing.T) {
	where, args := appendWebhookLeadFilter("w.account_id = $1", []any{1}, 99, "")
	if where != "w.account_id = $1 AND d.lead_id = $2" {
		t.Fatalf("where = %q", where)
	}
	if len(args) != 2 || args[1] != int64(99) {
		t.Fatalf("args = %v", args)
	}
}

func TestAppendWebhookLeadFilter_search(t *testing.T) {
	where, args := appendWebhookLeadFilter("w.account_id = $1", []any{1}, 0, "555")
	if len(args) != 2 || args[1] != "%555%" {
		t.Fatalf("args = %v", args)
	}
	if where == "w.account_id = $1" {
		t.Fatal("expected search clause")
	}
}

func TestAppendWebhookLeadFilter_empty(t *testing.T) {
	where, args := appendWebhookLeadFilter("base", []any{1}, 0, "  ")
	if where != "base" || len(args) != 1 {
		t.Fatalf("where=%q args=%v", where, args)
	}
}
