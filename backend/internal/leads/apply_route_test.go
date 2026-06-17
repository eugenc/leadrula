package leads

import (
	"context"
	"errors"
	"testing"

	"github.com/echayko/leadrula/backend/internal/config"
	"github.com/echayko/leadrula/backend/internal/database"
	"github.com/echayko/leadrula/backend/pkg/httpx"
	"github.com/jackc/pgx/v5"
)

func TestResolveBuyerDestStage_usesParticipationStage(t *testing.T) {
	got, err := resolveBuyerDestStage(context.Background(), nil, 42, 99)
	if err != nil {
		t.Fatal(err)
	}
	if got != 42 {
		t.Fatalf("got %d want 42", got)
	}
}

func TestResolveBuyerDestStage_firstBuyerStageWhenUnset(t *testing.T) {
	pool := connectLeadsTestDB(t)
	ctx := context.Background()

	var pipelineID int64
	err := pool.QueryRow(ctx,
		`SELECT pipeline_id FROM pipeline_stages ORDER BY pipeline_id, position, id LIMIT 1`).Scan(&pipelineID)
	if err != nil {
		t.Skip("no pipeline stages in database")
	}

	var wantStageID int64
	err = pool.QueryRow(ctx,
		`SELECT id FROM pipeline_stages WHERE pipeline_id=$1 ORDER BY position, id LIMIT 1`,
		pipelineID).Scan(&wantStageID)
	if err != nil {
		t.Fatal(err)
	}

	got, err := resolveBuyerDestStage(ctx, pool, 0, pipelineID)
	if err != nil {
		t.Fatal(err)
	}
	if got != wantStageID {
		t.Fatalf("got %d want first stage %d", got, wantStageID)
	}
}

func TestResolveBuyerDestStage_ignoresPublisherRouteStage(t *testing.T) {
	pool := connectLeadsTestDB(t)
	ctx := context.Background()

	var buyerPipelineID, publisherStageID int64
	err := pool.QueryRow(ctx,
		`SELECT ps.pipeline_id, ps.id
		 FROM pipeline_stages ps
		 JOIN pipelines p ON p.id = ps.pipeline_id
		 ORDER BY ps.pipeline_id, ps.position, ps.id
		 OFFSET 1 LIMIT 1`).Scan(&buyerPipelineID, &publisherStageID)
	if err != nil {
		t.Skip("need at least two pipeline stages")
	}

	got, err := resolveBuyerDestStage(ctx, pool, 0, buyerPipelineID)
	if err != nil {
		t.Fatal(err)
	}
	if got == publisherStageID {
		t.Fatalf("must not use publisher stage id %d as buyer dest", publisherStageID)
	}

	var stagePipelineID int64
	err = pool.QueryRow(ctx, `SELECT pipeline_id FROM pipeline_stages WHERE id=$1`, got).Scan(&stagePipelineID)
	if err != nil {
		t.Fatal(err)
	}
	if stagePipelineID != buyerPipelineID {
		t.Fatalf("dest stage %d belongs to pipeline %d want %d", got, stagePipelineID, buyerPipelineID)
	}
}

func TestPlaceInPipeline_rejectsStageFromWrongPipeline(t *testing.T) {
	pool := connectLeadsTestDB(t)
	ctx := context.Background()
	repo := NewRepository(pool)

	var leadID, pipelineID, stageID int64
	err := pool.QueryRow(ctx,
		`SELECT l.id, l.pipeline_id, ps.id
		 FROM leads l
		 JOIN pipeline_stages ps ON ps.pipeline_id <> l.pipeline_id
		 WHERE l.pipeline_id IS NOT NULL AND l.deleted_at IS NULL
		 LIMIT 1`).Scan(&leadID, &pipelineID, &stageID)
	if err != nil {
		t.Skip("no lead/pipeline/stage mismatch candidate")
	}

	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)

	err = repo.PlaceInPipeline(ctx, tx, leadID, 1, pipelineID, stageID, nil)
	var appErr *httpx.AppError
	if !errors.As(err, &appErr) || appErr.Code != httpx.CodeBusinessRule {
		t.Fatalf("expected business rule error, got %v", err)
	}
}

func TestPlaceInPipeline_acceptsMatchingStage(t *testing.T) {
	pool := connectLeadsTestDB(t)
	ctx := context.Background()
	repo := NewRepository(pool)

	var leadID, ownerID, pipelineID, stageID int64
	err := pool.QueryRow(ctx,
		`SELECT l.id, l.owner_account_id, ps.pipeline_id, ps.id
		 FROM leads l
		 JOIN pipeline_stages ps ON ps.pipeline_id IS NOT NULL
		 WHERE l.deleted_at IS NULL
		 LIMIT 1`).Scan(&leadID, &ownerID, &pipelineID, &stageID)
	if err != nil {
		t.Skip("no lead with pipeline stage")
	}

	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)

	if err := repo.PlaceInPipeline(ctx, tx, leadID, ownerID, pipelineID, stageID, nil); err != nil {
		t.Fatal(err)
	}
}

func TestRepairMigrationQuery_noRemainingMismatch(t *testing.T) {
	cfg := config.Load()
	ctx := context.Background()
	pool, err := database.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer pool.Close()

	var count int
	err = pool.QueryRow(ctx,
		`SELECT COUNT(*)
		 FROM leads l
		 JOIN pipeline_stages ps ON ps.id = l.stage_id
		 JOIN accounts oa ON oa.id = l.owner_account_id
		 WHERE l.pipeline_id IS NOT NULL
		   AND l.stage_id IS NOT NULL
		   AND l.pipeline_id <> ps.pipeline_id
		   AND l.deleted_at IS NULL
		   AND oa.type = 'buyer'`).Scan(&count)
	if err != nil {
		t.Fatal(err)
	}
	if count > 0 {
		t.Fatalf("%d buyer leads still have stage_id from a different pipeline; run migration 0073", count)
	}
}

func TestRepairMigrationQuery_skipsWhenClean(t *testing.T) {
	pool := connectLeadsTestDB(t)
	ctx := context.Background()

	var count int
	err := pool.QueryRow(ctx,
		`SELECT COUNT(*)
		 FROM leads l
		 JOIN pipeline_stages ps ON ps.id = l.stage_id
		 JOIN accounts oa ON oa.id = l.owner_account_id
		 WHERE l.pipeline_id IS NOT NULL
		   AND l.stage_id IS NOT NULL
		   AND l.pipeline_id <> ps.pipeline_id
		   AND l.deleted_at IS NULL
		   AND oa.type = 'buyer'`).Scan(&count)
	if errors.Is(err, pgx.ErrNoRows) {
		return
	}
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("buyer leads with pipeline/stage mismatch: %d", count)
}

func TestDiagnosticSunbrightBoardLeads(t *testing.T) {
	pool := connectLeadsTestDB(t)
	ctx := context.Background()

	var accountID int64
	err := pool.QueryRow(ctx,
		`SELECT id FROM accounts
		 WHERE deleted_at IS NULL AND type = 'buyer'
		   AND name ILIKE '%sunbright%'
		 LIMIT 1`).Scan(&accountID)
	if errors.Is(err, pgx.ErrNoRows) {
		t.Skip("Sunbright buyer account not in local database")
	}
	if err != nil {
		t.Fatal(err)
	}

	var total, mismatched, onBoard int
	err = pool.QueryRow(ctx,
		`SELECT COUNT(*)
		 FROM leads l
		 WHERE l.owner_account_id = $1 AND l.deleted_at IS NULL AND l.pipeline_id IS NOT NULL`,
		accountID).Scan(&total)
	if err != nil {
		t.Fatal(err)
	}
	err = pool.QueryRow(ctx,
		`SELECT COUNT(*)
		 FROM leads l
		 JOIN pipeline_stages ps ON ps.id = l.stage_id
		 WHERE l.owner_account_id = $1
		   AND l.deleted_at IS NULL
		   AND l.pipeline_id IS NOT NULL
		   AND l.pipeline_id <> ps.pipeline_id`,
		accountID).Scan(&mismatched)
	if err != nil {
		t.Fatal(err)
	}
	err = pool.QueryRow(ctx,
		`SELECT COUNT(*)
		 FROM leads l
		 JOIN pipeline_stages ps ON ps.id = l.stage_id AND ps.pipeline_id = l.pipeline_id
		 WHERE l.owner_account_id = $1
		   AND l.deleted_at IS NULL
		   AND l.pipeline_id IS NOT NULL`,
		accountID).Scan(&onBoard)
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("Sunbright (account %d): %d in pipeline, %d mismatched stage, %d board-visible", accountID, total, mismatched, onBoard)
	if mismatched > 0 {
		t.Fatalf("%d Sunbright leads still have stage_id outside their pipeline; run migration 0073", mismatched)
	}
}
