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
