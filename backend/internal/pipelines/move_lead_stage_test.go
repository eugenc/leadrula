package pipelines

import (
	"context"
	"testing"
)

func TestMoveLeadStage_updatesPipelineAndStage(t *testing.T) {
	pool := connectPipelinesTestDB(t)
	ctx := context.Background()

	var leadID, pipelineA, stageA, pipelineB, stageB int64
	err := pool.QueryRow(ctx,
		`SELECT l.id, l.pipeline_id, l.stage_id, ps2.pipeline_id, ps2.id
		 FROM leads l
		 JOIN pipeline_stages ps ON ps.id = l.stage_id
		 JOIN pipeline_stages ps2 ON ps2.pipeline_id <> l.pipeline_id
		 JOIN pipelines p ON p.id = ps2.pipeline_id AND p.account_id = l.owner_account_id
		 WHERE l.deleted_at IS NULL AND l.pipeline_id IS NOT NULL AND l.stage_id IS NOT NULL
		 LIMIT 1`).Scan(&leadID, &pipelineA, &stageA, &pipelineB, &stageB)
	if err != nil {
		t.Skip("need a lead and a stage in another pipeline on the same account")
	}

	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)

	if err := moveLeadStage(ctx, tx, leadID, stageB); err != nil {
		t.Fatalf("moveLeadStage: %v", err)
	}

	var gotPipelineID, gotStageID int64
	if err := tx.QueryRow(ctx,
		`SELECT pipeline_id, stage_id FROM leads WHERE id = $1`, leadID).Scan(&gotPipelineID, &gotStageID); err != nil {
		t.Fatal(err)
	}
	if gotPipelineID != pipelineB || gotStageID != stageB {
		t.Fatalf("got pipeline=%d stage=%d want pipeline=%d stage=%d", gotPipelineID, gotStageID, pipelineB, stageB)
	}
	_ = pipelineA
	_ = stageA
}
