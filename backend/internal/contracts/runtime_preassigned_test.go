package contracts

import (
	"context"
	"testing"

	"github.com/echayko/leadrula/backend/internal/config"
	"github.com/echayko/leadrula/backend/internal/database"
)

func TestFindActiveContractByBuyerPipeline_openOfferParticipation(t *testing.T) {
	cfg := config.Load()
	ctx := context.Background()
	pool, err := database.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer pool.Close()

	var publisherID, buyerID, sourcePipelineID, contractID int64
	err = pool.QueryRow(ctx,
		`SELECT c.publisher_id, p.buyer_id, c.source_pipeline_id, c.id
		 FROM contracts c
		 JOIN contract_participations p ON p.contract_id = c.id
		 WHERE c.deleted_at IS NULL AND c.status = 'active'
		   AND c.buyer_id IS NULL
		   AND c.source_pipeline_id IS NOT NULL AND c.source_pipeline_id > 0
		   AND p.status = 'active'
		 LIMIT 1`).Scan(&publisherID, &buyerID, &sourcePipelineID, &contractID)
	if err != nil {
		t.Skip("no open-offer contract with active participation")
	}

	got, err := FindActiveContractByBuyerPipeline(ctx, pool, publisherID, buyerID, sourcePipelineID)
	if err != nil {
		t.Fatalf("FindActiveContractByBuyerPipeline: %v", err)
	}
	if got != contractID {
		t.Fatalf("contract id = %d want %d", got, contractID)
	}
}

func TestGetTargetForPreassignedBuyer_openOffer(t *testing.T) {
	cfg := config.Load()
	ctx := context.Background()
	pool, err := database.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer pool.Close()

	var contractID, buyerID int64
	err = pool.QueryRow(ctx,
		`SELECT c.id, p.buyer_id
		 FROM contracts c
		 JOIN contract_participations p ON p.contract_id = c.id
		 WHERE c.deleted_at IS NULL AND c.status = 'active'
		   AND c.buyer_id IS NULL AND p.status = 'active'
		   AND p.buyer_pipeline_id > 0
		 LIMIT 1`).Scan(&contractID, &buyerID)
	if err != nil {
		t.Skip("no open-offer contract with active pipeline participation")
	}

	target, err := GetTargetForPreassignedBuyer(ctx, pool, contractID, buyerID)
	if err != nil {
		t.Fatalf("GetTargetForPreassignedBuyer: %v", err)
	}
	if target.BuyerID != buyerID {
		t.Fatalf("buyer id = %d want %d", target.BuyerID, buyerID)
	}
	if target.ParticipationID == 0 {
		t.Fatal("expected participation id on open-offer target")
	}
	if target.BuyerPipelineID == 0 {
		t.Fatal("expected buyer pipeline on open-offer target")
	}
}
