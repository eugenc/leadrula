package integrations

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/echayko/leadrula/backend/internal/database"
)

func TestCRMInboundStageSyncInflight(t *testing.T) {
	ctx := context.Background()
	pool, err := database.Connect(ctx, "postgres://crm:crm@localhost:5432/crm?sslmode=disable")
	if err != nil {
		t.Skip(err)
	}
	defer pool.Close()

	var accountID, leadID, connID, webhookID int64
	if err := pool.QueryRow(ctx, `SELECT id FROM accounts ORDER BY id LIMIT 1`).Scan(&accountID); err != nil {
		t.Skip(err)
	}
	if err := pool.QueryRow(ctx, `SELECT id FROM leads ORDER BY id LIMIT 1`).Scan(&leadID); err != nil {
		t.Skip(err)
	}
	if err := pool.QueryRow(ctx,
		`SELECT c.id FROM integration_connections c
		 JOIN integration_providers p ON p.id = c.provider_id
		 WHERE p.slug = 'ghl' ORDER BY c.id LIMIT 1`).Scan(&connID); err != nil {
		t.Skip(err)
	}
	if err := pool.QueryRow(ctx, `SELECT id FROM webhooks ORDER BY id LIMIT 1`).Scan(&webhookID); err != nil {
		t.Skip(err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx,
			`DELETE FROM crm_inbound_stage_sync_retries WHERE lead_id=$1 AND connection_id=$2`,
			leadID, connID)
	})
	_, _ = pool.Exec(ctx,
		`DELETE FROM crm_inbound_stage_sync_retries WHERE lead_id=$1 AND connection_id=$2`,
		leadID, connID)

	svc := &Service{pool: pool}

	inflight, err := svc.crmInboundStageSyncInflight(ctx, leadID, connID)
	if err != nil {
		t.Fatal(err)
	}
	if inflight {
		t.Fatal("expected no inflight row initially")
	}

	_, err = pool.Exec(ctx,
		`INSERT INTO crm_inbound_stage_sync_retries
		   (account_id, lead_id, connection_id, webhook_id, payload, next_attempt_at)
		 VALUES ($1, $2, $3, $4, '{}', now())
		 ON CONFLICT (lead_id, connection_id) DO UPDATE SET updated_at = now()`,
		accountID, leadID, connID, webhookID)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx,
			`DELETE FROM crm_inbound_stage_sync_retries WHERE lead_id=$1 AND connection_id=$2`,
			leadID, connID)
	})

	inflight, err = svc.crmInboundStageSyncInflight(ctx, leadID, connID)
	if err != nil {
		t.Fatal(err)
	}
	if !inflight {
		t.Fatal("expected inflight row")
	}
}

func TestDeferJobForInboundStageSync(t *testing.T) {
	ctx := context.Background()
	pool, err := database.Connect(ctx, "postgres://crm:crm@localhost:5432/crm?sslmode=disable")
	if err != nil {
		t.Skip(err)
	}
	defer pool.Close()

	var jobID, leadID, connID int64
	var statusBefore string
	var attemptsBefore int
	err = pool.QueryRow(ctx,
		`SELECT id, lead_id, connection_id, status::text, attempts
		 FROM integration_delivery_queue
		 WHERE lead_id IS NOT NULL AND connection_id IS NOT NULL
		 ORDER BY created_at DESC LIMIT 1`,
	).Scan(&jobID, &leadID, &connID, &statusBefore, &attemptsBefore)
	if err != nil {
		t.Skip(err)
	}

	t.Cleanup(func() {
		_, _ = pool.Exec(ctx,
			`UPDATE integration_delivery_queue
			 SET status=$2::delivery_status, attempts=$3, next_attempt_at=now(), updated_at=now()
			 WHERE id=$1`, jobID, statusBefore, attemptsBefore)
	})

	_, err = pool.Exec(ctx,
		`UPDATE integration_delivery_queue
		 SET status='processing', attempts=attempts+1, updated_at=now()
		 WHERE id=$1`, jobID)
	if err != nil {
		t.Fatal(err)
	}

	svc := &Service{pool: pool}
	svc.deferJobForInboundStageSync(ctx, jobID)

	var statusAfter string
	var attemptsAfter int
	if err := pool.QueryRow(ctx,
		`SELECT status::text, attempts FROM integration_delivery_queue WHERE id=$1`, jobID,
	).Scan(&statusAfter, &attemptsAfter); err != nil {
		t.Fatal(err)
	}
	if statusAfter != "pending" {
		t.Fatalf("status = %q, want pending", statusAfter)
	}
	if attemptsAfter != attemptsBefore {
		t.Fatalf("attempts = %d, want %d (defer should not consume attempt)", attemptsAfter, attemptsBefore)
	}
}

func TestExecuteJob_defersGHLWhenInboundInflight(t *testing.T) {
	ctx := context.Background()
	pool, err := database.Connect(ctx, "postgres://crm:crm@localhost:5432/crm?sslmode=disable")
	if err != nil {
		t.Skip(err)
	}
	defer pool.Close()

	var accountID, leadID, connID, webhookID int64
	if err := pool.QueryRow(ctx, `SELECT id FROM accounts ORDER BY id LIMIT 1`).Scan(&accountID); err != nil {
		t.Skip(err)
	}
	if err := pool.QueryRow(ctx, `SELECT id FROM leads ORDER BY id LIMIT 1`).Scan(&leadID); err != nil {
		t.Skip(err)
	}
	if err := pool.QueryRow(ctx,
		`SELECT c.id FROM integration_connections c
		 JOIN integration_providers p ON p.id = c.provider_id
		 WHERE p.slug = 'ghl' AND c.status = 'active' ORDER BY c.id LIMIT 1`).Scan(&connID); err != nil {
		t.Skip(err)
	}
	if err := pool.QueryRow(ctx, `SELECT id FROM webhooks ORDER BY id LIMIT 1`).Scan(&webhookID); err != nil {
		t.Skip(err)
	}

	var jobID int64
	payload, _ := json.Marshal(map[string]any{"first_name": "Test"})
	if err := pool.QueryRow(ctx,
		`INSERT INTO integration_delivery_queue (lead_id, connection_id, payload, status, attempts)
		 VALUES ($1, $2, $3, 'pending', 0)
		 RETURNING id`, leadID, connID, payload).Scan(&jobID); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, `DELETE FROM integration_delivery_queue WHERE id=$1`, jobID)
		_, _ = pool.Exec(ctx,
			`DELETE FROM crm_inbound_stage_sync_retries WHERE lead_id=$1 AND connection_id=$2`,
			leadID, connID)
	})
	_, _ = pool.Exec(ctx,
		`DELETE FROM crm_inbound_stage_sync_retries WHERE lead_id=$1 AND connection_id=$2`,
		leadID, connID)

	_, err = pool.Exec(ctx,
		`INSERT INTO crm_inbound_stage_sync_retries
		   (account_id, lead_id, connection_id, webhook_id, payload, next_attempt_at)
		 VALUES ($1, $2, $3, $4, '{}', now())
		 ON CONFLICT (lead_id, connection_id) DO UPDATE SET updated_at = now()`,
		accountID, leadID, connID, webhookID)
	if err != nil {
		t.Fatal(err)
	}

	_, err = pool.Exec(ctx,
		`UPDATE integration_delivery_queue
		 SET status='processing', attempts=1, updated_at=now()
		 WHERE id=$1`, jobID)
	if err != nil {
		t.Fatal(err)
	}

	svc := &Service{pool: pool}
	svc.executeJob(ctx, jobID, connID, leadID, payload, 1, nil)

	var status string
	if err := pool.QueryRow(ctx,
		`SELECT status::text FROM integration_delivery_queue WHERE id=$1`, jobID).Scan(&status); err != nil {
		t.Fatal(err)
	}
	if status != "pending" {
		t.Fatalf("status = %q, want pending (deferred for inbound inflight)", status)
	}
}
