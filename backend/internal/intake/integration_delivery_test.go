package intake

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/echayko/leadrula/backend/internal/config"
	"github.com/echayko/leadrula/backend/internal/database"
	"github.com/echayko/leadrula/backend/internal/leads"
	"github.com/echayko/leadrula/backend/pkg/httpx"
)

func TestGetIntegrationDelivery_notFound(t *testing.T) {
	cfg := config.Load()
	ctx := context.Background()
	pool, err := database.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(func() { pool.Close() })

	svc := &Service{pool: pool}
	_, err = svc.GetIntegrationDelivery(ctx, 999999999, 999999999)
	if err == nil {
		t.Fatal("expected not found")
	}
	var appErr *httpx.AppError
	if !errors.As(err, &appErr) || appErr.Code != httpx.CodeNotFound {
		t.Fatalf("expected not found, got %v", err)
	}
}

func TestRetryIntegrationDelivery_notFound(t *testing.T) {
	cfg := config.Load()
	ctx := context.Background()
	pool, err := database.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(func() { pool.Close() })

	svc := &Service{pool: pool}
	err = svc.RetryIntegrationDelivery(ctx, 999999999, 999999999)
	if err == nil {
		t.Fatal("expected not found")
	}
	var appErr *httpx.AppError
	if !errors.As(err, &appErr) || appErr.Code != httpx.CodeNotFound {
		t.Fatalf("expected not found, got %v", err)
	}
}

func TestLabelPayloadCustomFields_nonNumericKeys(t *testing.T) {
	svc := &Service{}
	labeled, err := svc.labelPayloadCustomFields(context.Background(), 1, []byte(`{"custom_fields":{"not-a-number":"x"}}`))
	if err != nil {
		t.Fatalf("labelPayloadCustomFields: %v", err)
	}
	if labeled != nil {
		t.Fatalf("expected nil for non-numeric keys, got %v", labeled)
	}
}

func TestMergeDeliveryConfig_preservesConfig(t *testing.T) {
	old := []byte(`{"first_name":"Old","_config":{"route_key":"x"}}`)
	rebuilt := []byte(`{"first_name":"New","custom_fields":{"22":"2026-06-17T17:00"}}`)
	got := leads.MergeDeliveryConfig(rebuilt, old)
	var m map[string]any
	if err := json.Unmarshal(got, &m); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if m["first_name"] != "New" {
		t.Fatalf("first_name = %v, want New", m["first_name"])
	}
	cfg, ok := m["_config"].(map[string]any)
	if !ok || cfg["route_key"] != "x" {
		t.Fatalf("_config = %v, want route_key=x", m["_config"])
	}
	custom, ok := m["custom_fields"].(map[string]any)
	if !ok || custom["22"] != "2026-06-17T17:00" {
		t.Fatalf("custom_fields = %v", m["custom_fields"])
	}
}

func TestMergeDeliveryConfig_invalidJSON(t *testing.T) {
	rebuilt := []byte(`{"ok":true}`)
	if got := leads.MergeDeliveryConfig(rebuilt, []byte(`not json`)); string(got) != string(rebuilt) {
		t.Fatalf("got %s, want unchanged rebuilt", got)
	}
}

func TestRetryIntegrationDelivery_rebuildsPayloadFromLead(t *testing.T) {
	cfg := config.Load()
	ctx := context.Background()
	pool, err := database.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(func() { pool.Close() })

	var accountID, deliveryID, leadID int64
	var oldPayload []byte
	err = pool.QueryRow(ctx,
		`SELECT c.account_id, q.id, q.lead_id, q.payload
		 FROM integration_delivery_queue q
		 JOIN integration_connections c ON c.id = q.connection_id
		 WHERE q.lead_id IS NOT NULL
		 ORDER BY q.created_at DESC
		 LIMIT 1`).Scan(&accountID, &deliveryID, &leadID, &oldPayload)
	if err != nil {
		t.Skip("no integration delivery with lead_id in database")
	}

	leadRepo := leads.NewRepository(pool)
	svc := &Service{pool: pool, leads: leadRepo}

	// Capture payload before retry (may already be stale).
	var payloadBefore []byte
	if err := pool.QueryRow(ctx, `SELECT payload FROM integration_delivery_queue WHERE id=$1`, deliveryID).Scan(&payloadBefore); err != nil {
		t.Fatalf("read payload: %v", err)
	}

	if err := svc.RetryIntegrationDelivery(ctx, accountID, deliveryID); err != nil {
		t.Fatalf("RetryIntegrationDelivery: %v", err)
	}

	var payloadAfter []byte
	var status string
	if err := pool.QueryRow(ctx,
		`SELECT payload, status FROM integration_delivery_queue WHERE id=$1`, deliveryID,
	).Scan(&payloadAfter, &status); err != nil {
		t.Fatalf("read after retry: %v", err)
	}
	if status != "pending" {
		t.Fatalf("status = %q, want pending", status)
	}

	lead, err := leadRepo.GetByID(ctx, pool, leadID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if err := leads.LoadCustomValues(ctx, pool, lead); err != nil {
		t.Fatalf("LoadCustomValues: %v", err)
	}
	expected, err := leads.BuildDeliveryPayload(lead)
	if err != nil {
		t.Fatalf("BuildDeliveryPayload: %v", err)
	}
	expected = leads.MergeDeliveryConfig(expected, payloadBefore)

	var gotMap, wantMap map[string]any
	if err := json.Unmarshal(payloadAfter, &gotMap); err != nil {
		t.Fatalf("unmarshal after: %v", err)
	}
	if err := json.Unmarshal(expected, &wantMap); err != nil {
		t.Fatalf("unmarshal expected: %v", err)
	}
	gotCustom, _ := gotMap["custom_fields"].(map[string]any)
	wantCustom, _ := wantMap["custom_fields"].(map[string]any)
	if len(gotCustom) != len(wantCustom) {
		t.Fatalf("custom_fields count after=%d want=%d\ngot=%v\nwant=%v", len(gotCustom), len(wantCustom), gotCustom, wantCustom)
	}
	for k, v := range wantCustom {
		if gotCustom[k] != v {
			t.Fatalf("custom_fields[%q] = %v, want %v", k, gotCustom[k], v)
		}
	}

	// Restore original payload/status so we don't leave prod queue mutated.
	_, _ = pool.Exec(ctx,
		`UPDATE integration_delivery_queue SET payload=$2, status='success', attempts=1, updated_at=now() WHERE id=$1`,
		deliveryID, payloadBefore)
}

func TestRetryIntegrationDelivery_lead26IncludesBuyerCustomFields(t *testing.T) {
	cfg := config.Load()
	ctx := context.Background()
	pool, err := database.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(func() { pool.Close() })

	const accountID int64 = 4
	const deliveryID int64 = 14
	const leadID int64 = 26

	var exists bool
	if err := pool.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM integration_delivery_queue WHERE id=$1 AND lead_id=$2)`,
		deliveryID, leadID,
	).Scan(&exists); err != nil || !exists {
		t.Skip("lead 26 delivery queue item 14 not in database")
	}

	var payloadBefore []byte
	var statusBefore string
	if err := pool.QueryRow(ctx,
		`SELECT payload, status FROM integration_delivery_queue WHERE id=$1`, deliveryID,
	).Scan(&payloadBefore, &statusBefore); err != nil {
		t.Fatalf("read before: %v", err)
	}

	svc := &Service{pool: pool, leads: leads.NewRepository(pool)}
	if err := svc.RetryIntegrationDelivery(ctx, accountID, deliveryID); err != nil {
		t.Fatalf("RetryIntegrationDelivery: %v", err)
	}

	var payloadAfter []byte
	if err := pool.QueryRow(ctx, `SELECT payload FROM integration_delivery_queue WHERE id=$1`, deliveryID).Scan(&payloadAfter); err != nil {
		t.Fatalf("read after: %v", err)
	}
	var root map[string]any
	if err := json.Unmarshal(payloadAfter, &root); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	custom, ok := root["custom_fields"].(map[string]any)
	if !ok {
		t.Fatalf("custom_fields missing: %v", root)
	}
	if custom["22"] == nil || custom["22"] == "" {
		t.Fatalf("custom_fields[22] (appt_time) missing: %v", custom)
	}
	if custom["23"] == nil || custom["23"] == "" {
		t.Fatalf("custom_fields[23] (apptComments) missing: %v", custom)
	}

	t.Cleanup(func() {
		_, _ = pool.Exec(ctx,
			`UPDATE integration_delivery_queue SET payload=$2, status=$3, updated_at=now() WHERE id=$1`,
			deliveryID, payloadBefore, statusBefore)
	})
}

func TestGetIntegrationDelivery_attemptsOrdered(t *testing.T) {
	cfg := config.Load()
	ctx := context.Background()
	pool, err := database.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(func() { pool.Close() })

	var accountID, deliveryID int64
	err = pool.QueryRow(ctx,
		`SELECT c.account_id, q.id
		 FROM integration_delivery_queue q
		 JOIN integration_connections c ON c.id = q.connection_id
		 WHERE q.webhook_trigger_id IS NULL
		   AND EXISTS (SELECT 1 FROM integration_delivery_logs l WHERE l.queue_item_id = q.id)
		 ORDER BY q.created_at DESC
		 LIMIT 1`).Scan(&accountID, &deliveryID)
	if err != nil {
		t.Skip("no integration delivery with attempt logs in database")
	}

	svc := &Service{pool: pool}
	detail, err := svc.GetIntegrationDelivery(ctx, accountID, deliveryID)
	if err != nil {
		t.Fatalf("GetIntegrationDelivery: %v", err)
	}
	if detail.ID != deliveryID {
		t.Fatalf("id = %d, want %d", detail.ID, deliveryID)
	}
	if len(detail.Attempts) == 0 {
		t.Fatal("expected at least one attempt")
	}
	for i, a := range detail.Attempts {
		want := i + 1
		if a.AttemptNumber != want {
			t.Fatalf("attempt[%d].number = %d, want %d", i, a.AttemptNumber, want)
		}
	}
}
