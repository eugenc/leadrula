package contracts

import (
	"context"
	"errors"

	"github.com/echayko/leadrula/backend/internal/database"
	"github.com/echayko/leadrula/backend/pkg/httpx"
	"github.com/jackc/pgx/v5"
)

type stageRow struct {
	id        int64
	stageType string
}

func loadPipelineStages(ctx context.Context, q database.Querier, pipelineID int64) ([]stageRow, error) {
	if pipelineID == 0 {
		return nil, nil
	}
	rows, err := q.Query(ctx,
		`SELECT id, stage_type FROM pipeline_stages WHERE pipeline_id = $1 ORDER BY position, id`,
		pipelineID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []stageRow
	for rows.Next() {
		var s stageRow
		if err := rows.Scan(&s.id, &s.stageType); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

func publisherStageByType(stages []stageRow, stageType string) (int64, bool) {
	for _, s := range stages {
		if s.stageType == stageType {
			return s.id, true
		}
	}
	return 0, false
}

func buildStageMaps(buyerStages, pubStages []stageRow) (map[int64]int64, error) {
	if len(buyerStages) == 0 {
		return nil, httpx.Validation("buyer pipeline has no stages")
	}
	if len(pubStages) == 0 {
		return nil, httpx.Validation("publisher pipeline has no stages")
	}
	pubWon, hasPubWon := publisherStageByType(pubStages, "won")
	buyerWon, hasBuyerWon := publisherStageByType(buyerStages, "won")
	if !hasPubWon || !hasBuyerWon {
		return nil, httpx.Validation("publisher and buyer pipelines must each have a Won stage for stage sync")
	}

	out := make(map[int64]int64, len(buyerStages))
	if len(buyerStages) == len(pubStages) {
		for i, bs := range buyerStages {
			out[bs.id] = pubStages[i].id
		}
	} else {
		pubByType := map[string][]int64{}
		for _, ps := range pubStages {
			pubByType[ps.stageType] = append(pubByType[ps.stageType], ps.id)
		}
		used := map[int64]bool{}
		for _, bs := range buyerStages {
			candidates := pubByType[bs.stageType]
			var pubStageID int64
			switch len(candidates) {
			case 0:
				return nil, httpx.Validation("buyer and publisher pipelines must have matching stage counts or matching stage types for sync")
			case 1:
				pubStageID = candidates[0]
			default:
				for _, id := range candidates {
					if !used[id] {
						pubStageID = id
						break
					}
				}
				if pubStageID == 0 {
					pubStageID = candidates[0]
				}
			}
			out[bs.id] = pubStageID
			used[pubStageID] = true
		}
	}
	out[buyerWon] = pubWon
	return out, nil
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

// RebuildContractStageMaps rebuilds buyer→publisher stage maps for a direct contract.
func RebuildContractStageMaps(ctx context.Context, q database.Querier, contractID int64) error {
	var sourcePipelineID, buyerPipelineID *int64
	err := q.QueryRow(ctx,
		`SELECT source_pipeline_id, buyer_pipeline_id FROM contracts WHERE id = $1 AND deleted_at IS NULL`,
		contractID).Scan(&sourcePipelineID, &buyerPipelineID)
	if errors.Is(err, pgx.ErrNoRows) {
		return httpx.NotFound("contract not found")
	}
	if err != nil {
		return err
	}
	if sourcePipelineID == nil || buyerPipelineID == nil || *sourcePipelineID == 0 || *buyerPipelineID == 0 {
		_, err = q.Exec(ctx, `DELETE FROM contract_stage_maps WHERE contract_id = $1 AND participation_id IS NULL`, contractID)
		return err
	}
	buyerStages, err := loadPipelineStages(ctx, q, *buyerPipelineID)
	if err != nil {
		return err
	}
	pubStages, err := loadPipelineStages(ctx, q, *sourcePipelineID)
	if err != nil {
		return err
	}
	maps, err := buildStageMaps(buyerStages, pubStages)
	if err != nil {
		_, clearErr := q.Exec(ctx, `DELETE FROM contract_stage_maps WHERE contract_id = $1 AND participation_id IS NULL`, contractID)
		return clearErr
	}
	return saveStageMaps(ctx, q, contractID, nil, maps)
}

// RebuildParticipationStageMaps rebuilds maps for a participation's buyer pipeline.
func RebuildParticipationStageMaps(ctx context.Context, q database.Querier, contractID, participationID int64) error {
	var sourcePipelineID *int64
	var buyerPipelineID int64
	err := q.QueryRow(ctx,
		`SELECT COALESCE(p.source_pipeline_id, c.source_pipeline_id), p.buyer_pipeline_id
		 FROM contract_participations p
		 JOIN contracts c ON c.id = p.contract_id
		 WHERE p.id = $1 AND p.contract_id = $2`,
		participationID, contractID).Scan(&sourcePipelineID, &buyerPipelineID)
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
	buyerStages, err := loadPipelineStages(ctx, q, buyerPipelineID)
	if err != nil {
		return err
	}
	pubStages, err := loadPipelineStages(ctx, q, *sourcePipelineID)
	if err != nil {
		return err
	}
	maps, err := buildStageMaps(buyerStages, pubStages)
	if err != nil {
		_, clearErr := q.Exec(ctx,
			`DELETE FROM contract_stage_maps WHERE contract_id = $1 AND participation_id = $2`, contractID, participationID)
		return clearErr
	}
	pid := participationID
	return saveStageMaps(ctx, q, contractID, &pid, maps)
}

// RebuildAllActiveContractStageMaps rebuilds stage maps for active contracts and participations.
func RebuildAllActiveContractStageMaps(ctx context.Context, q database.Querier) error {
	rows, err := q.Query(ctx,
		`SELECT id FROM contracts
		 WHERE deleted_at IS NULL AND status = 'active'
		   AND source_pipeline_id IS NOT NULL AND buyer_pipeline_id IS NOT NULL
		   AND source_pipeline_id > 0 AND buyer_pipeline_id > 0`)
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

// SyncPublisherStageWithRebuild updates publisher-board placement, rebuilding stage maps once if missing.
func SyncPublisherStageWithRebuild(ctx context.Context, q database.Querier, contractID, leadID, buyerID, buyerStageID int64) error {
	err := SyncPublisherStage(ctx, q, contractID, leadID, buyerID, buyerStageID)
	if err == nil {
		return nil
	}
	var appErr *httpx.AppError
	if !errors.As(err, &appErr) || appErr.Code != httpx.CodeBusinessRule {
		return err
	}
	if appErr.Message != "contract stage map is not configured for this buyer stage" {
		return err
	}

	var participationID int64
	partErr := q.QueryRow(ctx,
		`SELECT id FROM contract_participations
		 WHERE contract_id = $1 AND buyer_id = $2 AND status = 'active' LIMIT 1`,
		contractID, buyerID).Scan(&participationID)
	if partErr == nil {
		if rbErr := RebuildParticipationStageMaps(ctx, q, contractID, participationID); rbErr != nil {
			return err
		}
	} else if errors.Is(partErr, pgx.ErrNoRows) {
		if rbErr := RebuildContractStageMaps(ctx, q, contractID); rbErr != nil {
			return err
		}
	} else {
		return partErr
	}
	return SyncPublisherStage(ctx, q, contractID, leadID, buyerID, buyerStageID)
}

func lookupPublisherStage(ctx context.Context, q database.Querier, contractID, buyerID, buyerStageID int64) (int64, int64, error) {
	var sourcePipelineID *int64
	err := q.QueryRow(ctx,
		`SELECT source_pipeline_id FROM contracts WHERE id = $1 AND deleted_at IS NULL`,
		contractID).Scan(&sourcePipelineID)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, 0, httpx.NotFound("contract not found")
	}
	if err != nil {
		return 0, 0, err
	}
	var participationID *int64
	_ = q.QueryRow(ctx,
		`SELECT id FROM contract_participations
		 WHERE contract_id = $1 AND buyer_id = $2 AND status = 'active' LIMIT 1`,
		contractID, buyerID).Scan(&participationID)
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

// InitPublisherTracking sets publisher-board placement when a lead is distributed to a buyer.
func InitPublisherTracking(ctx context.Context, q database.Querier, contractID, leadID, buyerID, buyerStageID int64) error {
	pubPipelineID, pubStageID, err := lookupPublisherStage(ctx, q, contractID, buyerID, buyerStageID)
	if err != nil {
		return err
	}
	_, err = q.Exec(ctx,
		`UPDATE leads SET publisher_pipeline_id = $2, publisher_stage_id = $3 WHERE id = $1`,
		leadID, pubPipelineID, pubStageID)
	return err
}

// SyncPublisherStage updates publisher-board placement when a buyer moves a contracted lead.
func SyncPublisherStage(ctx context.Context, q database.Querier, contractID, leadID, buyerID, buyerStageID int64) error {
	pubPipelineID, pubStageID, err := lookupPublisherStage(ctx, q, contractID, buyerID, buyerStageID)
	if err != nil {
		return err
	}
	_, err = q.Exec(ctx,
		`UPDATE leads SET publisher_pipeline_id = $2, publisher_stage_id = $3 WHERE id = $1`,
		leadID, pubPipelineID, pubStageID)
	return err
}
