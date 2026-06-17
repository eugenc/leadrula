package webhooks

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/echayko/leadrula/backend/internal/database"
	"github.com/echayko/leadrula/backend/internal/integrations/providers"
	"github.com/echayko/leadrula/backend/internal/leads"
	"github.com/jackc/pgx/v5/pgxpool"
)

func testRandomSunbaseHexID(t *testing.T) string {
	t.Helper()
	raw := make([]byte, 16)
	if _, err := rand.Read(raw); err != nil {
		t.Fatal(err)
	}
	return hex.EncodeToString(raw)
}

func testSunbaseConnection(ctx context.Context, t *testing.T, pool *pgxpool.Pool, accountID int64, name string) (connID int64, publicID string) {
	t.Helper()
	err := pool.QueryRow(ctx,
		`INSERT INTO integration_connections (account_id, provider_id, name, credentials, config)
		 SELECT $1, id, $2, '\x00'::bytea, '{}'::jsonb
		 FROM integration_providers WHERE slug = 'sunbase'
		 RETURNING id, public_id::text`,
		accountID, name).Scan(&connID, &publicID)
	if err != nil {
		t.Fatalf("create sunbase connection: %v", err)
	}
	return connID, publicID
}

func testEncKey(t *testing.T) []byte {
	t.Helper()
	key, err := hex.DecodeString("0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef")
	if err != nil {
		t.Fatal(err)
	}
	return key
}

func TestProvisionSunbaseWebhooks_createsOutboundConnections(t *testing.T) {
	ctx := context.Background()
	pool, err := database.Connect(ctx, "postgres://crm:crm@localhost:5432/crm?sslmode=disable")
	if err != nil {
		t.Skip(err)
	}
	defer pool.Close()

	svc := NewService(pool, nil, nil, testEncKey(t), nil)

	var accountID int64
	if err := pool.QueryRow(ctx, `SELECT id FROM accounts ORDER BY id LIMIT 1`).Scan(&accountID); err != nil {
		t.Skip(err)
	}

	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	publicID := fmt.Sprintf("00000000-0000-4000-8000-%012s", suffix[len(suffix)-12:])
	fieldMap := defaultSunbaseOutboundFieldMapJSON("sunbrightsolarusa")

	ids, err := svc.ProvisionSunbaseWebhooks(
		ctx, accountID, 0, publicID, "SunBase Test "+suffix,
		"sunbrightsolarusa", sunbaseDefaultEndpoint, fieldMap,
	)
	if err != nil {
		t.Fatalf("ProvisionSunbaseWebhooks: %v", err)
	}
	defer svc.DeleteSunbaseWebhooks(ctx, accountID, *ids)

	if ids.Inbound == 0 || ids.OutboundPost == 0 || ids.OutboundGet == 0 {
		t.Fatalf("expected all webhook ids set, got %+v", ids)
	}
	if ids.InboundSlug == "" {
		t.Fatal("expected inbound slug")
	}

	var outboundConnCount int
	err = pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM integration_connections ic
		 JOIN integration_providers p ON p.id = ic.provider_id
		 WHERE ic.account_id = $1 AND p.slug = 'webhook'
		   AND ic.name IN ($2, $3)`,
		accountID, fmt.Sprintf("webhook:%d", ids.OutboundPost), fmt.Sprintf("webhook:%d", ids.OutboundGet),
	).Scan(&outboundConnCount)
	if err != nil {
		t.Fatal(err)
	}
	if outboundConnCount != 2 {
		t.Fatalf("expected 2 hidden outbound integration connections, got %d", outboundConnCount)
	}

	var postFormat string
	var postFieldMap json.RawMessage
	err = pool.QueryRow(ctx,
		`SELECT outbound_format, outbound_field_map FROM webhooks WHERE id = $1`,
		ids.OutboundPost,
	).Scan(&postFormat, &postFieldMap)
	if err != nil {
		t.Fatal(err)
	}
	if postFormat != "url" {
		t.Fatalf("expected url format, got %q", postFormat)
	}
	if len(postFieldMap) == 0 {
		t.Fatal("expected outbound field map on post webhook")
	}

	var conditions json.RawMessage
	err = pool.QueryRow(ctx,
		`SELECT conditions FROM webhook_events WHERE webhook_id=$1 AND action='create'`,
		ids.Inbound,
	).Scan(&conditions)
	if err != nil {
		t.Fatal(err)
	}
	if string(conditions) != "[]" {
		t.Fatalf("expected empty inbound conditions, got %s", conditions)
	}
}

func TestIngest_sunbaseInbound_updateUpsertsByUUID(t *testing.T) {
	ctx := context.Background()
	pool, err := database.Connect(ctx, "postgres://crm:crm@localhost:5432/crm?sslmode=disable")
	if err != nil {
		t.Skip(err)
	}
	defer pool.Close()

	svc := NewService(pool, nil, nil, testEncKey(t), nil)

	var accountID int64
	if err := pool.QueryRow(ctx, `SELECT id FROM accounts ORDER BY id LIMIT 1`).Scan(&accountID); err != nil {
		t.Skip(err)
	}

	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	publicID := fmt.Sprintf("00000000-0000-4000-8000-%012s", suffix[len(suffix)-12:])
	fieldMap := defaultSunbaseOutboundFieldMapJSON("sunbrightsolarusa")
	extID := "sunbase-inbound-update-" + suffix

	ids, err := svc.ProvisionSunbaseWebhooks(
		ctx, accountID, 0, publicID, "SunBase Ingest "+suffix,
		"sunbrightsolarusa", sunbaseDefaultEndpoint, fieldMap,
	)
	if err != nil {
		t.Fatalf("ProvisionSunbaseWebhooks: %v", err)
	}
	defer svc.DeleteSunbaseWebhooks(ctx, accountID, *ids)

	repo := leads.NewRepository(pool)
	leadSvc := leads.NewService(repo, nil, nil, nil, nil)
	svc.leads = repo
	svc.leadSvc = leadSvc

	createRes, err := svc.Ingest(ctx, &WebhookAuth{WebhookID: ids.Inbound, AccountID: accountID}, ids.InboundSlug, map[string]any{
		"action":     "Create",
		"uuid":       extID,
		"first_name": "Original",
		"last_name":  "Lead",
	})
	if err != nil {
		t.Fatalf("create ingest: %v", err)
	}
	if createRes.Status != "processed" {
		t.Fatalf("create status = %q, want processed", createRes.Status)
	}

	updateRes, err := svc.Ingest(ctx, &WebhookAuth{WebhookID: ids.Inbound, AccountID: accountID}, ids.InboundSlug, map[string]any{
		"action":     "Update",
		"uuid":       extID,
		"first_name": "Updated",
	})
	if err != nil {
		t.Fatalf("update ingest: %v", err)
	}
	if updateRes.Status != "processed" {
		t.Fatalf("update status = %q, want processed", updateRes.Status)
	}
	if len(updateRes.Results) == 0 || updateRes.Results[0].Status != "updated" {
		t.Fatalf("update results = %+v, want updated", updateRes.Results)
	}

	lead, err := repo.GetByExternalID(ctx, pool, accountID, extID)
	if err != nil {
		t.Fatalf("GetByExternalID: %v", err)
	}
	if lead.FirstName != "Updated" {
		t.Fatalf("first_name = %q, want Updated", lead.FirstName)
	}
}

func TestSyncSunbaseInboundEvent_clearsCreateConditions(t *testing.T) {
	ctx := context.Background()
	pool, err := database.Connect(ctx, "postgres://crm:crm@localhost:5432/crm?sslmode=disable")
	if err != nil {
		t.Skip(err)
	}
	defer pool.Close()

	svc := NewService(pool, nil, nil, testEncKey(t), nil)

	var accountID int64
	if err := pool.QueryRow(ctx, `SELECT id FROM accounts ORDER BY id LIMIT 1`).Scan(&accountID); err != nil {
		t.Skip(err)
	}

	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	publicID := fmt.Sprintf("00000000-0000-4000-8000-%012s", suffix[len(suffix)-12:])
	fieldMap := defaultSunbaseOutboundFieldMapJSON("sunbrightsolarusa")

	ids, err := svc.ProvisionSunbaseWebhooks(
		ctx, accountID, 0, publicID, "SunBase Sync "+suffix,
		"sunbrightsolarusa", sunbaseDefaultEndpoint, fieldMap,
	)
	if err != nil {
		t.Fatalf("ProvisionSunbaseWebhooks: %v", err)
	}
	defer svc.DeleteSunbaseWebhooks(ctx, accountID, *ids)

	_, err = pool.Exec(ctx,
		`UPDATE webhook_events SET conditions=$2::jsonb WHERE webhook_id=$1 AND action='create'`,
		ids.Inbound, `[{"field":"action","op":"eq","value":"Create"}]`,
	)
	if err != nil {
		t.Fatal(err)
	}

	if err := svc.SyncSunbaseInboundEvent(ctx, ids.Inbound); err != nil {
		t.Fatal(err)
	}

	var conditions json.RawMessage
	if err := pool.QueryRow(ctx,
		`SELECT conditions FROM webhook_events WHERE webhook_id=$1 AND action='create'`,
		ids.Inbound,
	).Scan(&conditions); err != nil {
		t.Fatal(err)
	}
	if string(conditions) != "[]" {
		t.Fatalf("conditions = %s, want []", conditions)
	}
}

func TestIngest_sunbaseInbound_hexUUIDMatchesCustStoredLead(t *testing.T) {
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
	fieldMap := defaultSunbaseOutboundFieldMapJSON("sunbrightsolarusa")
	hexID := testRandomSunbaseHexID(t)
	custID := ""
	for _, id := range providers.SunbaseExternalIDCandidates(hexID) {
		if len(id) > 5 && id[:5] == "cust-" {
			custID = id
			break
		}
	}
	if custID == "" {
		t.Fatal("expected cust id candidate")
	}
	connID, connPublicID := testSunbaseConnection(ctx, t, pool, accountID, "SunBase Cust Match Conn "+suffix)

	ids, err := svc.ProvisionSunbaseWebhooks(
		ctx, accountID, connID, connPublicID, "SunBase Cust Match "+suffix,
		"sunbrightsolarusa", sunbaseDefaultEndpoint, fieldMap,
	)
	if err != nil {
		t.Fatalf("ProvisionSunbaseWebhooks: %v", err)
	}
	defer svc.DeleteSunbaseWebhooks(ctx, accountID, *ids)

	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	leadID, _, err := repo.InsertLead(ctx, tx, accountID, accountID, "", nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.SetBuiltinField(ctx, tx, leadID, "first_name", "Stored"); err != nil {
		t.Fatal(err)
	}
	if err := repo.SetExternalID(ctx, tx, leadID, custID); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}

	res, err := svc.Ingest(ctx, &WebhookAuth{WebhookID: ids.Inbound, AccountID: accountID}, ids.InboundSlug, map[string]any{
		"action":     "Update",
		"uuid":       hexID,
		"first_name": "Updated",
	})
	if err != nil {
		t.Fatalf("ingest: %v", err)
	}
	if len(res.Results) == 0 || res.Results[0].Status != "updated" {
		t.Fatalf("results = %+v, want updated", res.Results)
	}

	lead, err := repo.GetByExternalID(ctx, pool, accountID, hexID)
	if err != nil {
		t.Fatalf("GetByExternalID: %v", err)
	}
	if lead.FirstName != "Updated" {
		t.Fatalf("first_name = %q, want Updated", lead.FirstName)
	}
	if lead.ID != leadID {
		t.Fatalf("matched lead id = %d, want %d", lead.ID, leadID)
	}
}

func TestIngest_sunbaseInbound_phoneFallbackLinksUUID(t *testing.T) {
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
	fieldMap := defaultSunbaseOutboundFieldMapJSON("sunbrightsolarusa")
	extID := testRandomSunbaseHexID(t)
	phone := fmt.Sprintf("416%07d", time.Now().UnixNano()%10000000)
	connID, connPublicID := testSunbaseConnection(ctx, t, pool, accountID, "SunBase Fallback Conn "+suffix)

	ids, err := svc.ProvisionSunbaseWebhooks(
		ctx, accountID, connID, connPublicID, "SunBase Fallback "+suffix,
		"sunbrightsolarusa", sunbaseDefaultEndpoint, fieldMap,
	)
	if err != nil {
		t.Fatalf("ProvisionSunbaseWebhooks: %v", err)
	}
	defer svc.DeleteSunbaseWebhooks(ctx, accountID, *ids)

	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	leadID, _, err := repo.InsertLead(ctx, tx, accountID, accountID, "", nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.SetBuiltinField(ctx, tx, leadID, "first_name", "Fallback"); err != nil {
		t.Fatal(err)
	}
	if err := repo.SetBuiltinField(ctx, tx, leadID, "phone", phone); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}

	first, err := svc.Ingest(ctx, &WebhookAuth{WebhookID: ids.Inbound, AccountID: accountID}, ids.InboundSlug, map[string]any{
		"action":     "Update",
		"uuid":       extID,
		"phone":      phone,
		"first_name": "Linked",
	})
	if err != nil {
		t.Fatalf("first ingest: %v", err)
	}
	if len(first.Results) == 0 || first.Results[0].Status != "updated" {
		t.Fatalf("first results = %+v, want updated", first.Results)
	}

	lead, err := repo.GetByExternalID(ctx, pool, accountID, extID)
	if err != nil {
		t.Fatalf("GetByExternalID after fallback: %v", err)
	}
	if lead.ID != leadID {
		t.Fatalf("lead id = %d, want %d", lead.ID, leadID)
	}
	if lead.FirstName != "Linked" {
		t.Fatalf("first_name = %q, want Linked", lead.FirstName)
	}

	second, err := svc.Ingest(ctx, &WebhookAuth{WebhookID: ids.Inbound, AccountID: accountID}, ids.InboundSlug, map[string]any{
		"action":     "Update",
		"uuid":       extID,
		"first_name": "Direct",
	})
	if err != nil {
		t.Fatalf("second ingest: %v", err)
	}
	if len(second.Results) == 0 || second.Results[0].Status != "updated" {
		t.Fatalf("second results = %+v, want updated", second.Results)
	}

	lead, err = repo.GetByExternalID(ctx, pool, accountID, extID)
	if err != nil {
		t.Fatalf("GetByExternalID after direct uuid: %v", err)
	}
	if lead.FirstName != "Direct" {
		t.Fatalf("first_name = %q, want Direct", lead.FirstName)
	}
}
