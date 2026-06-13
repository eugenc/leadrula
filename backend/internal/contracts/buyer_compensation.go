package contracts

import (
	"context"

	"github.com/echayko/leadrula/backend/pkg/httpx"
)

func (s *Service) ListCompensationsForParticipationBuyer(ctx context.Context, buyerID, participationID int64) ([]Compensation, error) {
	part, err := s.GetParticipationForBuyer(ctx, buyerID, participationID)
	if err != nil {
		return nil, err
	}
	rows, err := s.pool.Query(ctx,
		`SELECT `+compensationCols+` FROM contract_compensations
		 WHERE contract_id = $1 AND participation_id = $2 ORDER BY position, id`,
		part.ContractID, participationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Compensation
	for rows.Next() {
		c, err := scanCompensation(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *c)
	}
	return out, rows.Err()
}

func (s *Service) UpdateBuyerCompensationTriggerStage(ctx context.Context, buyerID, contractID, compID int64, stageID int64) (*Compensation, error) {
	if _, err := s.GetForBuyerContract(ctx, buyerID, contractID); err != nil {
		return nil, err
	}
	if stageID == 0 {
		return nil, httpx.Validation("trigger_stage_id is required")
	}
	c, err := scanCompensation(s.pool.QueryRow(ctx,
		`SELECT `+compensationCols+` FROM contract_compensations
		 WHERE id = $1 AND contract_id = $2 AND participation_id IS NULL`, compID, contractID))
	if err != nil {
		return nil, err
	}
	if err := validateBuyerTriggerStageUpdate(c, stageID); err != nil {
		return nil, err
	}
	var pipelineID int64
	if err := s.pool.QueryRow(ctx,
		`SELECT COALESCE(buyer_pipeline_id, 0) FROM contracts WHERE id = $1`, contractID).Scan(&pipelineID); err != nil {
		return nil, err
	}
	if err := validateBuyerCompTriggerStage(ctx, s.pool, stageID, pipelineID); err != nil {
		return nil, err
	}
	return scanCompensation(s.pool.QueryRow(ctx,
		`UPDATE contract_compensations SET trigger_stage_id = $3
		 WHERE id = $1 AND contract_id = $2
		 RETURNING `+compensationCols,
		compID, contractID, stageID))
}

func (s *Service) UpdateParticipationCompensationTriggerStage(ctx context.Context, buyerID, participationID, compID int64, stageID int64) (*Compensation, error) {
	part, err := s.GetParticipationForBuyer(ctx, buyerID, participationID)
	if err != nil {
		return nil, err
	}
	if !participationMutable(part.Status) {
		return nil, httpx.Validation("participation cannot be edited")
	}
	if stageID == 0 {
		return nil, httpx.Validation("trigger_stage_id is required")
	}
	c, err := scanCompensation(s.pool.QueryRow(ctx,
		`SELECT `+compensationCols+` FROM contract_compensations
		 WHERE id = $1 AND contract_id = $2 AND participation_id = $3`,
		compID, part.ContractID, participationID))
	if err != nil {
		return nil, err
	}
	if err := validateBuyerTriggerStageUpdate(c, stageID); err != nil {
		return nil, err
	}
	var pipelineID int64
	if part.BuyerPipelineID != nil {
		pipelineID = *part.BuyerPipelineID
	}
	if pipelineID == 0 {
		_ = s.pool.QueryRow(ctx,
			`SELECT COALESCE(buyer_pipeline_id, 0) FROM contracts WHERE id = $1`, part.ContractID).Scan(&pipelineID)
	}
	if pipelineID == 0 {
		return nil, httpx.Validation("buyer pipeline is required before setting trigger stage")
	}
	if err := validateBuyerCompTriggerStage(ctx, s.pool, stageID, pipelineID); err != nil {
		return nil, err
	}
	return scanCompensation(s.pool.QueryRow(ctx,
		`UPDATE contract_compensations SET trigger_stage_id = $3
		 WHERE id = $1 AND contract_id = $2 AND participation_id = $4
		 RETURNING `+compensationCols,
		compID, part.ContractID, stageID, participationID))
}

func validateBuyerTriggerStageUpdate(c *Compensation, stageID int64) error {
	if c.Kind != "rev_share" && c.Kind != "profit_share" {
		return httpx.Validation("trigger stage can only be set on rev_share or profit_share")
	}
	if c.Trigger != "buyer_stage" {
		return httpx.Validation("trigger stage can only be set when trigger is buyer_stage")
	}
	if stageID == 0 {
		return httpx.Validation("trigger_stage_id is required")
	}
	return nil
}
