package contracts

import (
	"context"
	"strings"

	"github.com/echayko/leadrula/backend/internal/database"
	"github.com/echayko/leadrula/backend/internal/handlerid"
	"github.com/echayko/leadrula/backend/pkg/httpx"
	"github.com/jackc/pgx/v5"
)

func validateLeadCriteriaForActivation(c *LeadCriteria) error {
	if c == nil {
		return httpx.Validation("lead criteria is required for activation")
	}
	hasName := false
	hasPhoneOrEmail := false
	for _, r := range c.RequiredFields {
		if r.FieldType != "builtin" {
			continue
		}
		switch r.BuiltinField {
		case "first_name":
			hasName = true
		case "phone", "email":
			hasPhoneOrEmail = true
		}
	}
	if !hasName {
		return httpx.Validation("lead criteria must include first_name as a required field")
	}
	if !hasPhoneOrEmail {
		return httpx.Validation("lead criteria must include phone or email as a required field")
	}
	return nil
}

func validateActivationParams(p CreateParams) error {
	if p.BuyerID == 0 {
		return httpx.Validation("counterparty is required")
	}
	if err := validateLeadType(p.LeadType, true); err != nil {
		return err
	}
	capPeriod := p.CapPeriod
	if capPeriod == "" {
		capPeriod = "one_time"
	}
	if err := validateCapLimits(capPeriod, p.CapTotal, p.CapMaxDaily); err != nil {
		return err
	}
	comps := p.Compensations
	if len(comps) == 0 {
		return httpx.Validation("at least one compensation is required")
	}
	for i := range comps {
		if err := validateCompensationParams(comps[i]); err != nil {
			return err
		}
	}
	delivery := strings.TrimSpace(p.Delivery)
	if delivery == "" {
		delivery = "leads_pipeline"
	}
	if delivery == "leads_pipeline" {
		if p.SourceStageID == 0 {
			return httpx.Validation("source_stage_id is required for pipeline delivery")
		}
	}
	if err := validateLeadCriteriaForActivation(p.LeadCriteria); err != nil {
		return err
	}
	return nil
}

func (s *Service) CreateDraft(ctx context.Context, publisherID int64, p CreateParams) (*Contract, error) {
	name := strings.TrimSpace(p.Name)
	if name == "" {
		return nil, httpx.Validation("name is required")
	}
	contractType := p.ContractType
	if contractType == "" {
		contractType = "sell"
	}
	capPeriod := p.CapPeriod
	if capPeriod == "" {
		capPeriod = "one_time"
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	modes, strategy := draftOfferFields(p)
	c, err := s.insertDraftContract(ctx, tx, publisherID, draftInsertParams{
		Name:                 name,
		Description:          p.Description,
		LeadType:             p.LeadType,
		ContractType:         contractType,
		CapPeriod:            capPeriod,
		CapTotal:             p.CapTotal,
		CapMaxDaily:          p.CapMaxDaily,
		BuyerID:              nullableID(p.BuyerID),
		SourcePipelineID:     nullableID(p.SourcePipelineID),
		SourceStageID:        nullableID(p.SourceStageID),
		BuyerPipelineID:      nullableID(p.BuyerPipelineID),
		ReturnStageID:        nullableID(p.ReturnStageID),
		RatePerLead:          p.RatePerLead,
		AllowedDeliveryModes: modes,
		DistributionStrategy: strategy,
	})
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
	return s.Get(ctx, publisherID, c.ID)
}

func (s *Service) UpdateDraft(ctx context.Context, publisherID, contractID int64, p CreateParams) (*Contract, error) {
	existing, err := s.Get(ctx, publisherID, contractID)
	if err != nil {
		return nil, err
	}
	if existing.Status != "draft" {
		return nil, httpx.Validation("only draft contracts can be updated this way")
	}
	name := strings.TrimSpace(p.Name)
	if name == "" {
		return nil, httpx.Validation("name is required")
	}
	contractType := p.ContractType
	if contractType == "" {
		contractType = existing.ContractType
	}
	capPeriod := p.CapPeriod
	if capPeriod == "" {
		capPeriod = "one_time"
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	modes, strategy := draftOfferFields(p)
	c, err := scanContract(tx.QueryRow(ctx,
		`UPDATE contracts SET
		   name = $3,
		   description = $4,
		   lead_type = NULLIF($5, ''),
		   contract_type = $6,
		   cap_period = $7,
		   cap_total = $8,
		   cap_max_daily = $9,
		   buyer_id = $10,
		   source_pipeline_id = $11,
		   source_stage_id = $12,
		   buyer_pipeline_id = $13,
		   return_stage_id = $14,
		   rate_per_lead = $15,
		   allowed_delivery_modes = $16,
		   distribution_strategy = $17
		 WHERE id = $1 AND publisher_id = $2 AND deleted_at IS NULL AND status = 'draft'
		 RETURNING `+contractCols,
		contractID, publisherID, name, p.Description, p.LeadType, contractType,
		capPeriod, p.CapTotal, p.CapMaxDaily,
		nullableID(p.BuyerID), nullableID(p.SourcePipelineID), nullableID(p.SourceStageID),
		nullableID(p.BuyerPipelineID), nullableID(p.ReturnStageID), p.RatePerLead, modes, strategy))
	if err != nil {
		return nil, err
	}

	if len(p.Compensations) > 0 {
		if err := s.replaceCompensationsTx(ctx, tx, contractID, p); err != nil {
			return nil, err
		}
	}
	if p.LeadCriteria != nil {
		if err := replaceLeadCriteriaTx(ctx, tx, contractID, p.LeadCriteria); err != nil {
			return nil, err
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	_ = c
	return s.Get(ctx, publisherID, contractID)
}

func (s *Service) ActivateDraft(ctx context.Context, publisherID, contractID int64, p CreateParams) (*Contract, error) {
	existing, err := s.Get(ctx, publisherID, contractID)
	if err != nil {
		return nil, err
	}
	if existing.Status != "draft" {
		return nil, httpx.Validation("only draft contracts can be activated")
	}
	if p.BuyerID == 0 {
		p.BuyerID = derefInt64(existing.BuyerID)
	}
	if p.ContractType == "" {
		p.ContractType = existing.ContractType
	}
	if strings.TrimSpace(p.Name) == "" {
		p.Name = existing.Name
	}
	if err := validateActivationParams(p); err != nil {
		return nil, err
	}

	ct, err := counterpartyType(ctx, s.pool, p.BuyerID)
	if err != nil {
		return nil, err
	}
	if err := ValidateCounterpartyAccountType(p.ContractType, ct); err != nil {
		return nil, err
	}
	if err := s.assertCounterpartyPartnership(ctx, publisherID, p.ContractType, p.BuyerID, ct); err != nil {
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

	if p.BuyerID != 0 {
		var exists bool
		if err := tx.QueryRow(ctx,
			`SELECT EXISTS(
			   SELECT 1 FROM contracts
			   WHERE publisher_id = $1 AND buyer_id = $2 AND contract_type = $3
			     AND deleted_at IS NULL AND id <> $4)`,
			publisherID, p.BuyerID, p.ContractType, contractID).Scan(&exists); err != nil {
			return nil, err
		}
		if exists {
			return nil, httpx.Conflict("a contract already exists for this counterparty and type")
		}
	}

	c, err := scanContract(tx.QueryRow(ctx,
		`UPDATE contracts SET
		   buyer_id = $3,
		   name = $4,
		   description = $5,
		   lead_type = $6,
		   contract_type = $7,
		   cap_period = $8,
		   cap_total = $9,
		   cap_max_daily = $10,
		   source_pipeline_id = $11,
		   source_stage_id = $12,
		   buyer_pipeline_id = $13,
		   return_stage_id = $14,
		   rate_per_lead = $15,
		   status = 'active'
		 WHERE id = $1 AND publisher_id = $2 AND deleted_at IS NULL AND status = 'draft'
		 RETURNING `+contractCols,
		contractID, publisherID, p.BuyerID, p.Name, p.Description, p.LeadType, p.ContractType,
		capPeriod, p.CapTotal, p.CapMaxDaily,
		nullableID(p.SourcePipelineID), nullableID(p.SourceStageID),
		nullableID(p.BuyerPipelineID), nullableID(p.ReturnStageID), p.RatePerLead))
	if err != nil {
		return nil, err
	}

	if err := s.replaceCompensationsTx(ctx, tx, contractID, p); err != nil {
		return nil, err
	}
	if err := replaceLeadCriteriaTx(ctx, tx, contractID, p.LeadCriteria); err != nil {
		return nil, err
	}

	if ct == "publisher" && needsMirror(p.ContractType) {
		mirrorType := mirrorContractType(p.ContractType)
		mirrorParams := p
		mirrorParams.ContractType = mirrorType
		mirrorParams.BuyerID = publisherID
		mirror, err := s.insertContract(ctx, tx, p.BuyerID, mirrorParams)
		if err != nil {
			return nil, err
		}
		if err := s.insertCompensations(ctx, tx, mirror.ID, mirrorParams); err != nil {
			return nil, err
		}
		if _, err := tx.Exec(ctx,
			`UPDATE contracts SET mirror_contract_id = $1 WHERE id = $2`,
			mirror.ID, c.ID); err != nil {
			return nil, err
		}
		if _, err := tx.Exec(ctx,
			`UPDATE contracts SET mirror_contract_id = $1 WHERE id = $2`,
			c.ID, mirror.ID); err != nil {
			return nil, err
		}
		c.MirrorContractID = &mirror.ID
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	s.syncContractRoute(ctx, publisherID, c.ID)
	return s.Get(ctx, publisherID, c.ID)
}

type draftInsertParams struct {
	Name                 string
	Description          string
	LeadType             string
	ContractType         string
	CapPeriod            string
	CapTotal             *int
	CapMaxDaily          *int
	BuyerID              any
	SourcePipelineID     any
	SourceStageID        any
	BuyerPipelineID      any
	ReturnStageID        any
	RatePerLead          float64
	AllowedDeliveryModes []string
	DistributionStrategy string
}

func draftOfferFields(p CreateParams) ([]string, string) {
	modes := p.AllowedDeliveryModes
	if len(modes) == 0 {
		modes = []string{"leads", "leads_pipeline"}
	}
	strategy := strings.TrimSpace(p.DistributionStrategy)
	if strategy == "" {
		strategy = "round_robin"
	}
	return modes, strategy
}

func nullableID(id int64) any {
	if id == 0 {
		return nil
	}
	return id
}

func derefInt64(v *int64) int64 {
	if v == nil {
		return 0
	}
	return *v
}

func (s *Service) insertDraftContract(ctx context.Context, tx pgx.Tx, publisherID int64, p draftInsertParams) (*Contract, error) {
	contractType := p.ContractType
	if contractType == "" {
		contractType = "sell"
	}
	for range 10 {
		hid := handlerid.GenerateContract()
		c, err := scanContract(tx.QueryRow(ctx,
			`INSERT INTO contracts(publisher_id, buyer_id, name, description, lead_type, contract_type,
			    cap_period, cap_total, cap_max_daily,
			    source_pipeline_id, source_stage_id, buyer_pipeline_id, return_stage_id,
			    rate_per_lead, handler_id, status,
			    allowed_delivery_modes, distribution_strategy)
			 VALUES ($1,$2,$3,$4,NULLIF($5,''),$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,'draft',$16,$17)
			 RETURNING `+contractCols,
			publisherID, p.BuyerID, p.Name, p.Description, p.LeadType, contractType,
			p.CapPeriod, p.CapTotal, p.CapMaxDaily,
			p.SourcePipelineID, p.SourceStageID, p.BuyerPipelineID, p.ReturnStageID,
			p.RatePerLead, hid, p.AllowedDeliveryModes, p.DistributionStrategy))
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

func (s *Service) replaceCompensationsTx(ctx context.Context, tx pgx.Tx, contractID int64, p CreateParams) error {
	if _, err := tx.Exec(ctx, `DELETE FROM contract_compensations WHERE contract_id = $1`, contractID); err != nil {
		return err
	}
	return s.insertCompensationsRelaxed(ctx, tx, contractID, p)
}

func (s *Service) insertCompensationsRelaxed(ctx context.Context, tx pgx.Tx, contractID int64, p CreateParams) error {
	comps := p.Compensations
	if len(comps) == 0 {
		return nil
	}
	for i, c := range comps {
		pos := c.Position
		if pos == 0 {
			pos = i
		}
		c = normalizeCompensationPipeline(c)
		if c.Kind == "flat_rate" && c.FlatAmount == nil && p.RatePerLead > 0 {
			rate := p.RatePerLead
			c.FlatAmount = &rate
		}
		period := c.CapPeriod
		if period == "" {
			period = "one_time"
		}
		c.CapPeriod = period
		delivery := c.Delivery
		if delivery == "" {
			delivery = "leads_pipeline"
		}
		_, err := tx.Exec(ctx,
			`INSERT INTO contract_compensations(
			    contract_id, kind, flat_amount, bid_min, bid_max, rev_percent, profit_percent,
			    cap_period, cap_total, cap_max_daily, trigger, trigger_stage_id,
			    source_pipeline_id, source_stage_id, counterparty_pipeline_id, counterparty_stage_id,
			    return_stage_id, delivery, position,
			    payout_frequency, payout_weekday, payout_month_day)
			 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,$21,$22)`,
			contractID, c.Kind, c.FlatAmount, c.BidMin, c.BidMax, c.RevPercent, c.ProfitPercent,
			c.CapPeriod, c.CapTotal, c.CapMaxDaily, c.Trigger, c.TriggerStageID,
			c.SourcePipelineID, c.SourceStageID, c.CounterpartyPipelineID, c.CounterpartyStageID,
			c.ReturnStageID, delivery, pos,
			c.PayoutFrequency, c.PayoutWeekday, c.PayoutMonthDay)
		if err != nil {
			return err
		}
	}
	return nil
}
