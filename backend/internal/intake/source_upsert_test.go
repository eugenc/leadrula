package intake

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/echayko/leadrula/backend/internal/config"
	"github.com/echayko/leadrula/backend/internal/database"
	"github.com/echayko/leadrula/backend/internal/leads"
	"github.com/echayko/leadrula/backend/internal/routing"
)

func TestExtractPhoneFromPayload(t *testing.T) {
	maps := []routing.SourceFieldMapEntry{
		{SourceKey: "caller_phone", TargetType: "builtin", BuiltinField: strPtr("phone")},
	}

	if got := extractPhoneFromPayload(map[string]any{"phone": "+15551234567"}, nil); got != "+15551234567" {
		t.Fatalf("direct phone = %q", got)
	}
	if got := extractPhoneFromPayload(map[string]any{"caller_phone": "5559876543"}, maps); got != "5559876543" {
		t.Fatalf("mapped phone = %q", got)
	}
	if got := extractPhoneFromPayload(map[string]any{"first_name": "Jane"}, maps); got != "" {
		t.Fatalf("missing phone = %q", got)
	}
}

func TestIngestFromSource_upsertByPhone(t *testing.T) {
	cfg := config.Load()
	ctx := context.Background()
	pool, err := database.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		t.Skipf("connect: %v", err)
	}
	t.Cleanup(func() { pool.Close() })

	routeSvc := routing.NewService(pool)
	leadRepo := leads.NewRepository(pool)
	svc := &Service{pool: pool, leads: leadRepo}

	var publisherID int64
	if err := pool.QueryRow(ctx,
		`SELECT id FROM accounts WHERE account_type = 'publisher' ORDER BY id LIMIT 1`).Scan(&publisherID); err != nil {
		t.Skip("no publisher account")
	}

	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	slug := "upsert-test-" + suffix
	src, err := routeSvc.CreateSource(ctx, publisherID, "Upsert test "+suffix, slug, "webhook", nil, nil)
	if err != nil {
		t.Fatalf("CreateSource: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM routing_sources WHERE id=$1`, src.ID)
	})

	recordingFieldID := insertTestCustomField(t, pool, publisherID, "call_recording_"+suffix)
	if _, err := routeSvc.AddSourceFieldMap(ctx, publisherID, src.ID, "recording_url", "custom", nil, &recordingFieldID); err != nil {
		t.Fatalf("map recording: %v", err)
	}

	phoneDigits := "555" + suffix[len(suffix)-7:]
	phoneStored := "+1 (" + phoneDigits[0:3] + ") " + phoneDigits[3:6] + "-" + phoneDigits[6:10]
	rawCreate, _ := json.Marshal(map[string]any{
		"first_name": "Upsert",
		"last_name":  "Test",
		"phone":      phoneStored,
	})
	leadID, publicID, err := leadRepo.InsertLead(ctx, pool, publisherID, publisherID, slug, rawCreate)
	if err != nil {
		t.Fatalf("InsertLead: %v", err)
	}
	if err := leadRepo.SetBuiltinField(ctx, pool, leadID, "phone", phoneStored); err != nil {
		t.Fatalf("SetBuiltinField phone: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM leads WHERE id=$1`, leadID)
	})

	recordingURL := "https://example.com/recording-" + suffix + ".mp3"
	res, err := svc.IngestFromSource(ctx, publisherID, slug, map[string]any{
		"phone":         phoneDigits,
		"recording_url": recordingURL,
	})
	if err != nil {
		t.Fatalf("IngestFromSource update: %v", err)
	}
	if res.Status != "updated" {
		t.Fatalf("status = %q want updated", res.Status)
	}
	if res.LeadID != publicID {
		t.Fatalf("lead_id = %q want %q", res.LeadID, publicID)
	}

	var count int
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM leads WHERE owner_account_id=$1 AND phone IS NOT NULL AND regexp_replace(phone, '[^0-9]', '', 'g') = regexp_replace($2, '[^0-9]', '', 'g')`,
		publisherID, phoneStored).Scan(&count); err != nil {
		t.Fatalf("count leads: %v", err)
	}
	if count != 1 {
		t.Fatalf("lead count = %d want 1", count)
	}

	lead, err := leadRepo.GetByID(ctx, pool, leadID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if err := leads.LoadCustomValues(ctx, pool, lead); err != nil {
		t.Fatalf("LoadCustomValues: %v", err)
	}
	key := fmt.Sprintf("%d", recordingFieldID)
	val, ok := lead.CustomValues[key]
	if !ok {
		t.Fatal("recording custom value missing")
	}
	var gotURL string
	if err := json.Unmarshal(val, &gotURL); err != nil || gotURL != recordingURL {
		t.Fatalf("recording value = %q want %q", gotURL, recordingURL)
	}
}

func TestIngestFromSource_createWhenPhoneUnknown(t *testing.T) {
	cfg := config.Load()
	ctx := context.Background()
	pool, err := database.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		t.Skipf("connect: %v", err)
	}
	t.Cleanup(func() { pool.Close() })

	routeSvc := routing.NewService(pool)
	svc := &Service{pool: pool, leads: leads.NewRepository(pool)}

	var publisherID int64
	if err := pool.QueryRow(ctx,
		`SELECT id FROM accounts WHERE account_type = 'publisher' ORDER BY id LIMIT 1`).Scan(&publisherID); err != nil {
		t.Skip("no publisher account")
	}

	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	slug := "upsert-create-" + suffix
	src, err := routeSvc.CreateSource(ctx, publisherID, "Upsert create "+suffix, slug, "webhook", nil, nil)
	if err != nil {
		t.Fatalf("CreateSource: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM routing_sources WHERE id=$1`, src.ID)
	})

	phone := "555000" + suffix[len(suffix)-4:]
	res, err := svc.IngestFromSource(ctx, publisherID, slug, map[string]any{
		"first_name": "New",
		"last_name":  "Lead",
		"phone":      phone,
	})
	if err != nil {
		t.Fatalf("IngestFromSource create: %v", err)
	}
	if res.Status == "updated" {
		t.Fatalf("expected create path, got updated")
	}
	if res.LeadID == "" {
		t.Fatal("expected lead_id")
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM leads WHERE public_id=$1`, res.LeadID)
	})
}

func insertTestCustomField(t *testing.T, pool database.Querier, publisherID int64, fieldKey string) int64 {
	t.Helper()
	ctx := context.Background()
	var id int64
	if err := pool.QueryRow(ctx,
		`INSERT INTO custom_fields(account_id, name, field_key, type)
		 VALUES ($1, $2, $3, 'text') RETURNING id`,
		publisherID, fieldKey, fieldKey).Scan(&id); err != nil {
		t.Fatalf("insert custom field: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM custom_fields WHERE id=$1`, id)
	})
	return id
}
