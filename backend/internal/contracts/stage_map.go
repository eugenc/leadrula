package contracts

import (
	"context"
	"errors"

	"github.com/echayko/leadrula/backend/internal/database"
	"github.com/echayko/leadrula/backend/pkg/httpx"
	"github.com/jackc/pgx/v5"
)

type returnRuleStage struct {
	buyerStageID  int64
	returnStageID int64
}

// RebuildStageMapParams optional overrides when rebuilding before the DB row is updated.
type RebuildStageMapParams struct {
	BuyerTargetStageID int64
}

func buildDeliveryStageMaps(buyerTargetStageID, sourceStageID int64, rules []returnRuleStage) map[int64]int64 {
	out := make(map[int64]int64)
	if buyerTargetStageID > 0 && sourceStageID > 0 {
		out[buyerTargetStageID] = sourceStageID
	}
	for _, r := range rules {
		if r.buyerStageID > 0 && r.returnStageID > 0 {
			out[r.buyerStageID] = r.returnStageID
		}
	}
	return out
}

func loadReturnRules(ctx context.Context, q database.Querier, contractID int64, participationID *int64) ([]returnRuleStage, error) {
	var rows pgx.Rows
	var err error
	if participationID == nil {
		rows, err = q.Query(ctx,
			`SELECT buyer_stage_id, return_stage_id FROM contract_return_rules
			 WHERE contract_id = $1 AND participation_id IS NULL`,
			contractID)
	} else {
		rows, err = q.Query(ctx,
			`SELECT buyer_stage_id, return_stage_id FROM contract_return_rules
			 WHERE contract_id = $1 AND participation_id = $2`,
			contractID, *participationID)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []returnRuleStage
	for rows.Next() {
		var r returnRuleStage
		var returnStageID *int64
		if err := rows.Scan(&r.buyerStageID, &returnStageID); err != nil {
			return nil, err
		}
		if returnStageID != nil && *returnStageID > 0 {
			r.returnStageID = *returnStageID
			out = append(out, r)
		}
	}
	return out, rows.Err()
}

func saveStageMaps(ctx context.Context, q database.Querier, contractID int64, participationID *int64, maps map[int64]int64) error {
	if participationID == nil {
		if _, err := q.Exec(ctx,
			`DELETE FROM contract_stage_maps WHERE contract_id = $1 AND participation_id IS NULL`, contractID); err != nil {
			return err
		}
	} else {
		if _, err := q.Exec(ctx,
			`DELETE FROM contract_stage_maps WHERE contract_id = $1 AND participation_id = $2`, contractID, *participationID); err != nil {
			return err
		}
	}
	for buyerStageID, pubStageID := range maps {
		if participationID == nil {
			if _, err := q.Exec(ctx,
				`INSERT INTO contract_stage_maps(contract_id, participation_id, buyer_stage_id, publisher_stage_id)
				 VALUES ($1, NULL, $2, $3)`,
				contractID, buyerStageID, pubStageID); err != nil {
				return err
			}
		} else {
			if _, err := q.Exec(ctx,
				`INSERT INTO contract_stage_maps(contract_id, participation_id, buyer_stage_id, publisher_stage_id)
				 VALUES ($1, $2, $3, $4)`,
				contractID, *participationID, buyerStageID, pubStageID); err != nil {
				return err
			}
		}
	}
	return nil
}

// RebuildContractStageMaps stores delivery-only buyer→publisher stage maps for a direct contract.
func RebuildContractStageMaps(ctx context.Context, q database.Querier, contractID int64, params ...RebuildStageMapParams) error {
	var overrides RebuildStageMapParams
	if len(params) > 0 {
		overrides = params[0]
	}

	var sourcePipelineID, sourceStageID, buyerPipelineID *int64
	var buyerTargetStageID *int64
	err := q.QueryRow(ctx,
		`SELECT c.source_pipeline_id, c.source_stage_id, c.buyer_pipeline_id,
		        cc.counterparty_stage_id
		 FROM contracts c
		 LEFT JOIN LATERAL (
		   SELECT counterparty_stage_id FROM contract_compensations
		   WHERE contract_id = c.id AND participation_id IS NULL
		   ORDER BY position, id LIMIT 1
		 ) cc ON true
		 WHERE c.id = $1 AND c.deleted_at IS NULL`,
		contractID).Scan(&sourcePipelineID, &sourceStageID, &buyerPipelineID, &buyerTargetStageID)
	if errors.Is(err, pgx.ErrNoRows) {
		return httpx.NotFound("contract not found")
	}
	if err != nil {
		return err
	}
	if sourcePipelineID == nil || *sourcePipelineID == 0 {
		_, err = q.Exec(ctx, `DELETE FROM contract_stage_maps WHERE contract_id = $1 AND participation_id IS NULL`, contractID)
		return err
	}

	buyerTarget := overrides.BuyerTargetStageID
	if buyerTarget == 0 {
		buyerTarget = derefInt64(buyerTargetStageID)
	}

	rules, err := loadReturnRules(ctx, q, contractID, nil)
	if err != nil {
		return err
	}
	maps := buildDeliveryStageMaps(buyerTarget, derefInt64(sourceStageID), rules)
	return saveStageMaps(ctx, q, contractID, nil, maps)
}

// RebuildParticipationStageMaps stores delivery-only maps for a participation's buyer pipeline.
func RebuildParticipationStageMaps(ctx context.Context, q database.Querier, contractID, participationID int64, params ...RebuildStageMapParams) error {
	var overrides RebuildStageMapParams
	if len(params) > 0 {
		overrides = params[0]
	}

	var sourcePipelineID, sourceStageID *int64
	var buyerPipelineID int64
	var buyerTargetStageID *int64
	err := q.QueryRow(ctx,
		`SELECT COALESCE(p.source_pipeline_id, c.source_pipeline_id),
		        COALESCE(p.source_stage_id, c.source_stage_id),
		        p.buyer_pipeline_id, p.buyer_target_stage_id
		 FROM contract_participations p
		 JOIN contracts c ON c.id = p.contract_id
		 WHERE p.id = $1 AND p.contract_id = $2`,
		participationID, contractID).Scan(&sourcePipelineID, &sourceStageID, &buyerPipelineID, &buyerTargetStageID)
	if errors.Is(err, pgx.ErrNoRows) {
		return httpx.NotFound("participation not found")
	}
	if err != nil {
		return err
	}
	if sourcePipelineID == nil || *sourcePipelineID == 0 || buyerPipelineID == 0 {
		_, err = q.Exec(ctx,
			`DELETE FROM contract_stage_maps WHERE contract_id = $1 AND participation_id = $2`, contractID, participationID)
		return err
	}

	buyerTarget := overrides.BuyerTargetStageID
	if buyerTarget == 0 {
		buyerTarget = derefInt64(buyerTargetStageID)
	}

	pid := participationID
	rules, err := loadReturnRules(ctx, q, contractID, &pid)
	if err != nil {
		return err
	}
	maps := buildDeliveryStageMaps(buyerTarget, derefInt64(sourceStageID), rules)
	return saveStageMaps(ctx, q, contractID, &pid, maps)
}

// RebuildActiveParticipationStageMaps rebuilds stage maps for all active participations on a contract.
func RebuildActiveParticipationStageMaps(ctx context.Context, q database.Querier, contractID int64) error {
	rows, err := q.Query(ctx,
		`SELECT id FROM contract_participations
		 WHERE contract_id = $1 AND status = 'active' AND buyer_pipeline_id > 0`,
		contractID)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var participationID int64
		if err := rows.Scan(&participationID); err != nil {
			return err
		}
		if err := RebuildParticipationStageMaps(ctx, q, contractID, participationID); err != nil {
			return err
		}
	}
	return rows.Err()
}

// RebuildAllActiveContractStageMaps rebuilds stage maps for active contracts and participations.
func RebuildAllActiveContractStageMaps(ctx context.Context, q database.Querier) error {
	rows, err := q.Query(ctx,
		`SELECT id FROM contracts
		 WHERE deleted_at IS NULL AND status = 'active'
		   AND source_pipeline_id IS NOT NULL AND source_pipeline_id > 0`)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var contractID int64
		if err := rows.Scan(&contractID); err != nil {
			return err
		}
		if err := RebuildContractStageMaps(ctx, q, contractID); err != nil {
			return err
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}

	pRows, err := q.Query(ctx,
		`SELECT contract_id, id FROM contract_participations
		 WHERE status = 'active' AND buyer_pipeline_id > 0`)
	if err != nil {
		return err
	}
	defer pRows.Close()
	for pRows.Next() {
		var contractID, participationID int64
		if err := pRows.Scan(&contractID, &participationID); err != nil {
			return err
		}
		if err := RebuildParticipationStageMaps(ctx, q, contractID, participationID); err != nil {
			return err
		}
	}
	return pRows.Err()
}

// SyncPublisherStageWithRebuild updates publisher-board placement when a mapped buyer stage exists.
func SyncPublisherStageWithRebuild(ctx context.Context, q database.Querier, contractID, leadID, buyerID, buyerStageID int64) error {
	return SyncPublisherStage(ctx, q, contractID, leadID, buyerID, buyerStageID)
}

// ClearPublisherTracking removes publisher-board placement so a distributed lead lives only on the buyer board.
func ClearPublisherTracking(ctx context.Context, q database.Querier, leadID int64) error {
	_, err := q.Exec(ctx,
		`UPDATE leads SET publisher_pipeline_id = NULL, publisher_stage_id = NULL WHERE id = $1`,
		leadID)
	return err
}

// SyncPublisherStage clears publisher-board tracking when a buyer-owned lead changes stage.
// Distributed leads live only on the buyer board; they never appear on the publisher pipeline
// until a return rule hands ownership back via MoveToPublisher.
func SyncPublisherStage(ctx context.Context, q database.Querier, contractID, leadID, buyerID, buyerStageID int64) error {
	return ClearPublisherTracking(ctx, q, leadID)
}
