package contracts

import (
	"context"
	"testing"

	"github.com/echayko/leadrula/backend/internal/config"
	"github.com/echayko/leadrula/backend/internal/database"
)

func TestUpdateBuyerContractDelivery_withCRMIntegration(t *testing.T) {
	cfg := config.Load()
	ctx := context.Background()
	pool, err := database.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer pool.Close()

	var contractID, buyerID, pipelineID, stageID, connID int64
	err = pool.QueryRow(ctx,
		`SELECT c.id, c.buyer_id, c.buyer_pipeline_id, cc.counterparty_stage_id, ic.id
		 FROM contracts c
		 JOIN contract_compensations cc ON cc.contract_id = c.id AND cc.participation_id IS NULL
		 JOIN integration_connections ic ON ic.account_id = c.buyer_id
		 JOIN integration_providers pr ON pr.id = ic.provider_id
		 WHERE c.deleted_at IS NULL AND c.status = 'active'
		   AND c.buyer_id IS NOT NULL AND c.contract_type = 'sell'
		   AND cc.delivery = 'leads_pipeline'
		   AND cc.counterparty_stage_id > 0
		   AND pr.slug = 'sunbase' AND ic.status = 'active'
		   AND NOT EXISTS (
		     SELECT 1 FROM contract_participations p
		     WHERE p.contract_id = c.id AND p.buyer_id = c.buyer_id AND p.status = 'active'
		   )
		 LIMIT 1`).Scan(&contractID, &buyerID, &pipelineID, &stageID, &connID)
	if err != nil {
		t.Skip("no active direct contract with sunbase connection")
	}

	svc := NewService(pool)
	c, err := svc.UpdateBuyerContractDelivery(ctx, buyerID, contractID, AcceptParticipationParams{
		Delivery:                "leads_pipeline",
		BuyerPipelineID:         pipelineID,
		BuyerTargetStageID:      stageID,
		IntegrationConnectionID: connID,
	})
	if err != nil {
		t.Fatalf("UpdateBuyerContractDelivery: %v", err)
	}
	if c.IntegrationConnectionID == nil || *c.IntegrationConnectionID != connID {
		t.Fatalf("integration_connection_id = %v, want %d", c.IntegrationConnectionID, connID)
	}

	target, err := GetTargetByContract(ctx, pool, contractID)
	if err != nil {
		t.Fatalf("GetTargetByContract: %v", err)
	}
	if target.IntegrationID != connID {
		t.Fatalf("target IntegrationID = %d, want %d", target.IntegrationID, connID)
	}
}
