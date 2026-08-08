package routing

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/echayko/leadrula/backend/internal/config"
	"github.com/echayko/leadrula/backend/internal/database"
	"github.com/echayko/leadrula/backend/pkg/httpx"
	"github.com/jackc/pgx/v5/pgxpool"
)

func connectRoutingTestDB(t *testing.T) *pgxpool.Pool {
	t.Helper()
	cfg := config.Load()
	ctx := context.Background()
	pool, err := database.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		t.Skipf("connect: %v", err)
	}
	t.Cleanup(func() { pool.Close() })
	return pool
}

func testPublisherSource(t *testing.T, pool *pgxpool.Pool) (publisherID, sourceID int64) {
	t.Helper()
	ctx := context.Background()
	svc := NewService(pool)

	if err := pool.QueryRow(ctx,
		`SELECT id FROM accounts WHERE account_type = 'publisher' ORDER BY id LIMIT 1`).Scan(&publisherID); err != nil {
		t.Skip("no publisher account in database")
	}

	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	src, err := svc.CreateSource(ctx, publisherID, "Field map test "+suffix, "field-map-test-"+suffix, "webhook", nil, nil, nil)
	if err != nil {
		t.Fatalf("CreateSource: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM routing_sources WHERE id=$1`, src.ID)
	})
	return publisherID, src.ID
}

func testPublisherCustomField(t *testing.T, pool *pgxpool.Pool, publisherID int64, fieldKey string) int64 {
	t.Helper()
	ctx := context.Background()
	var id int64
	err := pool.QueryRow(ctx,
		`INSERT INTO custom_fields(account_id, name, field_key, type)
		 VALUES ($1, $2, $3, 'text')
		 RETURNING id`,
		publisherID, fieldKey, fieldKey).Scan(&id)
	if err != nil {
		t.Fatalf("insert custom field %q: %v", fieldKey, err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM custom_fields WHERE id=$1`, id)
	})
	return id
}

func TestAddSourceFieldMap_multiTargetSameKey(t *testing.T) {
	pool := connectRoutingTestDB(t)
	ctx := context.Background()
	svc := NewService(pool)
	publisherID, sourceID := testPublisherSource(t, pool)

	cf1 := testPublisherCustomField(t, pool, publisherID, fmt.Sprintf("action_date_%d", time.Now().UnixNano()))
	cf2 := testPublisherCustomField(t, pool, publisherID, fmt.Sprintf("appt_time_%d", time.Now().UnixNano()))
	sourceKey := "appt_time"

	e1, err := svc.AddSourceFieldMap(ctx, publisherID, sourceID, sourceKey, "custom", nil, &cf1)
	if err != nil {
		t.Fatalf("first map: %v", err)
	}
	e2, err := svc.AddSourceFieldMap(ctx, publisherID, sourceID, sourceKey, "custom", nil, &cf2)
	if err != nil {
		t.Fatalf("second map: %v", err)
	}
	if e1.ID == e2.ID {
		t.Fatal("expected distinct mapping rows")
	}

	maps, err := svc.ListSourceFieldMap(ctx, sourceID)
	if err != nil {
		t.Fatal(err)
	}
	var matched int
	for _, m := range maps {
		if m.SourceKey != sourceKey || m.TargetType != "custom" {
			continue
		}
		if m.CustomFieldID != nil && (*m.CustomFieldID == cf1 || *m.CustomFieldID == cf2) {
			matched++
		}
	}
	if matched != 2 {
		t.Fatalf("expected 2 mappings for %q, got %d (all maps: %+v)", sourceKey, matched, maps)
	}
}

func TestAddSourceFieldMap_duplicateTargetRejected(t *testing.T) {
	pool := connectRoutingTestDB(t)
	ctx := context.Background()
	svc := NewService(pool)
	publisherID, sourceID := testPublisherSource(t, pool)

	cf1 := testPublisherCustomField(t, pool, publisherID, fmt.Sprintf("dup_field_%d", time.Now().UnixNano()))
	sourceKey := "appt_time"

	if _, err := svc.AddSourceFieldMap(ctx, publisherID, sourceID, sourceKey, "custom", nil, &cf1); err != nil {
		t.Fatalf("first map: %v", err)
	}
	_, err := svc.AddSourceFieldMap(ctx, publisherID, sourceID, sourceKey, "custom", nil, &cf1)
	if err == nil {
		t.Fatal("expected duplicate target error")
	}
	var appErr *httpx.AppError
	if !errors.As(err, &appErr) || appErr.Code != httpx.CodeValidation {
		t.Fatalf("expected validation error, got %v", err)
	}
}

func TestAddSourceFieldMap_ignoreClearsMappings(t *testing.T) {
	pool := connectRoutingTestDB(t)
	ctx := context.Background()
	svc := NewService(pool)
	publisherID, sourceID := testPublisherSource(t, pool)

	cf1 := testPublisherCustomField(t, pool, publisherID, fmt.Sprintf("ignore_clear_%d", time.Now().UnixNano()))
	sourceKey := "appt_time"

	if _, err := svc.AddSourceFieldMap(ctx, publisherID, sourceID, sourceKey, "custom", nil, &cf1); err != nil {
		t.Fatalf("map: %v", err)
	}
	if _, err := svc.AddSourceFieldMap(ctx, publisherID, sourceID, sourceKey, "ignore", nil, nil); err != nil {
		t.Fatalf("ignore: %v", err)
	}

	maps, err := svc.ListSourceFieldMap(ctx, sourceID)
	if err != nil {
		t.Fatal(err)
	}
	for _, m := range maps {
		if m.SourceKey == sourceKey && m.TargetType != "ignore" {
			t.Fatalf("expected only ignore for %q, got %+v", sourceKey, m)
		}
	}
}

func TestAddSourceFieldMap_mappingRemovesIgnore(t *testing.T) {
	pool := connectRoutingTestDB(t)
	ctx := context.Background()
	svc := NewService(pool)
	publisherID, sourceID := testPublisherSource(t, pool)

	cf1 := testPublisherCustomField(t, pool, publisherID, fmt.Sprintf("ignore_remove_%d", time.Now().UnixNano()))
	sourceKey := "notes"

	if _, err := svc.AddSourceFieldMap(ctx, publisherID, sourceID, sourceKey, "ignore", nil, nil); err != nil {
		t.Fatalf("ignore: %v", err)
	}
	if _, err := svc.AddSourceFieldMap(ctx, publisherID, sourceID, sourceKey, "custom", nil, &cf1); err != nil {
		t.Fatalf("map: %v", err)
	}

	maps, err := svc.ListSourceFieldMap(ctx, sourceID)
	if err != nil {
		t.Fatal(err)
	}
	var ignoreCount, customCount int
	for _, m := range maps {
		if m.SourceKey != sourceKey {
			continue
		}
		switch m.TargetType {
		case "ignore":
			ignoreCount++
		case "custom":
			customCount++
		}
	}
	if ignoreCount != 0 || customCount != 1 {
		t.Fatalf("expected 1 custom and 0 ignore, got ignore=%d custom=%d maps=%+v", ignoreCount, customCount, maps)
	}
}
