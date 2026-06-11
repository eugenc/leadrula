package webhooks

import (
	"context"
	"encoding/hex"
	"fmt"
	"testing"
	"time"

	"github.com/echayko/leadrula/backend/internal/database"
)

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
