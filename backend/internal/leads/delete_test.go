package leads

import (
	"context"
	"errors"
	"testing"

	"github.com/echayko/leadrula/backend/internal/auth"
	"github.com/echayko/leadrula/backend/pkg/httpx"
	"github.com/jackc/pgx/v5/pgxpool"
)

func ensureRouteExecutionsLeadCascade(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	ctx := context.Background()
	var deleteAction string
	err := pool.QueryRow(ctx,
		`SELECT confdeltype::text FROM pg_constraint WHERE conname = 'route_executions_lead_id_fkey'`).
		Scan(&deleteAction)
	if err != nil {
		t.Fatalf("route_executions_lead_id_fkey: %v", err)
	}
	if deleteAction == "c" {
		return
	}
	_, err = pool.Exec(ctx, `
		ALTER TABLE route_executions DROP CONSTRAINT route_executions_lead_id_fkey;
		ALTER TABLE route_executions
		  ADD CONSTRAINT route_executions_lead_id_fkey
		  FOREIGN KEY (lead_id) REFERENCES leads(id) ON DELETE CASCADE`)
	if err != nil {
		t.Fatalf("apply route_executions cascade: %v", err)
	}
}

func TestDelete_withRouteExecution_succeeds(t *testing.T) {
	pool := connectLeadsTestDB(t)
	ensureRouteExecutionsLeadCascade(t, pool)
	ctx := context.Background()
	repo := NewRepository(pool)

	var publisherID, pipelineID, stageID int64
	err := pool.QueryRow(ctx,
		`SELECT a.id, p.id, ps.id
		 FROM accounts a
		 JOIN pipelines p ON p.account_id = a.id
		 JOIN pipeline_stages ps ON ps.pipeline_id = p.id
		 WHERE a.type = 'publisher' AND a.deleted_at IS NULL
		 ORDER BY ps.position, ps.id
		 LIMIT 1`).Scan(&publisherID, &pipelineID, &stageID)
	if err != nil {
		t.Skip("no publisher pipeline in database")
	}

	var leadID int64
	if err := pool.QueryRow(ctx,
		`INSERT INTO leads(owner_account_id, publisher_id, first_name, last_name, status, pipeline_id, stage_id)
		 VALUES ($1,$1,'Delete','RouteTest','review',$2,$3) RETURNING id`,
		publisherID, pipelineID, stageID).Scan(&leadID); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, `DELETE FROM leads WHERE id = $1`, leadID)
	})

	var execID int64
	if err := pool.QueryRow(ctx,
		`INSERT INTO route_executions(route_name, lead_id, owner_account_id, destination, trigger_type)
		 VALUES ('test route', $1, $2, 'pipeline', 'source_ingest') RETURNING id`,
		leadID, publisherID).Scan(&execID); err != nil {
		t.Fatal(err)
	}

	p := &auth.Principal{AccountID: publisherID, AccountType: "publisher", FullAccess: true}
	n, err := repo.Delete(ctx, p, []int64{leadID})
	if err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if n != 1 {
		t.Fatalf("Delete affected = %d want 1", n)
	}

	var exists bool
	if err := pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM leads WHERE id = $1)`, leadID).Scan(&exists); err != nil {
		t.Fatal(err)
	}
	if exists {
		t.Fatal("lead still exists after delete")
	}
	if err := pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM route_executions WHERE id = $1)`, execID).Scan(&exists); err != nil {
		t.Fatal(err)
	}
	if exists {
		t.Fatal("route_execution still exists after lead delete")
	}
}

func TestDelete_withOpenDispute_blocked(t *testing.T) {
	pool := connectLeadsTestDB(t)
	ensureRouteExecutionsLeadCascade(t, pool)
	ctx := context.Background()
	repo := NewRepository(pool)

	var publisherID, buyerID, pipelineID, stageID int64
	err := pool.QueryRow(ctx,
		`SELECT c.publisher_id, c.buyer_id, c.source_pipeline_id, ps.id
		 FROM contracts c
		 JOIN pipeline_stages ps ON ps.pipeline_id = c.source_pipeline_id
		 WHERE c.deleted_at IS NULL AND c.status = 'active' AND c.source_pipeline_id IS NOT NULL
		 ORDER BY ps.position, ps.id
		 LIMIT 1`).Scan(&publisherID, &buyerID, &pipelineID, &stageID)
	if err != nil {
		t.Skip("no active contract with publisher pipeline")
	}

	var leadID int64
	if err := pool.QueryRow(ctx,
		`INSERT INTO leads(owner_account_id, publisher_id, first_name, last_name, status, pipeline_id, stage_id)
		 VALUES ($1,$1,'Delete','DisputeTest','review',$2,$3) RETURNING id`,
		publisherID, pipelineID, stageID).Scan(&leadID); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, `DELETE FROM disputes WHERE lead_id = $1`, leadID)
		_, _ = pool.Exec(ctx, `DELETE FROM leads WHERE id = $1`, leadID)
	})

	var txnID int64
	if err := pool.QueryRow(ctx,
		`INSERT INTO transactions(buyer_id, lead_id, type, amount, balance_after, description)
		 VALUES ($1, $2, 'debit', 10, 0, 'delete test') RETURNING id`,
		buyerID, leadID).Scan(&txnID); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, `DELETE FROM disputes WHERE transaction_id = $1`, txnID)
		_, _ = pool.Exec(ctx, `DELETE FROM transactions WHERE id = $1`, txnID)
	})

	var disputeID int64
	if err := pool.QueryRow(ctx,
		`INSERT INTO disputes(transaction_id, buyer_id, reason, status, lead_id)
		 VALUES ($1, $2, 'test dispute', 'open', $3) RETURNING id`,
		txnID, buyerID, leadID).Scan(&disputeID); err != nil {
		t.Fatal(err)
	}

	p := &auth.Principal{AccountID: publisherID, AccountType: "publisher", FullAccess: true}
	_, err = repo.Delete(ctx, p, []int64{leadID})
	if err == nil {
		t.Fatal("expected delete to fail for lead with open dispute")
	}
	var appErr *httpx.AppError
	if !errors.As(err, &appErr) {
		t.Fatalf("expected AppError, got %T: %v", err, err)
	}
	if appErr.Code != httpx.CodeBusinessRule {
		t.Fatalf("code = %q want %q", appErr.Code, httpx.CodeBusinessRule)
	}
	if appErr.Message != "lead cannot be deleted while it has a dispute" {
		t.Fatalf("message = %q", appErr.Message)
	}
}
