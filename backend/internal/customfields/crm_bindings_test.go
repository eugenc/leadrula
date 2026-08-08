package customfields

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/echayko/leadrula/backend/db"
	"github.com/echayko/leadrula/backend/internal/database"
)

type stubCRMSyncer struct {
	called bool
	connID int64
}

func (s *stubCRMSyncer) SyncCRMBindingFieldMaps(_ context.Context, connectionID int64) error {
	s.called = true
	s.connID = connectionID
	return nil
}

func TestImportFromCRM_dedupeAndCreate(t *testing.T) {
	pool := connectFoldersTestDB(t)
	ctx := context.Background()
	if err := database.Migrate(ctx, pool, db.Migrations, db.Dir); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	svc := NewService(pool)
	accountID := firstAccountID(t, pool)

	var providerID int64
	if err := pool.QueryRow(ctx,
		`SELECT id FROM integration_providers WHERE slug='hubspot' LIMIT 1`).Scan(&providerID); err != nil {
		t.Skip("hubspot provider not seeded")
	}

	connName := fmt.Sprintf("crm-import-test-%d", time.Now().UnixNano())
	var connectionID int64
	err := pool.QueryRow(ctx,
		`INSERT INTO integration_connections (account_id, provider_id, name, config, status)
		 VALUES ($1,$2,$3,'{}','active') RETURNING id`,
		accountID, providerID, connName).Scan(&connectionID)
	if err != nil {
		t.Fatalf("insert connection: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, `DELETE FROM custom_field_crm_bindings WHERE connection_id=$1`, connectionID)
		_, _ = pool.Exec(ctx, `DELETE FROM integration_connections WHERE id=$1`, connectionID)
	})

	syncer := &stubCRMSyncer{}
	svc.SetCRMBindingSyncer(syncer)

	crmFieldID := fmt.Sprintf("cf_%d", time.Now().UnixNano())
	fieldName := fmt.Sprintf("Test Score %d", time.Now().UnixNano())
	in := ImportFromCRMInput{
		ConnectionID: connectionID,
		Fields: []ImportFromCRMFieldInput{{
			CRMFieldID:       crmFieldID,
			CRMFieldKey:      "test_score",
			Name:             fieldName,
			DataType:         "number",
			Object:           "contact",
			LeadType:         "number",
			InboundSourceKey: "test_score",
		}},
	}

	res, err := svc.ImportFromCRM(ctx, accountID, in)
	if err != nil {
		t.Fatalf("ImportFromCRM: %v", err)
	}
	if res.Created != 1 {
		t.Fatalf("expected 1 created, got %d", res.Created)
	}
	if !syncer.called || syncer.connID != connectionID {
		t.Fatal("expected binding syncer to run")
	}

	var fieldID int64
	if err := pool.QueryRow(ctx,
		`SELECT custom_field_id FROM custom_field_crm_bindings WHERE connection_id=$1 AND crm_field_id=$2`,
		connectionID, crmFieldID).Scan(&fieldID); err != nil {
		t.Fatalf("binding missing: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, `DELETE FROM lead_custom_values WHERE custom_field_id=$1`, fieldID)
		_, _ = pool.Exec(ctx, `DELETE FROM custom_fields WHERE id=$1`, fieldID)
	})

	res2, err := svc.ImportFromCRM(ctx, accountID, in)
	if err != nil {
		t.Fatalf("second ImportFromCRM: %v", err)
	}
	if res2.Created != 0 || res2.Skipped != 1 {
		t.Fatalf("expected skip on re-import, got created=%d skipped=%d", res2.Created, res2.Skipped)
	}
}

func TestImportFromCRM_linkExistingByName(t *testing.T) {
	pool := connectFoldersTestDB(t)
	ctx := context.Background()
	if err := database.Migrate(ctx, pool, db.Migrations, db.Dir); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	svc := NewService(pool)
	accountID := firstAccountID(t, pool)

	fieldName := fmt.Sprintf("Test Score %d", time.Now().UnixNano())
	existing, err := svc.CreateField(ctx, accountID, fieldName, fmt.Sprintf("test_score_%d", time.Now().UnixNano()), "number", nil, nil)
	if err != nil {
		t.Fatalf("CreateField: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, `DELETE FROM custom_field_crm_bindings WHERE custom_field_id=$1`, existing.ID)
		_ = svc.DeleteField(ctx, accountID, existing.ID)
	})

	var providerID int64
	if err := pool.QueryRow(ctx,
		`SELECT id FROM integration_providers WHERE slug='hubspot' LIMIT 1`).Scan(&providerID); err != nil {
		t.Skip("hubspot provider not seeded")
	}

	connName := fmt.Sprintf("crm-link-test-%d", time.Now().UnixNano())
	var connectionID int64
	err = pool.QueryRow(ctx,
		`INSERT INTO integration_connections (account_id, provider_id, name, config, status)
		 VALUES ($1,$2,$3,'{}','active') RETURNING id`,
		accountID, providerID, connName).Scan(&connectionID)
	if err != nil {
		t.Fatalf("insert connection: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, `DELETE FROM custom_field_crm_bindings WHERE connection_id=$1`, connectionID)
		_, _ = pool.Exec(ctx, `DELETE FROM integration_connections WHERE id=$1`, connectionID)
	})

	syncer := &stubCRMSyncer{}
	svc.SetCRMBindingSyncer(syncer)

	crmFieldID := fmt.Sprintf("cf_link_%d", time.Now().UnixNano())
	in := ImportFromCRMInput{
		ConnectionID: connectionID,
		Fields: []ImportFromCRMFieldInput{{
			CRMFieldID:       crmFieldID,
			CRMFieldKey:      "test_score",
			Name:             fieldName,
			DataType:         "number",
			Object:           "contact",
			LeadType:         "number",
			InboundSourceKey: "test_score",
		}},
	}

	res, err := svc.ImportFromCRM(ctx, accountID, in)
	if err != nil {
		t.Fatalf("ImportFromCRM: %v", err)
	}
	if res.Created != 0 || res.Linked != 1 {
		t.Fatalf("expected linked=1 created=0, got linked=%d created=%d", res.Linked, res.Created)
	}

	var boundFieldID int64
	if err := pool.QueryRow(ctx,
		`SELECT custom_field_id FROM custom_field_crm_bindings WHERE connection_id=$1 AND crm_field_id=$2`,
		connectionID, crmFieldID).Scan(&boundFieldID); err != nil {
		t.Fatalf("binding missing: %v", err)
	}
	if boundFieldID != existing.ID {
		t.Fatalf("expected binding to existing field %d, got %d", existing.ID, boundFieldID)
	}

	var fieldCount int
	if err := pool.QueryRow(ctx,
		`SELECT count(*) FROM custom_fields WHERE account_id=$1 AND lower(trim(name))=lower(trim($2))`,
		accountID, fieldName).Scan(&fieldCount); err != nil {
		t.Fatalf("count fields: %v", err)
	}
	if fieldCount != 1 {
		t.Fatalf("expected 1 LR field with name, got %d", fieldCount)
	}

	res2, err := svc.ImportFromCRM(ctx, accountID, in)
	if err != nil {
		t.Fatalf("second ImportFromCRM: %v", err)
	}
	if res2.Skipped != 1 {
		t.Fatalf("expected skip on re-import, got skipped=%d", res2.Skipped)
	}
}

func TestSlugFieldKey_local(t *testing.T) {
	if got := slugFieldKey("My Custom Field"); got != "my_custom_field" {
		t.Fatalf("slugFieldKey = %q", got)
	}
}
