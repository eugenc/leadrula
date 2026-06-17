package contracts

import (
	"context"
	"testing"

	"github.com/echayko/leadrula/backend/internal/config"
	"github.com/echayko/leadrula/backend/internal/database"
)

func TestValidateBuyerCRMConnection_sunbase(t *testing.T) {
	cfg := config.Load()
	ctx := context.Background()
	pool, err := database.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer pool.Close()

	var connID int64
	err = pool.QueryRow(ctx,
		`SELECT ic.id FROM integration_connections ic
		 JOIN integration_providers p ON p.id = ic.provider_id
		 WHERE p.slug = 'sunbase' AND ic.status = 'active'
		 LIMIT 1`).Scan(&connID)
	if err != nil {
		t.Fatalf("no sunbase connection: %v", err)
	}
	var accountID int64
	err = pool.QueryRow(ctx, `SELECT account_id FROM integration_connections WHERE id = $1`, connID).Scan(&accountID)
	if err != nil {
		t.Fatalf("lookup account: %v", err)
	}
	if err := validateBuyerCRMConnection(ctx, pool, accountID, connID); err != nil {
		t.Fatalf("validateBuyerCRMConnection: %v", err)
	}
}

func TestUpdateParticipationDelivery_withCRMIntegration(t *testing.T) {
	cfg := config.Load()
	ctx := context.Background()
	pool, err := database.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer pool.Close()

	var participationID, buyerID, pipelineID, stageID, connID int64
	err = pool.QueryRow(ctx,
		`SELECT p.id, p.buyer_id, p.buyer_pipeline_id, p.buyer_target_stage_id, ic.id
		 FROM contract_participations p
		 JOIN integration_connections ic ON ic.account_id = p.buyer_id
		 JOIN integration_providers pr ON pr.id = ic.provider_id
		 WHERE p.status = 'active' AND p.delivery = 'leads_pipeline'
		   AND pr.slug = 'sunbase' AND ic.status = 'active'
		 AND p.buyer_pipeline_id > 0 AND p.buyer_target_stage_id > 0
		   AND EXISTS (SELECT 1 FROM pipeline_stages ps WHERE ps.pipeline_id = p.buyer_pipeline_id AND ps.stage_type = 'won')
		   AND EXISTS (SELECT 1 FROM contract_return_rules r WHERE r.participation_id = p.id)
		 LIMIT 1`).Scan(&participationID, &buyerID, &pipelineID, &stageID, &connID)
	if err != nil {
		t.Skip("no active pipeline participation with sunbase connection and return routes")
	}

	svc := NewService(pool)
	part, err := svc.UpdateParticipationDelivery(ctx, buyerID, participationID, AcceptParticipationParams{
		Delivery:                "leads_pipeline",
		BuyerPipelineID:         pipelineID,
		BuyerTargetStageID:      stageID,
		IntegrationConnectionID: connID,
	})
	if err != nil {
		t.Fatalf("UpdateParticipationDelivery: %v", err)
	}
	if part.IntegrationConnectionID == nil || *part.IntegrationConnectionID != connID {
		t.Fatalf("integration_connection_id = %v, want %d", part.IntegrationConnectionID, connID)
	}
}
