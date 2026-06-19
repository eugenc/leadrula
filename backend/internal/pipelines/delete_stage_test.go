package pipelines

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/echayko/leadrula/backend/internal/auth"
	"github.com/echayko/leadrula/backend/internal/collaboration"
	"github.com/echayko/leadrula/backend/internal/config"
	"github.com/echayko/leadrula/backend/internal/database"
	"github.com/echayko/leadrula/backend/pkg/httpx"
	"github.com/jackc/pgx/v5/pgxpool"
)

func connectPipelinesTestDB(t *testing.T) *pgxpool.Pool {
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

func newPipelinesTestService(t *testing.T, pool *pgxpool.Pool) *Service {
	t.Helper()
	return NewService(pool, collaboration.NewRepository(pool))
}

func testAdminPrincipal(accountID int64) *auth.Principal {
	return &auth.Principal{
		AccountID: accountID,
		Role:      "admin",
	}
}

func TestDeleteStage_unusedStage(t *testing.T) {
	pool := connectPipelinesTestDB(t)
	ctx := context.Background()
	svc := newPipelinesTestService(t, pool)

	var accountID, pipelineID int64
	err := pool.QueryRow(ctx,
		`SELECT account_id, id FROM pipelines ORDER BY id LIMIT 1`).Scan(&accountID, &pipelineID)
	if err != nil {
		t.Skip("no pipeline in database")
	}

	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)

	name := fmt.Sprintf("delete-test-%d", time.Now().UnixNano())
	var stageID int64
	err = tx.QueryRow(ctx,
		`INSERT INTO pipeline_stages(pipeline_id, name, position, color, stage_type)
		 VALUES ($1, $2, COALESCE((SELECT MAX(position)+1 FROM pipeline_stages WHERE pipeline_id=$1), 0), 'gray', 'standard')
		 RETURNING id`,
		pipelineID, name).Scan(&stageID)
	if err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM pipeline_stages WHERE id = $1`, stageID)
	})

	if err := svc.DeleteStage(ctx, testAdminPrincipal(accountID), stageID); err != nil {
		t.Fatalf("DeleteStage: %v", err)
	}

	var exists bool
	if err := pool.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM pipeline_stages WHERE id = $1)`, stageID).Scan(&exists); err != nil {
		t.Fatal(err)
	}
	if exists {
		t.Fatal("expected stage to be deleted")
	}
}

func TestDeleteStage_blocksWhenLeadInStage(t *testing.T) {
	pool := connectPipelinesTestDB(t)
	ctx := context.Background()
	svc := newPipelinesTestService(t, pool)

	var accountID, stageID int64
	err := pool.QueryRow(ctx,
		`SELECT l.owner_account_id, l.stage_id
		 FROM leads l
		 WHERE l.deleted_at IS NULL AND l.stage_id IS NOT NULL
		 LIMIT 1`).Scan(&accountID, &stageID)
	if err != nil {
		t.Skip("no lead with stage in database")
	}

	err = svc.DeleteStage(ctx, testAdminPrincipal(accountID), stageID)
	if err == nil {
		t.Fatal("expected business rule error")
	}
	var appErr *httpx.AppError
	if !errors.As(err, &appErr) || appErr.Code != httpx.CodeBusinessRule {
		t.Fatalf("expected business_rule, got %v", err)
	}
	if appErr.Message != "cannot delete stage with leads assigned; move leads first" {
		t.Fatalf("unexpected message: %q", appErr.Message)
	}
}

func TestDeleteStage_allowsDeleteWhenOnlyHistoryReferences(t *testing.T) {
	pool := connectPipelinesTestDB(t)
	ctx := context.Background()
	svc := newPipelinesTestService(t, pool)

	var accountID, pipelineID, leadID int64
	err := pool.QueryRow(ctx,
		`SELECT p.account_id, p.id, l.id
		 FROM pipelines p
		 JOIN pipeline_stages ps ON ps.pipeline_id = p.id
		 JOIN leads l ON l.stage_id = ps.id AND l.deleted_at IS NULL
		 LIMIT 1`).Scan(&accountID, &pipelineID, &leadID)
	if err != nil {
		t.Skip("no pipeline with lead in database")
	}

	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)

	name := fmt.Sprintf("history-delete-test-%d", time.Now().UnixNano())
	var stageID int64
	err = tx.QueryRow(ctx,
		`INSERT INTO pipeline_stages(pipeline_id, name, position, color, stage_type)
		 VALUES ($1, $2, COALESCE((SELECT MAX(position)+1 FROM pipeline_stages WHERE pipeline_id=$1), 0), 'gray', 'standard')
		 RETURNING id`,
		pipelineID, name).Scan(&stageID)
	if err != nil {
		t.Fatal(err)
	}

	var historyID int64
	err = tx.QueryRow(ctx,
		`INSERT INTO lead_stage_history(lead_id, from_stage_id, to_stage_id)
		 VALUES ($1, NULL, $2)
		 RETURNING id`,
		leadID, stageID).Scan(&historyID)
	if err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM lead_stage_history WHERE id = $1`, historyID)
	})

	if err := svc.DeleteStage(ctx, testAdminPrincipal(accountID), stageID); err != nil {
		t.Fatalf("DeleteStage: %v", err)
	}

	var toStageID *int64
	err = pool.QueryRow(ctx,
		`SELECT to_stage_id FROM lead_stage_history WHERE id = $1`, historyID).Scan(&toStageID)
	if err != nil {
		t.Fatal(err)
	}
	if toStageID != nil {
		t.Fatalf("expected history to_stage_id to be null, got %v", *toStageID)
	}

	var exists bool
	if err := pool.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM pipeline_stages WHERE id = $1)`, stageID).Scan(&exists); err != nil {
		t.Fatal(err)
	}
	if exists {
		t.Fatal("expected stage to be deleted")
	}
}
