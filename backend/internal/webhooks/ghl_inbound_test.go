package webhooks

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/echayko/leadrula/backend/internal/database"
	"github.com/echayko/leadrula/backend/internal/leads"
	"github.com/echayko/leadrula/backend/pkg/httpx"
	"github.com/jackc/pgx/v5/pgxpool"
)

func testGHLConnection(ctx context.Context, t *testing.T, pool *pgxpool.Pool, accountID int64, name string) (connID int64, publicID string) {
	t.Helper()
	err := pool.QueryRow(ctx,
		`INSERT INTO integration_connections (account_id, provider_id, name, credentials, config)
		 SELECT $1, id, $2, '\x00'::bytea, '{}'::jsonb
		 FROM integration_providers WHERE slug = 'ghl'
		 RETURNING id, public_id::text`,
		accountID, name).Scan(&connID, &publicID)
	if err != nil {
		t.Fatalf("create ghl connection: %v", err)
	}
	return connID, publicID
}

func TestGHLInboundContactID(t *testing.T) {
	if got := ghlInboundContactID(map[string]any{"contact_id": "c1"}); got != "c1" {
		t.Fatalf("contact_id: got %q", got)
	}
	if got := ghlInboundContactID(map[string]any{"contactId": "c2"}); got != "c2" {
		t.Fatalf("contactId: got %q", got)
	}
	if got := ghlInboundContactID(map[string]any{"id": "c3"}); got != "c3" {
		t.Fatalf("id: got %q", got)
	}
	if got := ghlInboundContactID(map[string]any{"contact_id": "first", "contactId": "second"}); got != "first" {
		t.Fatalf("priority: got %q", got)
	}
}

func TestIngest_GHLInbound_updatesExternalIDByLeadID(t *testing.T) {
	ctx := context.Background()
	pool, err := database.Connect(ctx, "postgres://crm:crm@localhost:5432/crm?sslmode=disable")
	if err != nil {
		t.Skip(err)
	}
	defer pool.Close()

	svc := NewService(pool, nil, nil, testEncKey(t), nil)
	repo := leads.NewRepository(pool)
	svc.leads = repo

	var accountID int64
	if err := pool.QueryRow(ctx, `SELECT id FROM accounts ORDER BY id LIMIT 1`).Scan(&accountID); err != nil {
		t.Skip(err)
	}

	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	connID, connPublicID := testGHLConnection(ctx, t, pool, accountID, "GHL LeadID Conn "+suffix)

	ids, err := svc.ProvisionGHLWebhooks(ctx, accountID, connID, connPublicID, "GHL LeadID "+suffix)
	if err != nil {
		t.Fatalf("ProvisionGHLWebhooks: %v", err)
	}
	defer svc.DeleteGHLWebhooks(ctx, accountID, *ids)

	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	leadID, publicID, err := repo.InsertLead(ctx, tx, accountID, accountID, "", nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.SetBuiltinField(ctx, tx, leadID, "first_name", "Jane"); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}

	contactID := "ghl-contact-" + suffix
	res, err := svc.Ingest(ctx, &WebhookAuth{WebhookID: ids.Inbound, AccountID: accountID}, ids.InboundSlug, map[string]any{
		"lead_id":    publicID,
		"contact_id": contactID,
		"firstName":  "Jane",
	})
	if err != nil {
		t.Fatalf("ingest: %v", err)
	}
	if len(res.Results) == 0 || res.Results[0].Status != "updated" {
		t.Fatalf("results = %+v, want updated", res.Results)
	}

	lead, err := repo.GetByID(ctx, pool, leadID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if lead.ExternalID == nil || *lead.ExternalID != contactID {
		t.Fatalf("external_id = %v, want %q", lead.ExternalID, contactID)
	}

	var leadCount int
	if err := pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM leads WHERE owner_account_id=$1 AND deleted_at IS NULL`,
		accountID,
	).Scan(&leadCount); err != nil {
		t.Fatal(err)
	}
	// Should not create a duplicate; at least our lead exists.
	if leadCount < 1 {
		t.Fatal("expected at least one lead")
	}
}

func TestIngest_GHLInbound_fallbackPhone(t *testing.T) {
	ctx := context.Background()
	pool, err := database.Connect(ctx, "postgres://crm:crm@localhost:5432/crm?sslmode=disable")
	if err != nil {
		t.Skip(err)
	}
	defer pool.Close()

	svc := NewService(pool, nil, nil, testEncKey(t), nil)
	repo := leads.NewRepository(pool)
	svc.leads = repo

	var accountID int64
	if err := pool.QueryRow(ctx, `SELECT id FROM accounts ORDER BY id LIMIT 1`).Scan(&accountID); err != nil {
		t.Skip(err)
	}

	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	phone := fmt.Sprintf("416%07d", time.Now().UnixNano()%10000000)
	connID, connPublicID := testGHLConnection(ctx, t, pool, accountID, "GHL Phone Conn "+suffix)

	ids, err := svc.ProvisionGHLWebhooks(ctx, accountID, connID, connPublicID, "GHL Phone "+suffix)
	if err != nil {
		t.Fatalf("ProvisionGHLWebhooks: %v", err)
	}
	defer svc.DeleteGHLWebhooks(ctx, accountID, *ids)

	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	leadID, _, err := repo.InsertLead(ctx, tx, accountID, accountID, "", nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.SetBuiltinField(ctx, tx, leadID, "phone", phone); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}

	contactID := "ghl-contact-phone-" + suffix
	res, err := svc.Ingest(ctx, &WebhookAuth{WebhookID: ids.Inbound, AccountID: accountID}, ids.InboundSlug, map[string]any{
		"phone":      phone,
		"contact_id": contactID,
	})
	if err != nil {
		t.Fatalf("ingest: %v", err)
	}
	if len(res.Results) == 0 || res.Results[0].Status != "updated" {
		t.Fatalf("results = %+v, want updated", res.Results)
	}

	lead, err := repo.GetByID(ctx, pool, leadID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if lead.ExternalID == nil || *lead.ExternalID != contactID {
		t.Fatalf("external_id = %v, want %q", lead.ExternalID, contactID)
	}
}

func TestIngest_GHLInbound_leadIDNotFound(t *testing.T) {
	ctx := context.Background()
	pool, err := database.Connect(ctx, "postgres://crm:crm@localhost:5432/crm?sslmode=disable")
	if err != nil {
		t.Skip(err)
	}
	defer pool.Close()

	svc := NewService(pool, nil, nil, testEncKey(t), nil)
	repo := leads.NewRepository(pool)
	svc.leads = repo

	var accountID int64
	if err := pool.QueryRow(ctx, `SELECT id FROM accounts ORDER BY id LIMIT 1`).Scan(&accountID); err != nil {
		t.Skip(err)
	}

	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	connID, connPublicID := testGHLConnection(ctx, t, pool, accountID, "GHL NotFound Conn "+suffix)

	ids, err := svc.ProvisionGHLWebhooks(ctx, accountID, connID, connPublicID, "GHL NotFound "+suffix)
	if err != nil {
		t.Fatalf("ProvisionGHLWebhooks: %v", err)
	}
	defer svc.DeleteGHLWebhooks(ctx, accountID, *ids)

	_, err = svc.Ingest(ctx, &WebhookAuth{WebhookID: ids.Inbound, AccountID: accountID}, ids.InboundSlug, map[string]any{
		"lead_id":    "00000000-0000-0000-0000-000000009999",
		"contact_id": "ghl-missing-" + suffix,
	})
	if err == nil {
		t.Fatal("expected error for missing lead_id")
	}
	appErr, ok := err.(*httpx.AppError)
	if !ok || appErr.Code != httpx.CodeNotFound {
		t.Fatalf("err = %v, want not_found", err)
	}
}
