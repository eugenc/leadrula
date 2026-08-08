package intake

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/echayko/leadrula/backend/internal/config"
	"github.com/echayko/leadrula/backend/internal/auth"
	"github.com/echayko/leadrula/backend/internal/database"
	"github.com/echayko/leadrula/backend/internal/leads"
	"github.com/echayko/leadrula/backend/internal/routing"
	"github.com/go-chi/chi/v5"
)

func TestIngest_upsertByExternalID(t *testing.T) {
	cfg := config.Load()
	ctx := context.Background()
	pool, err := database.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		t.Skipf("connect: %v", err)
	}
	t.Cleanup(func() { pool.Close() })

	leadRepo := leads.NewRepository(pool)
	svc := &Service{pool: pool, leads: leadRepo}

	var publisherID int64
	if err := pool.QueryRow(ctx,
		`SELECT id FROM accounts WHERE account_type = 'publisher' ORDER BY id LIMIT 1`).Scan(&publisherID); err != nil {
		t.Skip("no publisher account")
	}

	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	extID := "ext-upsert-" + suffix
	phone := "554000" + suffix[len(suffix)-4:]

	res1, err := svc.Ingest(ctx, publisherID, map[string]any{
		"external_id": extID,
		"first_name":  "External",
		"last_name":   "Create",
		"phone":       phone,
	})
	if err != nil {
		t.Fatalf("Ingest create: %v", err)
	}
	if res1.Status == "updated" {
		t.Fatal("expected create on first inbound")
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM leads WHERE public_id=$1`, res1.LeadID)
	})

	res2, err := svc.Ingest(ctx, publisherID, map[string]any{
		"external_id": extID,
		"first_name":  "ExternalUpdated",
	})
	if err != nil {
		t.Fatalf("Ingest update: %v", err)
	}
	if res2.Status != "updated" {
		t.Fatalf("status = %q want updated", res2.Status)
	}
	if res2.LeadID != res1.LeadID {
		t.Fatalf("lead_id = %q want %q", res2.LeadID, res1.LeadID)
	}

	var firstName string
	if err := pool.QueryRow(ctx, `SELECT first_name FROM leads WHERE public_id=$1`, res1.LeadID).Scan(&firstName); err != nil {
		t.Fatalf("query lead: %v", err)
	}
	if firstName != "ExternalUpdated" {
		t.Fatalf("first_name = %q want ExternalUpdated", firstName)
	}
}

func TestIngestFromSource_upsertByExternalID(t *testing.T) {
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
	slug := "ext-source-" + suffix
	src, err := routeSvc.CreateSource(ctx, publisherID, "External ID "+suffix, slug, "webhook", nil, nil, nil)
	if err != nil {
		t.Fatalf("CreateSource: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM routing_sources WHERE id=$1`, src.ID)
	})

	extID := "ext-src-" + suffix
	phone := "553000" + suffix[len(suffix)-4:]

	res1, err := svc.IngestFromSource(ctx, publisherID, slug, map[string]any{
		"external_id": extID,
		"first_name":  "SourceExt",
		"phone":       phone,
	})
	if err != nil {
		t.Fatalf("IngestFromSource create: %v", err)
	}
	if res1.Status == "updated" {
		t.Fatal("expected create on first inbound")
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM leads WHERE public_id=$1`, res1.LeadID)
	})

	res2, err := svc.IngestFromSource(ctx, publisherID, slug, map[string]any{
		"external_id": extID,
		"first_name":  "SourceExtUpdated",
	})
	if err != nil {
		t.Fatalf("IngestFromSource update: %v", err)
	}
	if res2.Status != "updated" {
		t.Fatalf("status = %q want updated", res2.Status)
	}
	if res2.LeadID != res1.LeadID {
		t.Fatalf("lead_id = %q want %q", res2.LeadID, res1.LeadID)
	}
}

func TestListPublicSources(t *testing.T) {
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
	slug := "public-list-" + suffix
	src, err := routeSvc.CreateSource(ctx, publisherID, "Public list "+suffix, slug, "webhook", nil, nil, nil)
	if err != nil {
		t.Fatalf("CreateSource: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM routing_sources WHERE id=$1`, src.ID)
	})

	items, err := svc.ListPublicSources(ctx, publisherID, "https://api.example.com")
	if err != nil {
		t.Fatalf("ListPublicSources: %v", err)
	}
	found := false
	for _, item := range items {
		if item.Slug == slug {
			found = true
			if item.Name != "Public list "+suffix {
				t.Fatalf("name = %q", item.Name)
			}
			if item.Type != "webhook" {
				t.Fatalf("type = %q want webhook", item.Type)
			}
			wantURL := "https://api.example.com/api/v1/sources/" + slug
			if item.IngestURL != wantURL {
				t.Fatalf("ingest_url = %q want %q", item.IngestURL, wantURL)
			}
		}
	}
	if !found {
		t.Fatalf("source %q not found in list", slug)
	}
}

func TestHandlerIngest_withMatchingSourceUsesSourceIngest(t *testing.T) {
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
	h := NewHandler(svc, nil, "https://api.example.com")

	var publisherID int64
	if err := pool.QueryRow(ctx,
		`SELECT id FROM accounts WHERE account_type = 'publisher' ORDER BY id LIMIT 1`).Scan(&publisherID); err != nil {
		t.Skip("no publisher account")
	}

	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	slug := "handler-source-" + suffix
	noteField := "note"
	src, err := routeSvc.CreateSource(ctx, publisherID, "Handler source "+suffix, slug, "webhook", nil, nil, nil)
	if err != nil {
		t.Fatalf("CreateSource: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM routing_sources WHERE id=$1`, src.ID)
	})
	if _, err := routeSvc.AddSourceFieldMap(ctx, publisherID, src.ID, "comments", "builtin", &noteField, nil); err != nil {
		t.Fatalf("AddSourceFieldMap: %v", err)
	}

	phone := "552100" + suffix[len(suffix)-4:]
	body, _ := json.Marshal(map[string]any{
		"source":     slug,
		"first_name": "Handler",
		"last_name":  "Route",
		"phone":      phone,
		"comments":   "via handler source slug",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/leads", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(withTestPublisher(ctx, publisherID))

	w := httptest.NewRecorder()
	r := chi.NewRouter()
	r.Post("/api/v1/leads", h.ingest)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusAccepted && w.Code != http.StatusOK {
		t.Fatalf("status = %d body = %s", w.Code, w.Body.String())
	}

	var resp struct {
		Data IngestResult `json:"data"`
	}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v body=%s", err, w.Body.String())
	}
	if resp.Data.LeadID == "" {
		t.Fatal("expected lead_id")
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM leads WHERE public_id=$1`, resp.Data.LeadID)
	})

	var noteCount int
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM lead_notes WHERE lead_id=(SELECT id FROM leads WHERE public_id=$1)`, resp.Data.LeadID).Scan(&noteCount); err != nil {
		t.Fatalf("note count: %v", err)
	}
	if noteCount != 1 {
		t.Fatalf("note count = %d want 1 (proves source ingest path ran)", noteCount)
	}
}

func TestHandlerIngest_unknownSourceFallsBackToQueue(t *testing.T) {
	cfg := config.Load()
	ctx := context.Background()
	pool, err := database.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		t.Skipf("connect: %v", err)
	}
	t.Cleanup(func() { pool.Close() })

	svc := &Service{pool: pool, leads: leads.NewRepository(pool)}
	h := NewHandler(svc, nil, "https://api.example.com")

	var publisherID int64
	if err := pool.QueryRow(ctx,
		`SELECT id FROM accounts WHERE account_type = 'publisher' ORDER BY id LIMIT 1`).Scan(&publisherID); err != nil {
		t.Skip("no publisher account")
	}

	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	body, _ := json.Marshal(map[string]any{
		"source":     "nonexistent-source-" + suffix,
		"first_name": "Queue",
		"phone":      "551100" + suffix[len(suffix)-4:],
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/leads", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(withTestPublisher(ctx, publisherID))

	w := httptest.NewRecorder()
	r := chi.NewRouter()
	r.Post("/api/v1/leads", h.ingest)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusAccepted {
		t.Fatalf("status = %d body = %s", w.Code, w.Body.String())
	}

	var resp struct {
		Data IngestResult `json:"data"`
	}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Data.Status != "review" {
		t.Fatalf("status = %q want review", resp.Data.Status)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM leads WHERE public_id=$1`, resp.Data.LeadID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM lead_intake_queue WHERE lead_id=(SELECT id FROM leads WHERE public_id=$1)`, resp.Data.LeadID)
	})

	var queueCount int
	if err := pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM lead_intake_queue WHERE lead_id=(SELECT id FROM leads WHERE public_id=$1)`,
		resp.Data.LeadID).Scan(&queueCount); err != nil {
		t.Fatalf("queue count: %v", err)
	}
	if queueCount != 1 {
		t.Fatalf("queue count = %d want 1", queueCount)
	}
}

func withTestPublisher(ctx context.Context, publisherID int64) context.Context {
	return auth.WithAPIKeyAccount(ctx, &auth.APIKeyAccount{
		AccountID:   publisherID,
		AccountType: "publisher",
		Scopes:      []string{"leads:read", "leads:write"},
	})
}
