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
}

func TestIngest_sunbaseInbound_nullSecretPrefix(t *testing.T) {
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
		ctx, accountID, 0, publicID, "SunBase Ingest "+suffix,
		"sunbrightsolarusa", sunbaseDefaultEndpoint, fieldMap,
	)
	if err != nil {
		t.Fatalf("ProvisionSunbaseWebhooks: %v", err)
	}
	defer svc.DeleteSunbaseWebhooks(ctx, accountID, *ids)

	var secretPrefix *string
	if err := pool.QueryRow(ctx, `SELECT secret_prefix FROM webhooks WHERE id=$1`, ids.Inbound).Scan(&secretPrefix); err != nil {
		t.Fatal(err)
	}
	if secretPrefix != nil {
		t.Fatalf("expected null secret_prefix on sunbase inbound webhook, got %q", *secretPrefix)
	}

	repo := leads.NewRepository(pool)
	leadSvc := leads.NewService(repo, nil, nil, nil, nil)
	svc.leads = repo
	svc.leadSvc = leadSvc

	res, err := svc.Ingest(ctx, &WebhookAuth{WebhookID: ids.Inbound, AccountID: accountID}, ids.InboundSlug, map[string]any{
		"action": "Update",
	})
	if err != nil {
		t.Fatalf("Ingest: %v", err)
	}
	if res.Status != "captured" {
		t.Fatalf("status = %q, want captured for non-matching action", res.Status)
	}
}
