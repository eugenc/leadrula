package contracts

import (
	"context"
	"errors"
	"testing"

	"github.com/echayko/leadrula/backend/internal/config"
	"github.com/echayko/leadrula/backend/internal/database"
	"github.com/echayko/leadrula/backend/pkg/httpx"
	"github.com/jackc/pgx/v5/pgxpool"
)

func connectContractsTestDB(t *testing.T) *pgxpool.Pool {
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

func TestValidateContractReturnRoutesRequired_missingRules(t *testing.T) {
	pool := connectContractsTestDB(t)
	ctx := context.Background()

	var contractID, buyerPipelineID int64
	err := pool.QueryRow(ctx,
		`SELECT id, buyer_pipeline_id FROM contracts
		 WHERE buyer_pipeline_id IS NOT NULL AND return_stage_id IS NOT NULL
		   AND buyer_id IS NOT NULL AND deleted_at IS NULL
		 LIMIT 1`).Scan(&contractID, &buyerPipelineID)
	if err != nil {
		t.Skip("no direct pipeline contract in database")
	}

	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx,
		`DELETE FROM contract_return_rules WHERE contract_id = $1 AND participation_id IS NULL`,
		contractID); err != nil {
		t.Fatal(err)
	}

	err = validateContractReturnRoutesRequired(ctx, tx, contractID, buyerPipelineID, true)
	if err == nil {
		t.Fatal("expected validation error when no contract return routes")
	}
	var appErr *httpx.AppError
	if !errors.As(err, &appErr) || appErr.Code != httpx.CodeValidation {
		t.Fatalf("expected validation error, got %v", err)
	}
}

func TestValidateContractReturnRoutesRequired_withRules(t *testing.T) {
	pool := connectContractsTestDB(t)
	ctx := context.Background()

	var contractID, buyerPipelineID, buyerStageID, returnStageID int64
	err := pool.QueryRow(ctx,
		`SELECT c.id, c.buyer_pipeline_id, c.return_stage_id,
		        (SELECT ps.id FROM pipeline_stages ps WHERE ps.pipeline_id = c.buyer_pipeline_id ORDER BY ps.position, ps.id LIMIT 1)
		 FROM contracts c
		 WHERE c.buyer_pipeline_id IS NOT NULL AND c.return_stage_id IS NOT NULL
		   AND c.buyer_id IS NOT NULL AND c.deleted_at IS NULL
		 LIMIT 1`).Scan(&contractID, &buyerPipelineID, &returnStageID, &buyerStageID)
	if err != nil {
		t.Skip("no direct pipeline contract in database")
	}
	if buyerStageID == 0 {
		t.Skip("contract buyer pipeline has no stages")
	}

	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx,
		`DELETE FROM contract_return_rules WHERE contract_id = $1 AND participation_id IS NULL`,
		contractID); err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(ctx,
		`INSERT INTO contract_return_rules(contract_id, buyer_stage_id, return_stage_id)
		 VALUES ($1, $2, $3)`,
		contractID, buyerStageID, returnStageID); err != nil {
		t.Fatal(err)
	}

	if err := validateContractReturnRoutesRequired(ctx, tx, contractID, buyerPipelineID, true); err != nil {
		t.Fatalf("expected success with return route, got %v", err)
	}
}

func TestDeleteReturnRule_blocksLastContractRule(t *testing.T) {
	pool := connectContractsTestDB(t)
	ctx := context.Background()
	svc := NewService(pool)

	var contractID int64
	err := pool.QueryRow(ctx,
		`SELECT c.id FROM contracts c
		 WHERE c.buyer_pipeline_id IS NOT NULL AND c.return_stage_id IS NOT NULL
		   AND c.buyer_id IS NOT NULL AND c.deleted_at IS NULL
		   AND EXISTS (
		     SELECT 1 FROM contract_return_rules rr
		     WHERE rr.contract_id = c.id AND rr.participation_id IS NULL
		   )
		 LIMIT 1`).Scan(&contractID)
	if err != nil {
		t.Skip("no direct pipeline contract with return rules")
	}

	required, err := contractRequiresReturnRoutes(ctx, pool, contractID)
	if err != nil {
		t.Fatal(err)
	}
	if !required {
		t.Skip("contract does not require return routes")
	}

	var ruleID int64
	err = pool.QueryRow(ctx,
		`SELECT id FROM contract_return_rules
		 WHERE contract_id = $1 AND participation_id IS NULL
		 ORDER BY id LIMIT 1`, contractID).Scan(&ruleID)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := pool.Exec(ctx,
		`DELETE FROM contract_return_rules
		 WHERE contract_id = $1 AND participation_id IS NULL AND id <> $2`,
		contractID, ruleID); err != nil {
		t.Fatal(err)
	}

	err = svc.DeleteReturnRule(ctx, ruleID)
	if err == nil {
		t.Fatal("expected error deleting last contract return route")
	}
	var appErr *httpx.AppError
	if !errors.As(err, &appErr) || appErr.Code != httpx.CodeValidation {
		t.Fatalf("expected validation error, got %v", err)
	}
}

func TestAddBuyerContractReturnRule_createsPendingRoute(t *testing.T) {
	pool := connectContractsTestDB(t)
	ctx := context.Background()
	svc := NewService(pool)

	var buyerID, contractID, buyerPipelineID, buyerStageID int64
	err := pool.QueryRow(ctx,
		`SELECT c.buyer_id, c.id, c.buyer_pipeline_id,
		        (SELECT ps.id FROM pipeline_stages ps WHERE ps.pipeline_id = c.buyer_pipeline_id ORDER BY ps.position, ps.id LIMIT 1)
		 FROM contracts c
		 WHERE c.buyer_id IS NOT NULL AND c.buyer_pipeline_id IS NOT NULL
		   AND c.source_pipeline_id IS NOT NULL
		   AND c.deleted_at IS NULL AND c.contract_type = 'sell'
		 LIMIT 1`).Scan(&buyerID, &contractID, &buyerPipelineID, &buyerStageID)
	if err != nil {
		t.Skip("no direct buyer contract in database")
	}
	if buyerStageID == 0 {
		t.Skip("contract buyer pipeline has no stages")
	}

	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx,
		`DELETE FROM contract_return_rules WHERE contract_id = $1 AND participation_id IS NULL AND buyer_stage_id = $2`,
		contractID, buyerStageID); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}

	rr, err := svc.AddBuyerContractReturnRule(ctx, buyerID, contractID, buyerStageID, buyerPipelineID, ReturnSchedulePatch{}, false, strPtr("No shows"))
	if err != nil {
		t.Fatalf("AddBuyerContractReturnRule: %v", err)
	}
	if rr.Label != "No shows" {
		t.Fatalf("label = %q, want No shows", rr.Label)
	}
	if rr.ReturnStageID != nil {
		t.Fatalf("return_stage_id = %v, want pending (nil)", rr.ReturnStageID)
	}
	if rr.ParticipationID != nil {
		t.Fatalf("expected contract-level rule, got participation_id %v", rr.ParticipationID)
	}

	if err := svc.DeleteBuyerContractReturnRule(ctx, buyerID, contractID, rr.ID); err != nil {
		t.Fatalf("DeleteBuyerContractReturnRule: %v", err)
	}
}

func TestActivateOfferDraft_allowsMissingContractReturnRoutes(t *testing.T) {
	pool := connectContractsTestDB(t)
	ctx := context.Background()
	svc := NewService(pool)

	var publisherID, contractID int64
	var modes []string
	err := pool.QueryRow(ctx,
		`SELECT publisher_id, id, allowed_delivery_modes FROM contracts
		 WHERE status = 'draft' AND buyer_id IS NULL AND deleted_at IS NULL
		   AND 'leads_pipeline' = ANY(allowed_delivery_modes)
		   AND source_pipeline_id IS NOT NULL AND return_stage_id IS NOT NULL
		 LIMIT 1`).Scan(&publisherID, &contractID, &modes)
	if err != nil {
		t.Skip("no open offer draft with pipeline delivery")
	}

	if _, err := pool.Exec(ctx,
		`DELETE FROM contract_return_rules WHERE contract_id = $1 AND participation_id IS NULL`,
		contractID); err != nil {
		t.Fatal(err)
	}

	c, err := svc.Get(ctx, publisherID, contractID)
	if err != nil {
		t.Fatal(err)
	}

	_, err = svc.ActivateOfferDraft(ctx, publisherID, contractID, CreateParams{
		Name:                 c.Name,
		ContractType:         c.ContractType,
		LeadType:             c.LeadType,
		AllowedDeliveryModes: modes,
		DistributionStrategy: c.DistributionStrategy,
		SourcePipelineID:     derefInt64(c.SourcePipelineID),
		SourceStageID:        derefInt64(c.SourceStageID),
		ReturnStageID:        derefInt64(c.ReturnStageID),
		LeadCriteria: &LeadCriteria{
			RequiredFields: []RequiredField{
				{FieldType: "builtin", BuiltinField: "first_name"},
				{FieldType: "builtin", BuiltinField: "phone"},
			},
		},
	})
	if err != nil {
		t.Fatalf("open offer activation should not require contract return routes: %v", err)
	}

	_, _ = pool.Exec(ctx, `UPDATE contracts SET status = 'draft' WHERE id = $1`, contractID)
}

func TestListContractParticipationReturnRules_andUpdateDestination(t *testing.T) {
	pool := connectContractsTestDB(t)
	ctx := context.Background()
	svc := NewService(pool)

	var publisherID, contractID, participationID, buyerStageID, defaultReturnStageID, altReturnStageID int64
	err := pool.QueryRow(ctx,
		`SELECT c.publisher_id, c.id, rr.participation_id, rr.buyer_stage_id, rr.return_stage_id,
		        (SELECT ps2.id FROM pipeline_stages ps2
		         WHERE ps2.pipeline_id = c.source_pipeline_id AND ps2.id <> rr.return_stage_id
		         ORDER BY ps2.position, ps2.id LIMIT 1)
		 FROM contract_return_rules rr
		 JOIN contracts c ON c.id = rr.contract_id
		 WHERE rr.participation_id IS NOT NULL AND c.deleted_at IS NULL
		   AND c.source_pipeline_id IS NOT NULL
		 LIMIT 1`).Scan(
		&publisherID, &contractID, &participationID, &buyerStageID, &defaultReturnStageID, &altReturnStageID)
	if err != nil {
		t.Skip("no participation return route in database")
	}
	if altReturnStageID == 0 {
		t.Skip("contract publisher pipeline has only one stage")
	}

	rules, err := svc.ListContractParticipationReturnRules(ctx, publisherID, contractID)
	if err != nil {
		t.Fatalf("ListContractParticipationReturnRules: %v", err)
	}
	if len(rules) == 0 {
		t.Fatal("expected at least one participation return rule")
	}
	found := false
	var ruleID int64
	for _, rr := range rules {
		if rr.ParticipationID != nil && *rr.ParticipationID == participationID && rr.BuyerStageID == buyerStageID {
			found = true
			ruleID = rr.ID
			break
		}
	}
	if !found {
		t.Fatal("expected participation return rule in list")
	}

	updated, err := svc.UpdateParticipationReturnRuleDestination(ctx, publisherID, ruleID, altReturnStageID, strPtr("Publisher label"), ReturnSchedulePatch{}, false)
	if err != nil {
		t.Fatalf("UpdateParticipationReturnRuleDestination: %v", err)
	}
	if updated.Label != "Publisher label" {
		t.Fatalf("label = %q, want Publisher label", updated.Label)
	}
	if updated.ReturnStageID == nil || *updated.ReturnStageID != altReturnStageID {
		t.Fatalf("return_stage_id = %v, want %d", updated.ReturnStageID, altReturnStageID)
	}
	if updated.BuyerStageID != buyerStageID {
		t.Fatalf("buyer_stage_id changed from %d to %d", buyerStageID, updated.BuyerStageID)
	}

	_, err = svc.UpdateParticipationReturnRuleDestination(ctx, publisherID, ruleID, defaultReturnStageID, nil, ReturnSchedulePatch{}, false)
	if err != nil {
		t.Fatalf("restore return_stage_id: %v", err)
	}
}

func TestListReturnRulesForPublisher_andUpdateContractDestination(t *testing.T) {
	pool := connectContractsTestDB(t)
	ctx := context.Background()
	svc := NewService(pool)

	var publisherID, contractID, buyerStageID, defaultReturnStageID, altReturnStageID int64
	err := pool.QueryRow(ctx,
		`SELECT c.publisher_id, c.id, rr.buyer_stage_id, rr.return_stage_id,
		        (SELECT ps2.id FROM pipeline_stages ps2
		         WHERE ps2.pipeline_id = c.source_pipeline_id AND ps2.id <> rr.return_stage_id
		         ORDER BY ps2.position, ps2.id LIMIT 1)
		 FROM contract_return_rules rr
		 JOIN contracts c ON c.id = rr.contract_id
		 WHERE rr.participation_id IS NULL AND c.deleted_at IS NULL
		   AND c.source_pipeline_id IS NOT NULL AND c.buyer_id IS NOT NULL
		 LIMIT 1`).Scan(
		&publisherID, &contractID, &buyerStageID, &defaultReturnStageID, &altReturnStageID)
	if err != nil {
		t.Skip("no direct contract return route in database")
	}
	if altReturnStageID == 0 {
		t.Skip("contract publisher pipeline has only one stage")
	}

	rules, err := svc.ListReturnRulesForPublisher(ctx, publisherID, contractID)
	if err != nil {
		t.Fatalf("ListReturnRulesForPublisher: %v", err)
	}
	if len(rules) == 0 {
		t.Fatal("expected at least one contract return rule")
	}
	found := false
	var ruleID int64
	for _, rr := range rules {
		if rr.BuyerStageID == buyerStageID {
			found = true
			ruleID = rr.ID
			break
		}
	}
	if !found {
		t.Fatal("expected contract return rule in list")
	}

	updated, err := svc.UpdateContractReturnRuleDestination(ctx, publisherID, ruleID, altReturnStageID, strPtr("Contract label"), ReturnSchedulePatch{}, false)
	if err != nil {
		t.Fatalf("UpdateContractReturnRuleDestination: %v", err)
	}
	if updated.Label != "Contract label" {
		t.Fatalf("label = %q, want Contract label", updated.Label)
	}
	if updated.ReturnStageID == nil || *updated.ReturnStageID != altReturnStageID {
		t.Fatalf("return_stage_id = %v, want %d", updated.ReturnStageID, altReturnStageID)
	}

	_, err = svc.UpdateContractReturnRuleDestination(ctx, publisherID, ruleID, defaultReturnStageID, nil, ReturnSchedulePatch{}, false)
	if err != nil {
		t.Fatalf("restore return_stage_id: %v", err)
	}
}

func TestFindReturnRule_pendingRouteDoesNotMatch(t *testing.T) {
	pool := connectContractsTestDB(t)
	ctx := context.Background()

	var buyerID, contractID, buyerStageID int64
	err := pool.QueryRow(ctx,
		`SELECT c.buyer_id, c.id,
		        (SELECT ps.id FROM pipeline_stages ps WHERE ps.pipeline_id = c.buyer_pipeline_id ORDER BY ps.position, ps.id LIMIT 1)
		 FROM contracts c
		 WHERE c.buyer_id IS NOT NULL AND c.buyer_pipeline_id IS NOT NULL
		   AND c.source_pipeline_id IS NOT NULL
		   AND c.deleted_at IS NULL AND c.contract_type = 'sell'
		 LIMIT 1`).Scan(&buyerID, &contractID, &buyerStageID)
	if err != nil {
		t.Skip("no direct buyer contract in database")
	}
	if buyerStageID == 0 {
		t.Skip("contract buyer pipeline has no stages")
	}

	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx,
		`DELETE FROM contract_return_rules WHERE contract_id = $1 AND participation_id IS NULL AND buyer_stage_id = $2`,
		contractID, buyerStageID); err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(ctx,
		`INSERT INTO contract_return_rules(contract_id, buyer_stage_id, return_stage_id)
		 VALUES ($1, $2, NULL)`,
		contractID, buyerStageID); err != nil {
		t.Fatal(err)
	}

	info, err := FindReturnRule(ctx, tx, contractID, buyerID, buyerStageID)
	if err != nil {
		t.Fatalf("FindReturnRule: %v", err)
	}
	if info != nil {
		t.Fatalf("expected no return for pending route, got %+v", info)
	}
}

func TestFindReturnRule_prefersRuleDestinationOverParticipationDefault(t *testing.T) {
	pool := connectContractsTestDB(t)
	ctx := context.Background()

	var contractID, buyerID, participationID, buyerStageID, ruleReturnStageID int64
	var participationReturnStageID *int64
	var altParticipationDefault int64
	err := pool.QueryRow(ctx,
		`SELECT c.id, p.buyer_id, rr.participation_id, rr.buyer_stage_id, rr.return_stage_id, p.return_stage_id,
		        (SELECT ps.id FROM pipeline_stages ps
		         WHERE ps.pipeline_id = c.source_pipeline_id AND ps.id <> rr.return_stage_id
		         ORDER BY ps.position, ps.id LIMIT 1)
		 FROM contract_return_rules rr
		 JOIN contract_participations p ON p.id = rr.participation_id
		 JOIN contracts c ON c.id = rr.contract_id
		 WHERE rr.participation_id IS NOT NULL AND c.deleted_at IS NULL
		   AND c.source_pipeline_id IS NOT NULL
		 LIMIT 1`).Scan(
		&contractID, &buyerID, &participationID, &buyerStageID, &ruleReturnStageID,
		&participationReturnStageID, &altParticipationDefault)
	if err != nil {
		t.Skip("no participation return rule in database")
	}
	if altParticipationDefault == 0 {
		t.Skip("contract publisher pipeline has only one stage")
	}

	origParticipationReturn := participationReturnStageID
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx,
			`UPDATE contract_participations SET return_stage_id = $2 WHERE id = $1`,
			participationID, origParticipationReturn)
	})

	if _, err := pool.Exec(ctx,
		`UPDATE contract_participations SET return_stage_id = $2 WHERE id = $1`,
		participationID, altParticipationDefault); err != nil {
		t.Fatal(err)
	}

	info, err := FindReturnRule(ctx, pool, contractID, buyerID, buyerStageID)
	if err != nil {
		t.Fatalf("FindReturnRule: %v", err)
	}
	if info == nil {
		t.Fatal("expected return rule match")
	}
	if info.ReturnStageID != ruleReturnStageID {
		t.Fatalf("ReturnStageID = %d, want rule destination %d (not participation default %d)",
			info.ReturnStageID, ruleReturnStageID, altParticipationDefault)
	}
}

func TestUpdateDelivery_preservesBuyerPipeline(t *testing.T) {
	pool := connectContractsTestDB(t)
	ctx := context.Background()
	svc := NewService(pool)

	var publisherID, contractID, buyerPipelineID int64
	var sourcePipelineID, sourceStageID int64
	err := pool.QueryRow(ctx,
		`SELECT c.publisher_id, c.id, c.buyer_pipeline_id, c.source_pipeline_id, c.source_stage_id
		 FROM contracts c
		 WHERE c.buyer_id IS NOT NULL AND c.buyer_pipeline_id IS NOT NULL
		   AND c.source_pipeline_id IS NOT NULL AND c.source_stage_id IS NOT NULL
		   AND c.deleted_at IS NULL
		 LIMIT 1`).Scan(&publisherID, &contractID, &buyerPipelineID, &sourcePipelineID, &sourceStageID)
	if err != nil {
		t.Skip("no direct pipeline contract with buyer pipeline in database")
	}

	updated, err := svc.UpdateDelivery(ctx, publisherID, contractID, DeliveryUpdateParams{
		Delivery:         "leads_pipeline",
		SourcePipelineID: sourcePipelineID,
		SourceStageID:    sourceStageID,
		BuyerPipelineID:  0,
	})
	if err != nil {
		t.Fatalf("UpdateDelivery: %v", err)
	}
	if updated.BuyerPipelineID == nil || *updated.BuyerPipelineID != buyerPipelineID {
		t.Fatalf("buyer_pipeline_id = %v, want %d preserved", updated.BuyerPipelineID, buyerPipelineID)
	}
}

func TestFindReturnRule_ignoresRuleOnWrongBuyerPipeline(t *testing.T) {
	pool := connectContractsTestDB(t)
	ctx := context.Background()

	var buyerID, contractID, buyerPipelineID, returnStageID int64
	var buyerStageOnContract int64
	err := pool.QueryRow(ctx,
		`SELECT c.buyer_id, c.id, c.buyer_pipeline_id, c.return_stage_id,
		        (SELECT ps.id FROM pipeline_stages ps
		         WHERE ps.pipeline_id = c.buyer_pipeline_id
		         ORDER BY ps.position, ps.id LIMIT 1)
		 FROM contracts c
		 WHERE c.buyer_id IS NOT NULL AND c.buyer_pipeline_id IS NOT NULL
		   AND c.return_stage_id IS NOT NULL AND c.source_pipeline_id IS NOT NULL
		   AND c.deleted_at IS NULL AND c.contract_type = 'sell'
		 LIMIT 1`).Scan(&buyerID, &contractID, &buyerPipelineID, &returnStageID, &buyerStageOnContract)
	if err != nil {
		t.Skip("no direct buyer contract in database")
	}

	var otherPipelineID int64
	err = pool.QueryRow(ctx,
		`SELECT p.id FROM pipelines p
		 WHERE p.account_id = $1 AND p.id <> $2
		 ORDER BY p.id LIMIT 1`, buyerID, buyerPipelineID).Scan(&otherPipelineID)
	if err != nil {
		t.Skip("buyer has no alternate pipeline")
	}

	var otherStageID int64
	err = pool.QueryRow(ctx,
		`SELECT id FROM pipeline_stages WHERE pipeline_id = $1 ORDER BY position, id LIMIT 1`,
		otherPipelineID).Scan(&otherStageID)
	if err != nil {
		t.Skip("alternate pipeline has no stages")
	}

	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)

	var ruleID int64
	err = tx.QueryRow(ctx,
		`INSERT INTO contract_return_rules(contract_id, buyer_stage_id, return_stage_id)
		 VALUES ($1,$2,$3)
		 ON CONFLICT (contract_id, buyer_stage_id) WHERE participation_id IS NULL
		 DO UPDATE SET return_stage_id = EXCLUDED.return_stage_id
		 RETURNING id`,
		contractID, otherStageID, returnStageID).Scan(&ruleID)
	if err != nil {
		t.Fatal(err)
	}

	info, err := FindReturnRule(ctx, tx, contractID, buyerID, otherStageID)
	if err != nil {
		t.Fatalf("FindReturnRule: %v", err)
	}
	if info != nil {
		t.Fatalf("expected no return for stage on wrong pipeline, got %+v", info)
	}

	info, err = FindReturnRule(ctx, tx, contractID, buyerID, buyerStageOnContract)
	if err != nil {
		t.Fatalf("FindReturnRule on contract pipeline: %v", err)
	}
	// May be nil if no rule for that stage — only asserting wrong-pipeline rule does not match.
	_ = info
}

func TestDeleteContractReturnRulesOffPipeline(t *testing.T) {
	pool := connectContractsTestDB(t)
	ctx := context.Background()

	var contractID, buyerPipelineID, otherPipelineID int64
	var stageOnContract, stageOnOther int64
	err := pool.QueryRow(ctx,
		`SELECT c.id, c.buyer_pipeline_id, p.id,
		        (SELECT ps.id FROM pipeline_stages ps WHERE ps.pipeline_id = c.buyer_pipeline_id ORDER BY ps.position, ps.id LIMIT 1),
		        (SELECT ps.id FROM pipeline_stages ps WHERE ps.pipeline_id = p.id ORDER BY ps.position, ps.id LIMIT 1)
		 FROM contracts c
		 JOIN pipelines p ON p.account_id = c.buyer_id AND p.id <> c.buyer_pipeline_id
		 WHERE c.buyer_id IS NOT NULL AND c.buyer_pipeline_id IS NOT NULL
		   AND c.return_stage_id IS NOT NULL AND c.deleted_at IS NULL
		 LIMIT 1`).Scan(&contractID, &buyerPipelineID, &otherPipelineID, &stageOnContract, &stageOnOther)
	if err != nil {
		t.Skip("no contract with two buyer pipelines in database")
	}

	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)

	var ruleID int64
	err = tx.QueryRow(ctx,
		`INSERT INTO contract_return_rules(contract_id, buyer_stage_id, return_stage_id)
		 VALUES ($1,$2,(SELECT return_stage_id FROM contracts WHERE id = $1))
		 RETURNING id`,
		contractID, stageOnOther).Scan(&ruleID)
	if err != nil {
		t.Fatal(err)
	}

	if err := deleteContractReturnRulesOffPipeline(ctx, tx, contractID, buyerPipelineID); err != nil {
		t.Fatal(err)
	}

	var remaining int
	if err := tx.QueryRow(ctx,
		`SELECT COUNT(*) FROM contract_return_rules WHERE id = $1`, ruleID).Scan(&remaining); err != nil {
		t.Fatal(err)
	}
	if remaining != 0 {
		t.Fatalf("expected off-pipeline rule deleted, still exists")
	}

	// Rule on contract pipeline should survive if we insert one.
	err = tx.QueryRow(ctx,
		`INSERT INTO contract_return_rules(contract_id, buyer_stage_id, return_stage_id)
		 VALUES ($1,$2,(SELECT return_stage_id FROM contracts WHERE id = $1))
		 ON CONFLICT (contract_id, buyer_stage_id) WHERE participation_id IS NULL
		 DO UPDATE SET return_stage_id = EXCLUDED.return_stage_id
		 RETURNING id`,
		contractID, stageOnContract).Scan(&ruleID)
	if err != nil {
		t.Fatal(err)
	}
	if err := deleteContractReturnRulesOffPipeline(ctx, tx, contractID, buyerPipelineID); err != nil {
		t.Fatal(err)
	}
	if err := tx.QueryRow(ctx,
		`SELECT COUNT(*) FROM contract_return_rules WHERE id = $1`, ruleID).Scan(&remaining); err != nil {
		t.Fatal(err)
	}
	if remaining != 1 {
		t.Fatalf("expected on-pipeline rule kept, count = %d", remaining)
	}
}

func TestListReturnRules_marksStaleWhenStageOffPipeline(t *testing.T) {
	pool := connectContractsTestDB(t)
	ctx := context.Background()
	svc := NewService(pool)

	var contractID, buyerPipelineID, otherPipelineID int64
	var stageOnOther int64
	err := pool.QueryRow(ctx,
		`SELECT c.id, c.buyer_pipeline_id, p.id,
		        (SELECT ps.id FROM pipeline_stages ps WHERE ps.pipeline_id = p.id ORDER BY ps.position, ps.id LIMIT 1)
		 FROM contracts c
		 JOIN pipelines p ON p.account_id = c.buyer_id AND p.id <> c.buyer_pipeline_id
		 WHERE c.buyer_id IS NOT NULL AND c.buyer_pipeline_id IS NOT NULL
		   AND c.return_stage_id IS NOT NULL AND c.deleted_at IS NULL
		 LIMIT 1`).Scan(&contractID, &buyerPipelineID, &otherPipelineID, &stageOnOther)
	if err != nil {
		t.Skip("no contract with two buyer pipelines in database")
	}

	var ruleID int64
	err = pool.QueryRow(ctx,
		`INSERT INTO contract_return_rules(contract_id, buyer_stage_id, return_stage_id)
		 VALUES ($1,$2,(SELECT return_stage_id FROM contracts WHERE id = $1))
		 ON CONFLICT (contract_id, buyer_stage_id) WHERE participation_id IS NULL
		 DO UPDATE SET return_stage_id = EXCLUDED.return_stage_id
		 RETURNING id`,
		contractID, stageOnOther).Scan(&ruleID)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, `DELETE FROM contract_return_rules WHERE id = $1`, ruleID)
	})

	rules, err := svc.ListReturnRules(ctx, contractID)
	if err != nil {
		t.Fatal(err)
	}
	var found *ReturnRule
	for i := range rules {
		if rules[i].ID == ruleID {
			found = &rules[i]
			break
		}
	}
	if found == nil {
		t.Fatal("inserted return rule not listed")
	}
	if !found.Stale {
		t.Fatal("expected stale=true for return rule on wrong buyer pipeline")
	}
	if found.BuyerPipelineName == "" {
		t.Fatal("expected buyer_pipeline_name on enriched return rule")
	}
	_ = buyerPipelineID
}

