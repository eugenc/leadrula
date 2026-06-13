package routing

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// SyncContractDistributionRoute upserts a pipeline-origin route that distributes leads to a contract.
func (s *Service) SyncContractDistributionRoute(ctx context.Context, publisherID, contractID int64) error {
	var sourcePipelineID, sourceStageID *int64
	var contractName string
	err := s.pool.QueryRow(ctx,
		`SELECT source_pipeline_id, source_stage_id, name
		 FROM contracts WHERE id = $1 AND publisher_id = $2 AND deleted_at IS NULL`,
		contractID, publisherID).Scan(&sourcePipelineID, &sourceStageID, &contractName)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	if sourcePipelineID == nil || sourceStageID == nil || *sourcePipelineID == 0 || *sourceStageID == 0 {
		_, err = s.pool.Exec(ctx,
			`UPDATE routes SET is_active = false
			 WHERE publisher_id = $1 AND contract_id = $2 AND origin = 'pipeline' AND destination = 'contract'`,
			publisherID, contractID)
		return err
	}

	var routeID int64
	err = s.pool.QueryRow(ctx,
		`SELECT id FROM routes
		 WHERE publisher_id = $1 AND contract_id = $2 AND origin = 'pipeline' AND destination = 'contract'
		 ORDER BY id LIMIT 1`,
		publisherID, contractID).Scan(&routeID)
	if errors.Is(err, pgx.ErrNoRows) {
		name := fmt.Sprintf("Contract: %s", contractName)
		pid := publisherID
		_, err = s.insertRoute(ctx, &pid, nil, CreateRouteParams{
			Name:               name,
			Origin:             "pipeline",
			OriginPipelineID:   sourcePipelineID,
			OriginStageID:      sourceStageID,
			Destination:        "contract",
			ContractID:         &contractID,
			Delivery:           "leads_pipeline",
		})
		return err
	}
	if err != nil {
		return err
	}
	_, err = s.pool.Exec(ctx,
		`UPDATE routes SET
		   origin_pipeline_id = $3,
		   origin_stage_id = $4,
		   delivery = 'leads_pipeline',
		   is_active = true
		 WHERE id = $1 AND publisher_id = $2`,
		routeID, publisherID, *sourcePipelineID, *sourceStageID)
	return err
}
