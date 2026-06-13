package contracts

import (
	"context"
	"strings"

	"github.com/echayko/leadrula/backend/internal/database"
	"github.com/echayko/leadrula/backend/internal/handlerid"
	"github.com/echayko/leadrula/backend/pkg/httpx"
	"github.com/jackc/pgx/v5"
)

func pipelineDeliveryAllowed(modes []string) bool {
	for _, m := range modes {
		if m == "leads_pipeline" {
			return true
		}
	}
	return false
}

func validateOfferPipelineConfig(modes []string, sourcePipelineID, sourceStageID, returnStageID int64) error {
	if !pipelineDeliveryAllowed(modes) {
		return nil
	}
	if sourcePipelineID == 0 || sourceStageID == 0 || returnStageID == 0 {
		return httpx.Validation("source_pipeline_id, source_stage_id, and return_stage_id are required when pipeline delivery is allowed")
	}
	return nil
}

func validateOfferParams(p CreateParams) error {
	if strings.TrimSpace(p.Name) == "" {
		return httpx.Validation("name is required")
	}
	if err := validateLeadType(p.LeadType, true); err != nil {
		return err
	}
	if err := validateAllowedDeliveryModes(p.LeadType, p.AllowedDeliveryModes); err != nil {
		return err
	}
	strategy := strings.TrimSpace(p.DistributionStrategy)
	if strategy == "" {
		strategy = "round_robin"
	}
	if !allowedDistributionStrategy[strategy] {
		return httpx.Validation("invalid distribution strategy")
	}
	capPeriod := p.CapPeriod
	if capPeriod == "" {
		capPeriod = "one_time"
	}
	if err := validateCapLimits(capPeriod, p.CapTotal, p.CapMaxDaily); err != nil {
		return err
	}
	for i := range p.Compensations {
		if err := validateCompensationParams(p.Compensations[i]); err != nil {
			return err
		}
	}
	return validateOfferPipelineConfig(p.AllowedDeliveryModes, p.SourcePipelineID, p.SourceStageID, p.ReturnStageID)
}

func (s *Service) enrichOffer(ctx context.Context, c *Contract) error {
	var parentID *int64
	var inviteToken *string
	err := s.pool.QueryRow(ctx,
		`SELECT allowed_delivery_modes, distribution_strategy, parent_contract_id, invite_token
		 FROM contracts WHERE id = $1`, c.ID).Scan(
		&c.AllowedDeliveryModes, &c.DistributionStrategy, &parentID, &inviteToken)
	if err != nil {
		return err
	}
	c.ParentContractID = parentID
	if inviteToken != nil {
		c.InviteToken = *inviteToken
	}
	return nil
}

func (s *Service) CreateActiveOffer(ctx context.Context, publisherID int64, p CreateParams) (*Contract, error) {
	if p.ContractType == "" {
		p.ContractType = "sell"
	}
	if p.ContractType != "sell" {
		return nil, httpx.Validation("open offers are only supported for sell contracts")
	}
	if p.DistributionStrategy == "" {
		p.DistributionStrategy = "round_robin"
	}
	if err := validateOfferParams(p); err != nil {
		return nil, err
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	c, err := s.insertActiveOffer(ctx, tx, publisherID, p)
	if err != nil {
		return nil, err
	}
	if len(p.Compensations) > 0 {
		if err := s.replaceCompensationsTx(ctx, tx, c.ID, p); err != nil {
			return nil, err
		}
	}
	if p.LeadCriteria != nil {
		if err := saveLeadCriteriaTx(ctx, tx, c.ID, p.LeadCriteria); err != nil {
			return nil, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	s.syncContractRoute(ctx, publisherID, c.ID)
	return s.Get(ctx, publisherID, c.ID)
}

func (s *Service) insertActiveOffer(ctx context.Context, tx pgx.Tx, publisherID int64, p CreateParams) (*Contract, error) {
	capPeriod := p.CapPeriod
	if capPeriod == "" {
		capPeriod = "one_time"
	}
	strategy := p.DistributionStrategy
	if strategy == "" {
		strategy = "round_robin"
	}
	for range 10 {
		hid := handlerid.GenerateContract()
		c, err := scanContract(tx.QueryRow(ctx,
			`INSERT INTO contracts(
			    publisher_id, name, description, lead_type, contract_type,
			    cap_period, cap_total, cap_max_daily, handler_id, status,
			    allowed_delivery_modes, distribution_strategy,
			    source_pipeline_id, source_stage_id, return_stage_id)
			 VALUES ($1,$2,$3,NULLIF($4,''),$5,$6,$7,$8,$9,'active',$10,$11,
			    NULLIF($12,0), NULLIF($13,0), NULLIF($14,0))
			 RETURNING `+contractCols,
			publisherID, p.Name, p.Description, p.LeadType, p.ContractType,
			capPeriod, p.CapTotal, p.CapMaxDaily, hid, p.AllowedDeliveryModes, strategy,
			p.SourcePipelineID, p.SourceStageID, p.ReturnStageID))
		if err == nil {
			return c, nil
		}
		if database.IsUniqueViolation(err) {
			continue
		}
		return nil, err
	}
	return nil, httpx.Conflict("could not generate unique contract handler id")
}

type OfferUpdateParams struct {
	AllowedDeliveryModes *[]string
	DistributionStrategy *string
	SourcePipelineID     *int64
	SourceStageID        *int64
	ReturnStageID        *int64
}

func (s *Service) UpdateOffer(ctx context.Context, publisherID, contractID int64, p OfferUpdateParams) (*Contract, error) {
	c, err := s.Get(ctx, publisherID, contractID)
	if err != nil {
		return nil, err
	}
	if c.Status != "active" {
		return nil, httpx.Validation("only active contracts can update offer settings")
	}
	modes := c.AllowedDeliveryModes
	if p.AllowedDeliveryModes != nil {
		modes = *p.AllowedDeliveryModes
		if err := validateAllowedDeliveryModes(c.LeadType, modes); err != nil {
			return nil, err
		}
	}
	strategy := c.DistributionStrategy
	if p.DistributionStrategy != nil {
		strategy = strings.TrimSpace(*p.DistributionStrategy)
		if !allowedDistributionStrategy[strategy] {
			return nil, httpx.Validation("invalid distribution strategy")
		}
	}
	sourcePipelineID := derefInt64(c.SourcePipelineID)
	sourceStageID := derefInt64(c.SourceStageID)
	returnStageID := derefInt64(c.ReturnStageID)
	if p.SourcePipelineID != nil {
		sourcePipelineID = *p.SourcePipelineID
	}
	if p.SourceStageID != nil {
		sourceStageID = *p.SourceStageID
	}
	if p.ReturnStageID != nil {
		returnStageID = *p.ReturnStageID
	}
	if err := validateOfferPipelineConfig(modes, sourcePipelineID, sourceStageID, returnStageID); err != nil {
		return nil, err
	}
	_, err = s.pool.Exec(ctx,
		`UPDATE contracts SET
		   allowed_delivery_modes = $3,
		   distribution_strategy = $4,
		   source_pipeline_id = NULLIF($5,0),
		   source_stage_id = NULLIF($6,0),
		   return_stage_id = NULLIF($7,0),
		   updated_at = now()
		 WHERE id = $1 AND publisher_id = $2`,
		contractID, publisherID, modes, strategy,
		sourcePipelineID, sourceStageID, returnStageID)
	if err != nil {
		return nil, err
	}
	s.syncContractRoute(ctx, publisherID, contractID)
	return s.Get(ctx, publisherID, contractID)
}

func (s *Service) ActivateOfferDraft(ctx context.Context, publisherID, contractID int64, p CreateParams) (*Contract, error) {
	existing, err := s.Get(ctx, publisherID, contractID)
	if err != nil {
		return nil, err
	}
	if existing.Status != "draft" {
		return nil, httpx.Validation("only draft contracts can be activated")
	}
	if p.ContractType == "" {
		p.ContractType = existing.ContractType
	}
	if strings.TrimSpace(p.Name) == "" {
		p.Name = existing.Name
	}
	if len(p.AllowedDeliveryModes) == 0 {
		p.AllowedDeliveryModes = existing.AllowedDeliveryModes
	}
	if p.DistributionStrategy == "" {
		p.DistributionStrategy = existing.DistributionStrategy
	}
	if p.DistributionStrategy == "" {
		p.DistributionStrategy = "round_robin"
	}
	if err := validateOfferParams(p); err != nil {
		return nil, err
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	capPeriod := p.CapPeriod
	if capPeriod == "" {
		capPeriod = "one_time"
	}
	c, err := scanContract(tx.QueryRow(ctx,
		`UPDATE contracts SET
		   name = $3, description = $4, lead_type = $5, contract_type = $6,
		   cap_period = $7, cap_total = $8, cap_max_daily = $9,
		   allowed_delivery_modes = $10, distribution_strategy = $11,
		   source_pipeline_id = NULLIF($12,0), source_stage_id = NULLIF($13,0),
		   return_stage_id = NULLIF($14,0),
		   status = 'active', buyer_id = NULL
		 WHERE id = $1 AND publisher_id = $2 AND deleted_at IS NULL AND status = 'draft'
		 RETURNING `+contractCols,
		contractID, publisherID, p.Name, p.Description, p.LeadType, p.ContractType,
		capPeriod, p.CapTotal, p.CapMaxDaily, p.AllowedDeliveryModes, p.DistributionStrategy,
		p.SourcePipelineID, p.SourceStageID, p.ReturnStageID))
	if err != nil {
		return nil, err
	}
	if len(p.Compensations) > 0 {
		if err := s.replaceCompensationsTx(ctx, tx, contractID, p); err != nil {
			return nil, err
		}
	}
	if p.LeadCriteria != nil {
		if err := saveLeadCriteriaTx(ctx, tx, contractID, p.LeadCriteria); err != nil {
			return nil, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	s.syncContractRoute(ctx, publisherID, contractID)
	_ = c
	return s.Get(ctx, publisherID, contractID)
}
