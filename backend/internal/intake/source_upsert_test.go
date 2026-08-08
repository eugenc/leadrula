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
	if got := extractPhoneFromPayload(map[string]any{"mobile": "5551112222"}, nil); got != "5551112222" {
		t.Fatalf("fallback mobile = %q", got)
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
	src, err := routeSvc.CreateSource(ctx, publisherID, "Upsert test "+suffix, slug, "webhook", nil, nil, nil)
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

func TestIngestFromSource_upsertByPhone_afterBuyerTransfer(t *testing.T) {
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
	var buyerID int64
	if err := pool.QueryRow(ctx,
		`SELECT id FROM accounts WHERE account_type = 'buyer' AND id <> $1 LIMIT 1`, publisherID).Scan(&buyerID); err != nil {
		t.Skip("no buyer account")
	}

	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	slug := "upsert-buyer-" + suffix
	src, err := routeSvc.CreateSource(ctx, publisherID, "Upsert buyer "+suffix, slug, "webhook", nil, nil, nil)
	if err != nil {
		t.Fatalf("CreateSource: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM routing_sources WHERE id=$1`, src.ID)
	})

	phoneDigits := "556" + suffix[len(suffix)-7:]
	phoneStored := "+1 (" + phoneDigits[0:3] + ") " + phoneDigits[3:6] + "-" + phoneDigits[6:10]
	rawCreate, _ := json.Marshal(map[string]any{
		"first_name": "Routed",
		"last_name":  "Lead",
		"phone":      phoneStored,
	})
	leadID, publicID, err := leadRepo.InsertLead(ctx, pool, publisherID, publisherID, slug, rawCreate)
	if err != nil {
		t.Fatalf("InsertLead: %v", err)
	}
	if err := leadRepo.SetBuiltinField(ctx, pool, leadID, "phone", phoneStored); err != nil {
		t.Fatalf("SetBuiltinField phone: %v", err)
	}
	if err := leadRepo.TransferOwner(ctx, pool, leadID, buyerID, nil); err != nil {
		t.Fatalf("TransferOwner: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM leads WHERE id=$1`, leadID)
	})

	res, err := svc.IngestFromSource(ctx, publisherID, slug, map[string]any{
		"phone":      phoneDigits,
		"first_name": "Updated",
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
	if err := pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM leads WHERE publisher_id=$1 AND phone IS NOT NULL
		 AND regexp_replace(phone, '[^0-9]', '', 'g') = regexp_replace($2, '[^0-9]', '', 'g')`,
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
	if lead.FirstName != "Updated" {
		t.Fatalf("first_name = %q want Updated", lead.FirstName)
	}
	if lead.OwnerAccountID != buyerID {
		t.Fatalf("owner_account_id = %d want buyer %d", lead.OwnerAccountID, buyerID)
	}
}

func TestIngestFromSource_upsertByFallbackPhoneKey(t *testing.T) {
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
	slug := "upsert-fallback-" + suffix
	src, err := routeSvc.CreateSource(ctx, publisherID, "Upsert fallback "+suffix, slug, "webhook", nil, nil, nil)
	if err != nil {
		t.Fatalf("CreateSource: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM routing_sources WHERE id=$1`, src.ID)
	})

	phone := "558000" + suffix[len(suffix)-4:]
	res, err := svc.IngestFromSource(ctx, publisherID, slug, map[string]any{
		"first_name": "Fallback",
		"last_name":  "Create",
		"phone":      phone,
	})
	if err != nil {
		t.Fatalf("IngestFromSource create: %v", err)
	}
	if res.Status == "updated" {
		t.Fatal("expected create on first inbound")
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM leads WHERE public_id=$1`, res.LeadID)
	})

	res2, err := svc.IngestFromSource(ctx, publisherID, slug, map[string]any{
		"mobile":     phone,
		"first_name": "FallbackUpdated",
	})
	if err != nil {
		t.Fatalf("IngestFromSource update via mobile: %v", err)
	}
	if res2.Status != "updated" {
		t.Fatalf("status = %q want updated", res2.Status)
	}
	if res2.LeadID != res.LeadID {
		t.Fatalf("lead_id = %q want %q", res2.LeadID, res.LeadID)
	}

	var count int
	if err := pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM leads WHERE publisher_id=$1 AND deleted_at IS NULL
		 AND phone IS NOT NULL AND regexp_replace(phone, '[^0-9]', '', 'g') = regexp_replace($2, '[^0-9]', '', 'g')`,
		publisherID, phone).Scan(&count); err != nil {
		t.Fatalf("count leads: %v", err)
	}
	if count != 1 {
		t.Fatalf("lead count = %d want 1", count)
	}
}

func TestIngestFromSource_upsertMappedPhoneThenDifferentKey(t *testing.T) {
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
	slug := "upsert-mapped-" + suffix
	src, err := routeSvc.CreateSource(ctx, publisherID, "Upsert mapped "+suffix, slug, "webhook", nil, nil, nil)
	if err != nil {
		t.Fatalf("CreateSource: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM routing_sources WHERE id=$1`, src.ID)
	})

	if _, err := routeSvc.AddSourceFieldMap(ctx, publisherID, src.ID, "caller_phone", "builtin", strPtr("phone"), nil); err != nil {
		t.Fatalf("AddSourceFieldMap phone: %v", err)
	}

	phone := "559000" + suffix[len(suffix)-4:]
	res, err := svc.IngestFromSource(ctx, publisherID, slug, map[string]any{
		"first_name":   "Mapped",
		"last_name":    "Create",
		"caller_phone": phone,
	})
	if err != nil {
		t.Fatalf("IngestFromSource create: %v", err)
	}
	if res.Status == "updated" {
		t.Fatal("expected create on first inbound")
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM leads WHERE public_id=$1`, res.LeadID)
	})

	res2, err := svc.IngestFromSource(ctx, publisherID, slug, map[string]any{
		"phone_number": phone,
		"first_name":   "MappedUpdated",
	})
	if err != nil {
		t.Fatalf("IngestFromSource update via phone_number: %v", err)
	}
	if res2.Status != "updated" {
		t.Fatalf("status = %q want updated", res2.Status)
	}
	if res2.LeadID != res.LeadID {
		t.Fatalf("lead_id = %q want %q", res2.LeadID, res.LeadID)
	}

	var count int
	if err := pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM leads WHERE publisher_id=$1 AND deleted_at IS NULL
		 AND phone IS NOT NULL AND regexp_replace(phone, '[^0-9]', '', 'g') = regexp_replace($2, '[^0-9]', '', 'g')`,
		publisherID, phone).Scan(&count); err != nil {
		t.Fatalf("count leads: %v", err)
	}
	if count != 1 {
		t.Fatalf("lead count = %d want 1", count)
	}

	var firstName string
	if err := pool.QueryRow(ctx, `SELECT first_name FROM leads WHERE public_id=$1`, res.LeadID).Scan(&firstName); err != nil {
		t.Fatalf("query lead: %v", err)
	}
	if firstName != "MappedUpdated" {
		t.Fatalf("first_name = %q want MappedUpdated", firstName)
	}
}

func TestIngestFromSource_upsertExistingViaMappedContactPhone(t *testing.T) {
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
	slug := "upsert-mapped-existing-" + suffix
	src, err := routeSvc.CreateSource(ctx, publisherID, "Upsert mapped existing "+suffix, slug, "webhook", nil, nil, nil)
	if err != nil {
		t.Fatalf("CreateSource: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM routing_sources WHERE id=$1`, src.ID)
	})

	phoneDigits := "551" + suffix[len(suffix)-7:]
	phoneStored := "+1 (" + phoneDigits[0:3] + ") " + phoneDigits[3:6] + "-" + phoneDigits[6:10]

	existingID, existingPublicID, err := leadRepo.InsertLead(ctx, pool, publisherID, publisherID, slug, []byte(`{}`))
	if err != nil {
		t.Fatalf("InsertLead existing: %v", err)
	}
	if err := leadRepo.SetBuiltinField(ctx, pool, existingID, "phone", phoneStored); err != nil {
		t.Fatalf("SetBuiltinField phone: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM leads WHERE id=$1`, existingID)
	})

	// Phone arrives via mapped contact_phone key against a lead that already has that phone stored.
	if _, err := routeSvc.AddSourceFieldMap(ctx, publisherID, src.ID, "contact_phone", "builtin", strPtr("phone"), nil); err != nil {
		t.Fatalf("AddSourceFieldMap phone: %v", err)
	}

	res, err := svc.IngestFromSource(ctx, publisherID, slug, map[string]any{
		"first_name":    "Safety",
		"contact_phone": phoneDigits,
	})
	if err != nil {
		t.Fatalf("IngestFromSource: %v", err)
	}
	if res.Status != "updated" {
		t.Fatalf("status = %q want updated", res.Status)
	}
	if res.LeadID != existingPublicID {
		t.Fatalf("lead_id = %q want %q", res.LeadID, existingPublicID)
	}

	var activeCount int
	if err := pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM leads WHERE publisher_id=$1 AND deleted_at IS NULL
		 AND phone IS NOT NULL AND regexp_replace(phone, '[^0-9]', '', 'g') = regexp_replace($2, '[^0-9]', '', 'g')`,
		publisherID, phoneStored).Scan(&activeCount); err != nil {
		t.Fatalf("count active leads: %v", err)
	}
	if activeCount != 1 {
		t.Fatalf("active lead count = %d want 1", activeCount)
	}
}

func TestIngestFromSource_repeatInboundNo409(t *testing.T) {
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
	slug := "upsert-no409-" + suffix
	src, err := routeSvc.CreateSource(ctx, publisherID, "Upsert no409 "+suffix, slug, "webhook", nil, nil, nil)
	if err != nil {
		t.Fatalf("CreateSource: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM routing_sources WHERE id=$1`, src.ID)
	})

	phone := "552000" + suffix[len(suffix)-4:]
	payload := map[string]any{
		"first_name": "No409",
		"last_name":  "Test",
		"phone":      phone,
	}

	res1, err := svc.IngestFromSource(ctx, publisherID, slug, payload)
	if err != nil {
		t.Fatalf("IngestFromSource first: %v", err)
	}
	if res1.Status == "updated" {
		t.Fatal("expected create on first inbound")
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM leads WHERE public_id=$1`, res1.LeadID)
	})

	payload["first_name"] = "No409Updated"
	res2, err := svc.IngestFromSource(ctx, publisherID, slug, payload)
	if err != nil {
		t.Fatalf("IngestFromSource repeat: %v", err)
	}
	if res2.Status != "updated" {
		t.Fatalf("status = %q want updated", res2.Status)
	}
	if res2.LeadID != res1.LeadID {
		t.Fatalf("lead_id = %q want %q", res2.LeadID, res1.LeadID)
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
	src, err := routeSvc.CreateSource(ctx, publisherID, "Upsert create "+suffix, slug, "webhook", nil, nil, nil)
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

func TestIngestFromSource_noteMapping(t *testing.T) {
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
	sourceName := "Note map test " + suffix
	slug := "note-map-" + suffix
	src, err := routeSvc.CreateSource(ctx, publisherID, sourceName, slug, "webhook", nil, nil, nil)
	if err != nil {
		t.Fatalf("CreateSource: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM routing_sources WHERE id=$1`, src.ID)
	})

	noteField := "note"
	if _, err := routeSvc.AddSourceFieldMap(ctx, publisherID, src.ID, "comments", "builtin", &noteField, nil); err != nil {
		t.Fatalf("AddSourceFieldMap note: %v", err)
	}

	phone := "557000" + suffix[len(suffix)-4:]
	res, err := svc.IngestFromSource(ctx, publisherID, slug, map[string]any{
		"first_name": "Note",
		"last_name":  "Test",
		"phone":      phone,
		"comments":   "hello from source",
	})
	if err != nil {
		t.Fatalf("IngestFromSource: %v", err)
	}
	if res.LeadID == "" {
		t.Fatal("expected lead_id")
	}

	var leadID int64
	if err := pool.QueryRow(ctx, `SELECT id FROM leads WHERE public_id=$1`, res.LeadID).Scan(&leadID); err != nil {
		t.Fatalf("lookup lead: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM leads WHERE id=$1`, leadID)
	})

	var noteCount int
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM lead_notes WHERE lead_id=$1`, leadID).Scan(&noteCount); err != nil {
		t.Fatal(err)
	}
	if noteCount != 1 {
		t.Fatalf("note count = %d, want 1", noteCount)
	}

	var authorName, body string
	if err := pool.QueryRow(ctx,
		`SELECT COALESCE(author_name, ''), body FROM lead_notes WHERE lead_id=$1 ORDER BY created_at DESC LIMIT 1`,
		leadID,
	).Scan(&authorName, &body); err != nil {
		t.Fatalf("query note: %v", err)
	}
	if authorName != sourceName {
		t.Fatalf("author_name = %q, want %q", authorName, sourceName)
	}
	if body != "hello from source" {
		t.Fatalf("body = %q, want hello from source", body)
	}

	res2, err := svc.IngestFromSource(ctx, publisherID, slug, map[string]any{
		"phone":      phone,
		"first_name": "Note",
	})
	if err != nil {
		t.Fatalf("IngestFromSource update: %v", err)
	}
	if res2.Status != "updated" {
		t.Fatalf("status = %q, want updated", res2.Status)
	}

	var totalNotes int
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM lead_notes WHERE lead_id=$1`, leadID).Scan(&totalNotes); err != nil {
		t.Fatal(err)
	}
	if totalNotes != 1 {
		t.Fatalf("note count after update without comments = %d, want 1", totalNotes)
	}
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
