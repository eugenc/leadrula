package leads

import (
	"context"
	"errors"
	"testing"

	"github.com/echayko/leadrula/backend/internal/auth"
	"github.com/echayko/leadrula/backend/internal/config"
	"github.com/echayko/leadrula/backend/internal/database"
	"github.com/echayko/leadrula/backend/pkg/httpx"
	"github.com/jackc/pgx/v5/pgxpool"
)

func connectLeadsTestDB(t *testing.T) *pgxpool.Pool {
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

func TestAssertCanEdit(t *testing.T) {
	assigned := int64(42)
	lead := &Lead{AssignedUserID: &assigned}

	if err := assertCanEdit(&auth.Principal{Role: "admin"}, lead); err != nil {
		t.Fatalf("admin: %v", err)
	}
	if err := assertCanEdit(&auth.Principal{Role: "user", UserID: 42}, lead); err != nil {
		t.Fatalf("assigned user: %v", err)
	}
	if err := assertCanEdit(&auth.Principal{Role: "user", UserID: 99}, lead); err == nil {
		t.Fatal("expected forbidden for non-assigned user")
	}
	if err := assertCanEdit(&auth.Principal{Role: "viewer"}, lead); err == nil {
		t.Fatal("expected forbidden for viewer")
	}
}

func TestUpdateStage_setsPipelineID(t *testing.T) {
	pool := connectLeadsTestDB(t)
	ctx := context.Background()
	repo := NewRepository(pool)

	var leadID, stageID, pipelineID int64
	err := pool.QueryRow(ctx,
		`SELECT l.id, l.stage_id, ps.pipeline_id
		 FROM leads l
		 JOIN pipeline_stages ps ON ps.id = l.stage_id
		 WHERE l.deleted_at IS NULL AND l.stage_id IS NOT NULL
		 LIMIT 1`).Scan(&leadID, &stageID, &pipelineID)
	if err != nil {
		t.Skip("no lead with stage in database")
	}

	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)

	var otherStageID int64
	err = pool.QueryRow(ctx,
		`SELECT id FROM pipeline_stages WHERE pipeline_id = $1 AND id != $2 ORDER BY position, id LIMIT 1`,
		pipelineID, stageID).Scan(&otherStageID)
	if err != nil {
		t.Skip("pipeline has only one stage")
	}

	if err := repo.UpdateStage(ctx, tx, leadID, pipelineID, otherStageID); err != nil {
		t.Fatal(err)
	}

	var gotPipelineID, gotStageID int64
	err = tx.QueryRow(ctx,
		`SELECT pipeline_id, stage_id FROM leads WHERE id = $1`, leadID).Scan(&gotPipelineID, &gotStageID)
	if err != nil {
		t.Fatal(err)
	}
	if gotPipelineID != pipelineID || gotStageID != otherStageID {
		t.Fatalf("got pipeline=%d stage=%d want pipeline=%d stage=%d",
			gotPipelineID, gotStageID, pipelineID, otherStageID)
	}
}

func TestClearFromPipeline_nullsPipelineAndStage(t *testing.T) {
	pool := connectLeadsTestDB(t)
	ctx := context.Background()
	repo := NewRepository(pool)

	var leadID int64
	err := pool.QueryRow(ctx,
		`SELECT id FROM leads WHERE deleted_at IS NULL AND stage_id IS NOT NULL LIMIT 1`).Scan(&leadID)
	if err != nil {
		t.Skip("no lead with stage in database")
	}

	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)

	if err := repo.ClearFromPipeline(ctx, tx, leadID); err != nil {
		t.Fatal(err)
	}

	var pipelineID, stageID *int64
	err = tx.QueryRow(ctx,
		`SELECT pipeline_id, stage_id FROM leads WHERE id = $1`, leadID).Scan(&pipelineID, &stageID)
	if err != nil {
		t.Fatal(err)
	}
	if pipelineID != nil || stageID != nil {
		t.Fatalf("expected null pipeline and stage, got pipeline=%v stage=%v", pipelineID, stageID)
	}
}

func TestClearFromPipeline_serviceForbiddenForNonAssignee(t *testing.T) {
	pool := connectLeadsTestDB(t)
	ctx := context.Background()
	repo := NewRepository(pool)
	svc := &Service{repo: repo}

	var leadID, ownerAccountID, assigneeID int64
	err := pool.QueryRow(ctx,
		`SELECT l.id, l.owner_account_id, l.assigned_user_id
		 FROM leads l
		 WHERE l.deleted_at IS NULL AND l.assigned_user_id IS NOT NULL
		   AND l.stage_id IS NOT NULL
		 LIMIT 1`).Scan(&leadID, &ownerAccountID, &assigneeID)
	if err != nil {
		t.Skip("no assigned lead with stage in database")
	}

	p := &auth.Principal{
		AccountID: ownerAccountID,
		UserID:    assigneeID + 9999,
		Role:      "user",
	}
	_, err = svc.ClearFromPipeline(ctx, p, leadID)
	if err == nil {
		t.Fatal("expected forbidden for non-assigned user")
	}
	var appErr *httpx.AppError
	if !errors.As(err, &appErr) || appErr.Code != httpx.CodeForbidden {
		t.Fatalf("expected forbidden, got %v", err)
	}
}
