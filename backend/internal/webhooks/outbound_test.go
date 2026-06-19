package webhooks

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/echayko/leadrula/backend/internal/database"
	"github.com/echayko/leadrula/backend/internal/leads"
)

func TestBuildURLPayload_sunbaseDatetimeFormat(t *testing.T) {
	fid := int64(7)
	lead := &leads.Lead{
		CustomValues: map[string]json.RawMessage{
			"7": json.RawMessage(`"2026-06-08T14:30:00-04:00"`),
		},
	}
	entries := []OutboundFieldMapEntry{
		{DestKey: "appt_time", SourceType: "custom", CustomFieldID: &fid},
	}
	fieldTypes := map[string]string{"7": "datetime"}
	payload, err := buildURLPayload("lead.update", lead, PipelineContext{}, entries, fieldTypes, "")
	if err != nil {
		t.Fatal(err)
	}
	var out map[string]string
	if err := json.Unmarshal(payload, &out); err != nil {
		t.Fatal(err)
	}
	if out["appt_time"] != "2026-06-08T14:30" {
		t.Fatalf("appt_time = %q, want 2026-06-08T14:30", out["appt_time"])
	}
}

func TestBuildURLPayload_actionAtSunbaseTimezone(t *testing.T) {
	actionAt := time.Date(2026, 6, 19, 23, 0, 0, 0, time.UTC)
	bf := "action_at"
	lead := &leads.Lead{ActionAt: &actionAt}
	entries := []OutboundFieldMapEntry{
		{DestKey: "appt_time", SourceType: "builtin", BuiltinField: &bf},
	}
	fieldTypes := map[string]string{}
	payload, err := buildURLPayload("lead.update", lead, PipelineContext{}, entries, fieldTypes, "America/New_York")
	if err != nil {
		t.Fatal(err)
	}
	var out map[string]string
	if err := json.Unmarshal(payload, &out); err != nil {
		t.Fatal(err)
	}
	if out["appt_time"] != "2026-06-19T19:00" {
		t.Fatalf("appt_time = %q, want 2026-06-19T19:00", out["appt_time"])
	}
}

func TestBuildURLPayload_nonSunbaseKeepsRawDatetime(t *testing.T) {
	fid := int64(7)
	lead := &leads.Lead{
		CustomValues: map[string]json.RawMessage{
			"7": json.RawMessage(`"2026-06-08T14:30:00-04:00"`),
		},
	}
	entries := []OutboundFieldMapEntry{
		{DestKey: "appt_time", SourceType: "custom", CustomFieldID: &fid},
	}
	payload, err := buildURLPayload("lead.update", lead, PipelineContext{}, entries, nil, "")
	if err != nil {
		t.Fatal(err)
	}
	var out map[string]string
	if err := json.Unmarshal(payload, &out); err != nil {
		t.Fatal(err)
	}
	if out["appt_time"] != "2026-06-08T14:30:00-04:00" {
		t.Fatalf("appt_time = %q, want raw RFC3339", out["appt_time"])
	}
}

func TestSyncOutboundSecretValue_signDisabled(t *testing.T) {
	got, err := syncOutboundSecretValue(false, "existing-secret")
	if err != nil {
		t.Fatal(err)
	}
	if got != "" {
		t.Fatalf("expected empty secret, got %q", got)
	}
}

func TestSyncOutboundSecretValue_keepsExisting(t *testing.T) {
	got, err := syncOutboundSecretValue(true, "existing-secret")
	if err != nil {
		t.Fatal(err)
	}
	if got != "existing-secret" {
		t.Fatalf("expected existing secret preserved, got %q", got)
	}
}

func TestSyncOutboundSecretValue_generatesWhenEnabled(t *testing.T) {
	got, err := syncOutboundSecretValue(true, "")
	if err != nil {
		t.Fatal(err)
	}
	if got == "" {
		t.Fatal("expected generated secret")
	}
	if len(got) != 64 {
		t.Fatalf("expected 64-char hex secret, got len %d", len(got))
	}
}

func TestSyncOutboundConnection_upsert(t *testing.T) {
	ctx := context.Background()
	pool, err := database.Connect(ctx, "postgres://crm:crm@localhost:5432/crm?sslmode=disable")
	if err != nil {
		t.Skip(err)
	}
	defer pool.Close()

	key, err := hex.DecodeString("0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef")
	if err != nil {
		t.Fatal(err)
	}
	svc := NewService(pool, nil, nil, key, nil)

	var accountID int64
	if err := pool.QueryRow(ctx, `SELECT id FROM accounts ORDER BY id LIMIT 1`).Scan(&accountID); err != nil {
		t.Skip(err)
	}

	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	falseVal := false
	trueVal := true
	url := "https://example.com/outbound"
	wh, _, err := svc.Create(ctx, accountID, CreateWebhookInput{
		Name:                "Outbound sync test " + suffix,
		Slug:                "outbound-sync-test-" + suffix,
		InboundEnabled:      &falseVal,
		OutboundEnabled:     &trueVal,
		OutboundSignEnabled: &falseVal,
		OutboundURL:         &url,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = svc.Delete(ctx, accountID, wh.ID) }()

	format := "url"
	method := "POST"
	fieldMap := defaultSunbaseOutboundFieldMapJSON("testschema")
	emptyTemplate := ""
	if _, err := svc.Update(ctx, accountID, wh.ID, UpdateWebhookInput{
		OutboundFormat:          &format,
		OutboundMethod:          &method,
		OutboundFieldMap:        fieldMap,
		OutboundPayloadTemplate: &emptyTemplate,
	}); err != nil {
		t.Fatalf("Update outbound webhook: %v", err)
	}

	updated, err := svc.getWebhook(ctx, accountID, wh.ID)
	if err != nil {
		t.Fatal(err)
	}
	if updated.OutboundConnectionID == nil || *updated.OutboundConnectionID == 0 {
		t.Fatal("expected outbound_connection_id to be set")
	}

	var connCount int
	if err := pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM integration_connections WHERE id = $1`,
		*updated.OutboundConnectionID,
	).Scan(&connCount); err != nil {
		t.Fatal(err)
	}
	if connCount != 1 {
		t.Fatalf("expected hidden integration connection, got count %d", connCount)
	}
}
