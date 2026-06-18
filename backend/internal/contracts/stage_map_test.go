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
