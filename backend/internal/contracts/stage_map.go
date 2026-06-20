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
		if err := rows.Scan(&r.buyerStageID, &r.returnStageID); err != nil {
			return nil, err
		}
		out = append(out, r)
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

func lookupPublisherStage(ctx context.Context, q database.Querier, contractID, buyerID, buyerStageID int64) (int64, int64, error) {
	var sourcePipelineID *int64
	var participationID *int64
	err := q.QueryRow(ctx,
		`SELECT COALESCE(p.source_pipeline_id, c.source_pipeline_id), p.id
		 FROM contracts c
		 LEFT JOIN contract_participations p
		   ON p.contract_id = c.id AND p.buyer_id = $2 AND p.status = 'active'
		 WHERE c.id = $1 AND c.deleted_at IS NULL
		 ORDER BY p.id NULLS LAST
		 LIMIT 1`,
		contractID, buyerID).Scan(&sourcePipelineID, &participationID)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, 0, httpx.NotFound("contract not found")
	}
	if err != nil {
		return 0, 0, err
	}
	if sourcePipelineID == nil || *sourcePipelineID == 0 {
		return 0, 0, httpx.BusinessRule("publisher pipeline is not configured on contract")
	}

	var pubStageID int64
	if participationID != nil {
		err = q.QueryRow(ctx,
			`SELECT publisher_stage_id FROM contract_stage_maps
			 WHERE contract_id = $1 AND participation_id = $2 AND buyer_stage_id = $3`,
			contractID, *participationID, buyerStageID).Scan(&pubStageID)
		if err == nil {
			return *sourcePipelineID, pubStageID, nil
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return 0, 0, err
		}
	}
	err = q.QueryRow(ctx,
		`SELECT publisher_stage_id FROM contract_stage_maps
		 WHERE contract_id = $1 AND participation_id IS NULL AND buyer_stage_id = $2`,
		contractID, buyerStageID).Scan(&pubStageID)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, 0, httpx.BusinessRule("contract stage map is not configured for this buyer stage")
	}
	if err != nil {
		return 0, 0, err
	}
	return *sourcePipelineID, pubStageID, nil
}

func contractDistributeStage(ctx context.Context, q database.Querier, contractID int64) (pubPipelineID, pubStageID int64, err error) {
	var sourcePipelineID, sourceStageID *int64
	err = q.QueryRow(ctx,
		`SELECT source_pipeline_id, source_stage_id FROM contracts WHERE id = $1 AND deleted_at IS NULL`,
		contractID).Scan(&sourcePipelineID, &sourceStageID)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, 0, httpx.NotFound("contract not found")
	}
	if err != nil {
		return 0, 0, err
	}
	if sourcePipelineID == nil || sourceStageID == nil || *sourcePipelineID == 0 || *sourceStageID == 0 {
		return 0, 0, httpx.BusinessRule("contract distribute stage is not configured")
	}
	return *sourcePipelineID, *sourceStageID, nil
}

func isStageMapMissingErr(err error) bool {
	var appErr *httpx.AppError
	return errors.As(err, &appErr) &&
		appErr.Code == httpx.CodeBusinessRule &&
		appErr.Message == "contract stage map is not configured for this buyer stage"
}

func setPublisherTracking(ctx context.Context, q database.Querier, leadID, pubPipelineID, pubStageID int64) error {
	_, err := q.Exec(ctx,
		`UPDATE leads SET publisher_pipeline_id = $2, publisher_stage_id = $3 WHERE id = $1`,
		leadID, pubPipelineID, pubStageID)
	return err
}

func clearPublisherTracking(ctx context.Context, q database.Querier, leadID int64) error {
	_, err := q.Exec(ctx,
		`UPDATE leads SET publisher_pipeline_id = NULL, publisher_stage_id = NULL WHERE id = $1`,
		leadID)
	return err
}

// InitPublisherTracking sets publisher-board placement when a lead is distributed to a buyer.
// When no stage map exists yet, falls back to the contract distribute-from stage (source_stage_id).
func InitPublisherTracking(ctx context.Context, q database.Querier, contractID, leadID, buyerID, buyerStageID int64) error {
	pubPipelineID, pubStageID, err := lookupPublisherStage(ctx, q, contractID, buyerID, buyerStageID)
	if err != nil && isStageMapMissingErr(err) {
		pubPipelineID, pubStageID, err = contractDistributeStage(ctx, q, contractID)
	}
	if err != nil {
		return err
	}
	return setPublisherTracking(ctx, q, leadID, pubPipelineID, pubStageID)
}

// SyncPublisherStage updates publisher-board placement only when an explicit delivery map exists.
// When no map exists, clears publisher tracking so distributed leads leave the publisher board.
func SyncPublisherStage(ctx context.Context, q database.Querier, contractID, leadID, buyerID, buyerStageID int64) error {
	pubPipelineID, pubStageID, err := lookupPublisherStage(ctx, q, contractID, buyerID, buyerStageID)
	if isStageMapMissingErr(err) {
		return clearPublisherTracking(ctx, q, leadID)
	}
	if err != nil {
		return err
	}
	if err := ValidateReturnDestination(ctx, q, pubPipelineID, pubStageID); err != nil {
		return nil
	}
	return setPublisherTracking(ctx, q, leadID, pubPipelineID, pubStageID)
}
