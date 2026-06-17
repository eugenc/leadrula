package contracts

import (
	"context"
	"testing"

	"github.com/echayko/leadrula/backend/internal/config"
	"github.com/echayko/leadrula/backend/internal/database"
)

func TestInitPublisherTracking_fallsBackToDistributeStage(t *testing.T) {
	cfg := config.Load()
	ctx := context.Background()
	pool, err := database.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer pool.Close()

	var contractID, publisherID, buyerID, sourcePipelineID, sourceStageID int64
	err = pool.QueryRow(ctx,
		`SELECT c.id, c.publisher_id, c.buyer_id, c.source_pipeline_id, c.source_stage_id
		 FROM contracts c
		 WHERE c.deleted_at IS NULL AND c.status = 'active'
		   AND c.source_pipeline_id IS NOT NULL AND c.source_stage_id IS NOT NULL
		   AND c.buyer_id IS NOT NULL AND c.buyer_pipeline_id IS NOT NULL
		 LIMIT 1`).Scan(&contractID, &publisherID, &buyerID, &sourcePipelineID, &sourceStageID)
	if err != nil {
		t.Skip("no active direct contract with publisher pipeline delivery")
	}

	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)

	_, err = tx.Exec(ctx, `DELETE FROM contract_stage_maps WHERE contract_id = $1`, contractID)
	if err != nil {
		t.Fatalf("clear maps: %v", err)
	}

	var buyerStageID int64
	if err := tx.QueryRow(ctx,
		`SELECT id FROM pipeline_stages WHERE pipeline_id = (
		   SELECT buyer_pipeline_id FROM contracts WHERE id = $1
		 ) ORDER BY position, id LIMIT 1`, contractID).Scan(&buyerStageID); err != nil {
		t.Fatalf("buyer stage: %v", err)
	}

	var leadID int64
	if err := tx.QueryRow(ctx,
		`INSERT INTO leads(owner_account_id, publisher_id, first_name, last_name, status, pipeline_id, stage_id, contract_id)
		 VALUES ($1,$2,'Fallback','Test','distributed',
		   (SELECT buyer_pipeline_id FROM contracts WHERE id = $3), $4, $3)
		 RETURNING id`,
		buyerID, publisherID, contractID, buyerStageID).Scan(&leadID); err != nil {
		t.Fatalf("insert lead: %v", err)
	}

	if err := InitPublisherTracking(ctx, tx, contractID, leadID, buyerID, buyerStageID); err != nil {
		t.Fatalf("InitPublisherTracking: %v", err)
	}

	var pubPipe, pubStage *int64
	if err := tx.QueryRow(ctx,
		`SELECT publisher_pipeline_id, publisher_stage_id FROM leads WHERE id = $1`, leadID).
		Scan(&pubPipe, &pubStage); err != nil {
		t.Fatalf("scan lead: %v", err)
	}
	if pubPipe == nil || *pubPipe != sourcePipelineID {
		t.Fatalf("publisher_pipeline_id = %v want %d", pubPipe, sourcePipelineID)
	}
	if pubStage == nil || *pubStage != sourceStageID {
		t.Fatalf("publisher_stage_id = %v want distribute stage %d", pubStage, sourceStageID)
	}
}
