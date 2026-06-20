package contracts

import (
	"context"
	"testing"

	"github.com/echayko/leadrula/backend/internal/config"
	"github.com/echayko/leadrula/backend/internal/database"
)

func TestBuildDeliveryStageMaps_entryAndReturnRoutes(t *testing.T) {
	rules := []returnRuleStage{
		{buyerStageID: 50, returnStageID: 500},
		{buyerStageID: 60, returnStageID: 600},
	}
	maps := buildDeliveryStageMaps(10, 100, rules)
	if maps[10] != 100 {
		t.Fatalf("entry map = %d want 100", maps[10])
	}
	if maps[50] != 500 || maps[60] != 600 {
		t.Fatalf("return route maps = %v", maps)
	}
	if len(maps) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(maps))
	}
}

func TestBuildDeliveryStageMaps_mismatchedPipelinesOk(t *testing.T) {
	// Buyer and publisher pipelines can differ; only explicit delivery + return routes are mapped.
	maps := buildDeliveryStageMaps(42, 99, nil)
	if len(maps) != 1 || maps[42] != 99 {
		t.Fatalf("maps = %v want single entry 42→99", maps)
	}
}

func TestBuildDeliveryStageMaps_skipsZeroIDs(t *testing.T) {
	maps := buildDeliveryStageMaps(0, 100, []returnRuleStage{{buyerStageID: 0, returnStageID: 500}})
	if len(maps) != 0 {
		t.Fatalf("expected empty maps, got %v", maps)
	}
}

func TestBuildDeliveryStageMaps_returnRuleOverridesSameBuyerStage(t *testing.T) {
	rules := []returnRuleStage{{buyerStageID: 10, returnStageID: 200}}
	maps := buildDeliveryStageMaps(10, 100, rules)
	if maps[10] != 200 {
		t.Fatalf("return route should win for same buyer stage: %v", maps)
	}
}

func TestLookupPublisherStage_usesParticipationSourcePipeline(t *testing.T) {
	cfg := config.Load()
	ctx := context.Background()
	pool, err := database.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(func() { pool.Close() })

	var contractID, buyerID, buyerStageID int64
	var partSourcePipeline, contractSourcePipeline int64
	err = pool.QueryRow(ctx,
		`SELECT p.contract_id, p.buyer_id,
		        (SELECT ps.id FROM pipeline_stages ps
		         WHERE ps.pipeline_id = p.buyer_pipeline_id
		         ORDER BY ps.position, ps.id LIMIT 1),
		        COALESCE(p.source_pipeline_id, c.source_pipeline_id),
		        c.source_pipeline_id
		 FROM contract_participations p
		 JOIN contracts c ON c.id = p.contract_id
		 WHERE p.status = 'active' AND p.buyer_pipeline_id > 0
		   AND p.source_pipeline_id IS NOT NULL
		   AND p.source_pipeline_id <> c.source_pipeline_id
		 LIMIT 1`).Scan(
		&contractID, &buyerID, &buyerStageID, &partSourcePipeline, &contractSourcePipeline)
	if err != nil {
		t.Skip("no participation with distinct source_pipeline_id in database")
	}

	pubPipelineID, _, err := lookupPublisherStage(ctx, pool, contractID, buyerID, buyerStageID)
	if err != nil && !isStageMapMissingErr(err) {
		// missing map is ok; we only care which pipeline ID was resolved
		if pubPipelineID == 0 {
			t.Fatalf("lookupPublisherStage: %v", err)
		}
	}
	if pubPipelineID != 0 && pubPipelineID != partSourcePipeline {
		t.Fatalf("publisher pipeline = %d, want participation source %d (contract has %d)",
			pubPipelineID, partSourcePipeline, contractSourcePipeline)
	}
}

func TestSyncPublisherStage_skipsInvalidMappedStage(t *testing.T) {
	cfg := config.Load()
	ctx := context.Background()
	pool, err := database.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(func() { pool.Close() })

	var contractID, leadID, buyerID, buyerStageID, wrongPubStage int64
	err = pool.QueryRow(ctx,
		`SELECT l.contract_id, l.id, l.owner_account_id, ps.id,
		        (SELECT ps2.id FROM pipeline_stages ps2
		         WHERE ps2.pipeline_id = l.pipeline_id AND ps2.id <> ps.id
		         ORDER BY ps2.position, ps2.id LIMIT 1)
		 FROM leads l
		 JOIN pipeline_stages ps ON ps.pipeline_id = l.pipeline_id
		 WHERE l.contract_id IS NOT NULL AND l.deleted_at IS NULL
		   AND l.pipeline_id IS NOT NULL
		 LIMIT 1`).Scan(&contractID, &leadID, &buyerID, &buyerStageID, &wrongPubStage)
	if err != nil {
		t.Skip("no contracted lead for sync test")
	}
	if wrongPubStage == 0 {
		t.Skip("pipeline has only one stage")
	}

	var mapID int64
	err = pool.QueryRow(ctx,
		`INSERT INTO contract_stage_maps(contract_id, participation_id, buyer_stage_id, publisher_stage_id)
		 VALUES ($1, NULL, $2, $3)
		 ON CONFLICT (contract_id, participation_id, buyer_stage_id)
		 DO UPDATE SET publisher_stage_id = EXCLUDED.publisher_stage_id
		 RETURNING id`,
		contractID, buyerStageID, wrongPubStage).Scan(&mapID)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, `DELETE FROM contract_stage_maps WHERE id = $1`, mapID)
	})

	if err := SyncPublisherStage(ctx, pool, contractID, leadID, buyerID, buyerStageID); err != nil {
		t.Fatalf("SyncPublisherStage should skip invalid map, got %v", err)
	}
}

func TestSyncPublisherStage_clearsTrackingWhenNoMap(t *testing.T) {
	cfg := config.Load()
	ctx := context.Background()
	pool, err := database.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(func() { pool.Close() })

	var contractID, publisherID, buyerID, buyerPipelineID int64
	err = pool.QueryRow(ctx,
		`SELECT c.id, c.publisher_id, c.buyer_id, c.buyer_pipeline_id
		 FROM contracts c
		 WHERE c.deleted_at IS NULL AND c.status = 'active'
		   AND c.buyer_pipeline_id IS NOT NULL
		 LIMIT 1`).Scan(&contractID, &publisherID, &buyerID, &buyerPipelineID)
	if err != nil {
		t.Skip("no active contract with buyer pipeline")
	}

	var buyerStageID int64
	if err := pool.QueryRow(ctx,
		`SELECT id FROM pipeline_stages WHERE pipeline_id = $1 ORDER BY position, id LIMIT 1`,
		buyerPipelineID).Scan(&buyerStageID); err != nil {
		t.Fatalf("buyer stage: %v", err)
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

	var leadID int64
	if err := tx.QueryRow(ctx,
		`INSERT INTO leads(owner_account_id, publisher_id, first_name, last_name, status, pipeline_id, stage_id, contract_id,
		 publisher_pipeline_id, publisher_stage_id)
		 VALUES ($1,$2,'Clear','Test','distributed',$3,$4,$5,$6,$7) RETURNING id`,
		buyerID, publisherID, buyerPipelineID, buyerStageID, contractID, buyerPipelineID, buyerStageID).Scan(&leadID); err != nil {
		t.Fatalf("insert lead: %v", err)
	}

	if err := SyncPublisherStage(ctx, tx, contractID, leadID, buyerID, buyerStageID); err != nil {
		t.Fatalf("SyncPublisherStage: %v", err)
	}

	var pubPipe, pubStage *int64
	if err := tx.QueryRow(ctx,
		`SELECT publisher_pipeline_id, publisher_stage_id FROM leads WHERE id = $1`, leadID).
		Scan(&pubPipe, &pubStage); err != nil {
		t.Fatalf("scan lead: %v", err)
	}
	if pubPipe != nil || pubStage != nil {
		t.Fatalf("publisher tracking = %v,%v want cleared", pubPipe, pubStage)
	}
}
