package intake

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/echayko/leadrula/backend/internal/config"
	"github.com/echayko/leadrula/backend/internal/database"
	"github.com/echayko/leadrula/backend/internal/routing"
	"github.com/echayko/leadrula/backend/pkg/httpx"
	"github.com/jackc/pgx/v5/pgxpool"
)

func connectIntakeTestDB(t *testing.T) *pgxpool.Pool {
	t.Helper()
	cfg := config.Load()
	ctx := context.Background()
	pool, err := database.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(func() { pool.Close() })
	return pool
}

func TestComputeUnmappedKeysWithBuyerMaps(t *testing.T) {
	raw := map[string]any{
		"extra_a": "1",
		"extra_b": "2",
	}
	maps := []routing.SourceFieldMapEntry{
		{SourceKey: "extra_a"},
	}
	got := computeUnmappedKeys(raw, maps)
	if len(got) != 1 || got[0] != "extra_b" {
		t.Fatalf("expected [extra_b], got %v", got)
	}
}

func TestMapInboundFieldForBuyer_notFound(t *testing.T) {
	pool := connectIntakeTestDB(t)
	ctx := context.Background()
	svc := &Service{pool: pool}

	_, err := svc.MapInboundFieldForBuyer(ctx, 999999999, 999999999, "foo", "ignore", nil, nil)
	if err == nil {
		t.Fatal("expected not found")
	}
	var appErr *httpx.AppError
	if !errors.As(err, &appErr) || appErr.Code != httpx.CodeNotFound {
		t.Fatalf("expected not found, got %v", err)
	}
}

func TestMapInboundFieldForBuyer_ignoreReducesUnmapped(t *testing.T) {
	pool := connectIntakeTestDB(t)
	ctx := context.Background()

	var buyerID, leadID int64
	var rawPayload []byte
	var source *string
	err := pool.QueryRow(ctx,
		`SELECT t.buyer_id, l.id, COALESCE(q.raw_payload, l.raw_payload), COALESCE(q.source, l.source)
		 FROM transactions t
		 JOIN leads l ON l.id = t.lead_id
		 LEFT JOIN lead_intake_queue q ON q.lead_id = l.id
		 WHERE t.buyer_id IS NOT NULL
		   AND t.contract_id IS NOT NULL
		   AND t.type = 'debit'
		   AND t.lead_id IS NOT NULL
		   AND l.owner_account_id = t.buyer_id
		   AND COALESCE(q.raw_payload, l.raw_payload) IS NOT NULL
		   AND (
		     t.description LIKE 'lead routed:%'
		     OR t.description = 'lead routed from intake queue'
		     OR t.description = 'lead re-distributed'
		   )
		 LIMIT 1`).Scan(&buyerID, &leadID, &rawPayload, &source)
	if err != nil {
		t.Skip("no routed buyer lead with payload in database")
	}

	var raw map[string]any
	if err := json.Unmarshal(rawPayload, &raw); err != nil {
		t.Fatal(err)
	}
	flat := flattenPayload(raw)
	var unmappedKey string
	for k := range flat {
		if skipPayloadKeys[k] {
			continue
		}
		unmappedKey = k
		break
	}
	if unmappedKey == "" {
		t.Skip("lead payload has no mappable keys")
	}

	svc := &Service{pool: pool, leads: nil}
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)

	sourceSlug := ""
	if source != nil {
		sourceSlug = *source
	}
	if _, err := tx.Exec(ctx,
		`DELETE FROM buyer_inbound_field_map WHERE buyer_id = $1 AND source_slug = $2 AND source_key = $3`,
		buyerID, sourceSlug, unmappedKey); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}

	item, err := svc.MapInboundFieldForBuyer(ctx, buyerID, leadID, unmappedKey, "ignore", nil, nil)
	if err != nil {
		t.Fatalf("map ignore: %v", err)
	}
	for _, k := range item.UnmappedKeys {
		if k == unmappedKey {
			t.Fatalf("expected %q removed from unmapped, got %v", unmappedKey, item.UnmappedKeys)
		}
	}
}

func TestMapInboundFieldForBuyer_wrongBuyer(t *testing.T) {
	pool := connectIntakeTestDB(t)
	ctx := context.Background()

	var ownerBuyerID, leadID int64
	err := pool.QueryRow(ctx,
		`SELECT t.buyer_id, l.id
		 FROM transactions t
		 JOIN leads l ON l.id = t.lead_id
		 WHERE t.buyer_id IS NOT NULL
		   AND t.contract_id IS NOT NULL
		   AND t.type = 'debit'
		   AND l.owner_account_id = t.buyer_id
		 LIMIT 1`).Scan(&ownerBuyerID, &leadID)
	if err != nil {
		t.Skip("no routed buyer lead in database")
	}

	var otherBuyerID int64
	err = pool.QueryRow(ctx,
		`SELECT id FROM accounts WHERE account_type = 'buyer' AND id <> $1 LIMIT 1`, ownerBuyerID).
		Scan(&otherBuyerID)
	if err != nil {
		t.Skip("no second buyer account in database")
	}

	svc := &Service{pool: pool}
	_, err = svc.MapInboundFieldForBuyer(ctx, otherBuyerID, leadID, "first_name", "ignore", nil, nil)
	if err == nil {
		t.Fatal("expected not found for wrong buyer")
	}
	var appErr *httpx.AppError
	if !errors.As(err, &appErr) || appErr.Code != httpx.CodeNotFound {
		t.Fatalf("expected not found, got %v", err)
	}
}

func TestMapInboundFieldForBuyer_customFieldWrongAccount(t *testing.T) {
	pool := connectIntakeTestDB(t)
	ctx := context.Background()

	var buyerID, publisherID, leadID int64
	var rawPayload []byte
	err := pool.QueryRow(ctx,
		`SELECT t.buyer_id, l.publisher_id, l.id, COALESCE(q.raw_payload, l.raw_payload)
		 FROM transactions t
		 JOIN leads l ON l.id = t.lead_id
		 LEFT JOIN lead_intake_queue q ON q.lead_id = l.id
		 WHERE t.buyer_id IS NOT NULL
		   AND t.contract_id IS NOT NULL
		   AND t.type = 'debit'
		   AND l.owner_account_id = t.buyer_id
		   AND l.publisher_id <> t.buyer_id
		   AND COALESCE(q.raw_payload, l.raw_payload) IS NOT NULL
		 LIMIT 1`).Scan(&buyerID, &publisherID, &leadID, &rawPayload)
	if err != nil {
		t.Skip("no routed buyer lead with distinct publisher in database")
	}

	var raw map[string]any
	if err := json.Unmarshal(rawPayload, &raw); err != nil {
		t.Fatal(err)
	}
	flat := flattenPayload(raw)
	var payloadKey string
	for k := range flat {
		if skipPayloadKeys[k] {
			continue
		}
		payloadKey = k
		break
	}
	if payloadKey == "" {
		t.Skip("lead payload has no mappable keys")
	}

	var publisherFieldID int64
	err = pool.QueryRow(ctx,
		`SELECT id FROM custom_fields WHERE account_id = $1 LIMIT 1`, publisherID).
		Scan(&publisherFieldID)
	if err != nil {
		t.Skip("publisher has no custom fields")
	}

	svc := &Service{pool: pool}
	_, err = svc.MapInboundFieldForBuyer(ctx, buyerID, leadID, payloadKey, "custom", nil, &publisherFieldID)
	if err == nil {
		t.Fatal("expected validation error for publisher custom field")
	}
	var appErr *httpx.AppError
	if !errors.As(err, &appErr) || appErr.Code != httpx.CodeValidation {
		t.Fatalf("expected validation error, got %v", err)
	}
}
