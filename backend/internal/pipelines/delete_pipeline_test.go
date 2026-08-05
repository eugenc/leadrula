package pipelines

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/echayko/leadrula/backend/pkg/httpx"
)

func TestDeletePipeline_unusedPipeline(t *testing.T) {
	pool := connectPipelinesTestDB(t)
	ctx := context.Background()
	svc := newPipelinesTestService(t, pool)

	var accountID int64
	err := pool.QueryRow(ctx, `SELECT id FROM accounts ORDER BY id LIMIT 1`).Scan(&accountID)
	if err != nil {
		t.Skip("no account in database")
	}

	pl, err := svc.Create(ctx, testAdminPrincipal(accountID), fmt.Sprintf("delete-pipeline-test-%d", time.Now().UnixNano()))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM pipelines WHERE id = $1`, pl.ID)
	})

	if err := svc.Delete(ctx, testAdminPrincipal(accountID), pl.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	var exists bool
	if err := pool.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM pipelines WHERE id = $1)`, pl.ID).Scan(&exists); err != nil {
		t.Fatal(err)
	}
	if exists {
		t.Fatal("expected pipeline to be deleted")
	}
}

func TestDeletePipeline_blocksWhenLeadAssigned(t *testing.T) {
	pool := connectPipelinesTestDB(t)
	ctx := context.Background()
	svc := newPipelinesTestService(t, pool)

	var accountID, pipelineID int64
	err := pool.QueryRow(ctx,
		`SELECT l.owner_account_id, l.pipeline_id
		 FROM leads l
		 WHERE l.deleted_at IS NULL AND l.pipeline_id IS NOT NULL
		 LIMIT 1`).Scan(&accountID, &pipelineID)
	if err != nil {
		t.Skip("no lead with pipeline in database")
	}

	err = svc.Delete(ctx, testAdminPrincipal(accountID), pipelineID)
	if err == nil {
		t.Fatal("expected business rule error")
	}
	var appErr *httpx.AppError
	if !errors.As(err, &appErr) || appErr.Code != httpx.CodeBusinessRule {
		t.Fatalf("expected business_rule, got %v", err)
	}
	if appErr.Message != "cannot delete pipeline with leads assigned; move leads first" {
		t.Fatalf("unexpected message: %q", appErr.Message)
	}
}

func TestDeletePipeline_blocksWhenLeadInStage(t *testing.T) {
	pool := connectPipelinesTestDB(t)
	ctx := context.Background()
	svc := newPipelinesTestService(t, pool)

	var accountID, pipelineID int64
	err := pool.QueryRow(ctx,
		`SELECT p.account_id, p.id
		 FROM pipelines p
		 JOIN pipeline_stages ps ON ps.pipeline_id = p.id
		 JOIN leads l ON l.stage_id = ps.id AND l.deleted_at IS NULL
		 WHERE l.pipeline_id IS NULL OR l.pipeline_id <> p.id
		 LIMIT 1`).Scan(&accountID, &pipelineID)
	if err != nil {
		t.Skip("no pipeline with lead in stage only")
	}

	err = svc.Delete(ctx, testAdminPrincipal(accountID), pipelineID)
	if err == nil {
		t.Fatal("expected business rule error")
	}
	var appErr *httpx.AppError
	if !errors.As(err, &appErr) || appErr.Code != httpx.CodeBusinessRule {
		t.Fatalf("expected business_rule, got %v", err)
	}
	if appErr.Message != "cannot delete pipeline with leads assigned; move leads first" {
		t.Fatalf("unexpected message: %q", appErr.Message)
	}
}

func TestDeletePipeline_blocksWhenUsedByContract(t *testing.T) {
	pool := connectPipelinesTestDB(t)
	ctx := context.Background()
	svc := newPipelinesTestService(t, pool)

	var accountID, pipelineID int64
	err := pool.QueryRow(ctx,
		`SELECT a.id, c.source_pipeline_id
		 FROM contracts c
		 JOIN accounts a ON a.id = c.publisher_id OR a.id = c.buyer_id
		 JOIN pipelines p ON p.id = c.source_pipeline_id AND p.account_id = a.id
		 WHERE c.deleted_at IS NULL
		 LIMIT 1`).Scan(&accountID, &pipelineID)
	if err != nil {
		t.Skip("no contract with source pipeline in database")
	}

	err = svc.Delete(ctx, testAdminPrincipal(accountID), pipelineID)
	if err == nil {
		t.Fatal("expected business rule error")
	}
	var appErr *httpx.AppError
	if !errors.As(err, &appErr) || appErr.Code != httpx.CodeBusinessRule {
		t.Fatalf("expected business_rule, got %v", err)
	}
	if appErr.Message != "cannot delete pipeline used by a contract" {
		t.Fatalf("unexpected message: %q", appErr.Message)
	}
}
