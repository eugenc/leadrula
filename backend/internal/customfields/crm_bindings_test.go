package customfields

import (
	"context"
	"fmt"
	"testing"
	"time"
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
	in := ImportFromCRMInput{
		ConnectionID: connectionID,
		Fields: []ImportFromCRMFieldInput{{
			CRMFieldID:       crmFieldID,
			CRMFieldKey:      "test_score",
			Name:             "Test Score",
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

func TestSlugFieldKey_local(t *testing.T) {
	if got := slugFieldKey("My Custom Field"); got != "my_custom_field" {
		t.Fatalf("slugFieldKey = %q", got)
	}
}
