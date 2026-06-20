package leads

import (
	"context"
	"testing"

	"github.com/echayko/leadrula/backend/internal/config"
	"github.com/echayko/leadrula/backend/internal/contracts"
	"github.com/echayko/leadrula/backend/internal/database"
	"github.com/jackc/pgx/v5/pgxpool"
)

func connectPublisherTrackingTestDB(t *testing.T) *pgxpool.Pool {
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

func TestPublisherPipelineTracking_distributeAndBuyerWon(t *testing.T) {
	pool := connectPublisherTrackingTestDB(t)
	ctx := context.Background()
	repo := NewRepository(pool)

	var publisherID, buyerID, contractID, pubPipelineID, buyerPipelineID int64
	var pubStage1, buyerStage1, buyerWon int64
	err := pool.QueryRow(ctx,
		`SELECT c.publisher_id, c.buyer_id, c.id, c.source_pipeline_id, c.buyer_pipeline_id
		 FROM contracts c
		 WHERE c.deleted_at IS NULL AND c.status = 'active'
		   AND c.source_pipeline_id IS NOT NULL AND c.buyer_pipeline_id IS NOT NULL
		   AND EXISTS(SELECT 1 FROM pipeline_stages ps WHERE ps.pipeline_id = c.source_pipeline_id AND ps.stage_type = 'won')
		   AND EXISTS(SELECT 1 FROM pipeline_stages ps WHERE ps.pipeline_id = c.buyer_pipeline_id AND ps.stage_type = 'won')
		 LIMIT 1`).Scan(&publisherID, &buyerID, &contractID, &pubPipelineID, &buyerPipelineID)
	if err != nil {
		t.Skip("no active contract with publisher and buyer pipelines")
	}

	if err := pool.QueryRow(ctx,
		`SELECT id FROM pipeline_stages WHERE pipeline_id = $1 ORDER BY position, id LIMIT 1`, pubPipelineID).Scan(&pubStage1); err != nil {
		t.Fatalf("pub stage: %v", err)
	}
	if err := pool.QueryRow(ctx,
		`SELECT id FROM pipeline_stages WHERE pipeline_id = $1 ORDER BY position, id LIMIT 1`, buyerPipelineID).Scan(&buyerStage1); err != nil {
		t.Fatalf("buyer stage: %v", err)
	}
	if err := pool.QueryRow(ctx,
		`SELECT id FROM pipeline_stages WHERE pipeline_id = $1 AND stage_type = 'won' ORDER BY position, id LIMIT 1`, buyerPipelineID).Scan(&buyerWon); err != nil {
		t.Fatalf("buyer won: %v", err)
	}

	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)

	// Lead starts tracked on the publisher board (pre-distribution state).
	var leadID int64
	if err := tx.QueryRow(ctx,
		`INSERT INTO leads(owner_account_id, publisher_id, first_name, last_name, status, pipeline_id, stage_id,
		 publisher_pipeline_id, publisher_stage_id)
		 VALUES ($1,$1,'Track','Test','review',$2,$3,$2,$3) RETURNING id`,
		publisherID, pubPipelineID, pubStage1).Scan(&leadID); err != nil {
		t.Fatalf("insert lead: %v", err)
	}

	// Distribution moves the lead to the buyer and clears publisher tracking.
	if err := repo.PlaceInPipeline(ctx, tx, leadID, buyerID, buyerPipelineID, buyerStage1, &contractID); err != nil {
		t.Fatalf("PlaceInPipeline: %v", err)
	}
	if err := contracts.ClearPublisherTracking(ctx, tx, leadID); err != nil {
		t.Fatalf("ClearPublisherTracking: %v", err)
	}
	if err := repo.SetStatus(ctx, tx, leadID, "distributed"); err != nil {
		t.Fatalf("SetStatus: %v", err)
	}

	var ownerID int64
	var pubPipe, pubStage *int64
	if err := tx.QueryRow(ctx,
		`SELECT owner_account_id, publisher_pipeline_id, publisher_stage_id FROM leads WHERE id = $1`,
		leadID).Scan(&ownerID, &pubPipe, &pubStage); err != nil {
		t.Fatalf("after distribute: %v", err)
	}
	if ownerID != buyerID {
		t.Fatalf("owner = %d want buyer %d", ownerID, buyerID)
	}
	if pubPipe != nil || pubStage != nil {
		t.Fatalf("publisher tracking = %v,%v want cleared after distribute", pubPipe, pubStage)
	}

	// A buyer stage change keeps the lead off the publisher board.
	if err := repo.UpdateStage(ctx, tx, leadID, buyerPipelineID, buyerWon); err != nil {
		t.Fatalf("UpdateStage buyer won: %v", err)
	}
	if err := contracts.SyncPublisherStage(ctx, tx, contractID, leadID, buyerID, buyerWon); err != nil {
		t.Fatalf("SyncPublisherStage: %v", err)
	}
	if err := repo.SetStatus(ctx, tx, leadID, "closed"); err != nil {
		t.Fatalf("SetStatus closed: %v", err)
	}

	var status string
	if err := tx.QueryRow(ctx,
		`SELECT owner_account_id, publisher_stage_id, status FROM leads WHERE id = $1`, leadID).
		Scan(&ownerID, &pubStage, &status); err != nil {
		t.Fatalf("after won scan: %v", err)
	}
	if ownerID != buyerID {
		t.Fatal("buyer should still own lead after won")
	}
	if pubStage != nil {
		t.Fatalf("publisher_stage_id = %v want cleared (nil) after buyer stage change", *pubStage)
	}
	if status != "closed" {
		t.Fatalf("status = %q want closed", status)
	}
}
