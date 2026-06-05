package contracts

import (
	"context"

	"github.com/echayko/leadrula/backend/internal/database"
	"github.com/echayko/leadrula/backend/internal/handlerid"
	"github.com/echayko/leadrula/backend/pkg/httpx"
	"github.com/jackc/pgx/v5"
)

func counterpartyType(ctx context.Context, q database.Querier, accountID int64) (string, error) {
	var typ string
	err := q.QueryRow(ctx, `SELECT type FROM accounts WHERE id = $1`, accountID).Scan(&typ)
	return typ, err
}

func (s *Service) createWithMirror(ctx context.Context, ownerID int64, p CreateParams) (*Contract, error) {
	ct, err := counterpartyType(ctx, s.pool, p.BuyerID)
	if err != nil {
		return nil, err
	}
	if err := ValidateCounterpartyAccountType(p.ContractType, ct); err != nil {
		return nil, err
	}
	if err := s.assertCounterpartyPartnership(ctx, ownerID, p.ContractType, p.BuyerID, ct); err != nil {
		return nil, err
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	primary, err := s.insertContract(ctx, tx, ownerID, p)
	if err != nil {
		return nil, err
	}
	if err := s.insertCompensations(ctx, tx, primary.ID, p); err != nil {
		return nil, err
	}
	if err := saveLeadCriteriaTx(ctx, tx, primary.ID, p.LeadCriteria); err != nil {
		return nil, err
	}

	if ct == "publisher" && needsMirror(p.ContractType) {
		mirrorType := mirrorContractType(p.ContractType)
		mirrorParams := p
		mirrorParams.ContractType = mirrorType
		mirrorParams.BuyerID = ownerID
		mirror, err := s.insertContract(ctx, tx, p.BuyerID, mirrorParams)
		if err != nil {
			return nil, err
		}
		if err := s.insertCompensations(ctx, tx, mirror.ID, mirrorParams); err != nil {
			return nil, err
		}
		if _, err := tx.Exec(ctx,
			`UPDATE contracts SET mirror_contract_id = $1 WHERE id = $2`,
			mirror.ID, primary.ID); err != nil {
			return nil, err
		}
		if _, err := tx.Exec(ctx,
			`UPDATE contracts SET mirror_contract_id = $1 WHERE id = $2`,
			primary.ID, mirror.ID); err != nil {
			return nil, err
		}
		primary.MirrorContractID = &mirror.ID
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return s.Get(ctx, ownerID, primary.ID)
}

func needsMirror(contractType string) bool {
	return contractType == "sell" || contractType == "buy"
}

func mirrorContractType(t string) string {
	if t == "sell" {
		return "buy"
	}
	return "sell"
}

func (s *Service) insertContract(ctx context.Context, tx pgx.Tx, publisherID int64, p CreateParams) (*Contract, error) {
	var exists bool
	if err := tx.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM contracts WHERE publisher_id = $1 AND buyer_id = $2 AND contract_type = $3 AND deleted_at IS NULL)`,
		publisherID, p.BuyerID, p.ContractType).Scan(&exists); err != nil {
		return nil, err
	}
	if exists {
		return nil, httpx.Conflict("a contract already exists for this counterparty and type")
	}

	contractType := p.ContractType
	if contractType == "" {
		contractType = "sell"
	}
	capPeriod := p.CapPeriod
	if capPeriod == "" {
		capPeriod = "one_time"
	}

	var c *Contract
	var err error
	for range 10 {
		hid := handlerid.Generate("C")
		c, err = scanContract(tx.QueryRow(ctx,
			`INSERT INTO contracts(publisher_id, buyer_id, name, description, lead_type, contract_type,
			    cap_period, cap_total, cap_max_daily,
			    source_pipeline_id, source_stage_id, buyer_pipeline_id, return_stage_id, rate_per_lead, handler_id)
			 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15)
			 RETURNING `+contractCols,
			publisherID, p.BuyerID, p.Name, p.Description, p.LeadType, contractType,
			capPeriod, p.CapTotal, p.CapMaxDaily,
			p.SourcePipelineID, p.SourceStageID,
			p.BuyerPipelineID, p.ReturnStageID, p.RatePerLead, hid))
		if err == nil {
			return c, nil
		}
		if database.IsUniqueViolation(err) {
			continue
		}
		return nil, err
	}
	return nil, err
}

func (s *Service) insertCompensations(ctx context.Context, tx pgx.Tx, contractID int64, p CreateParams) error {
	comps := p.Compensations
	if len(comps) == 0 {
		delivery := p.Delivery
		if delivery == "" {
			delivery = "leads_pipeline"
		}
		comps = []CompensationParams{{
			Kind:                   "flat_rate",
			FlatAmount:             &p.RatePerLead,
			CapPeriod:              p.CapPeriod,
			CapTotal:               p.CapTotal,
			CapMaxDaily:            p.CapMaxDaily,
			Trigger:                "per_lead",
			SourcePipelineID:       &p.SourcePipelineID,
			SourceStageID:          &p.SourceStageID,
			CounterpartyPipelineID: &p.BuyerPipelineID,
			ReturnStageID:          &p.ReturnStageID,
			Delivery:               delivery,
		}}
	}
	for i, c := range comps {
		pos := c.Position
		if pos == 0 {
			pos = i
		}
		if err := validateCompensationParams(c); err != nil {
			return err
		}
		delivery := c.Delivery
		if delivery == "" {
			delivery = "leads_pipeline"
		}
		_, err := tx.Exec(ctx,
			`INSERT INTO contract_compensations(
			    contract_id, kind, flat_amount, bid_min, bid_max, rev_percent, profit_percent,
			    cap_period, cap_total, cap_max_daily, trigger, trigger_stage_id,
			    source_pipeline_id, source_stage_id, counterparty_pipeline_id, counterparty_stage_id,
			    return_stage_id, delivery, position)
			 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19)`,
			contractID, c.Kind, c.FlatAmount, c.BidMin, c.BidMax, c.RevPercent, c.ProfitPercent,
			c.CapPeriod, c.CapTotal, c.CapMaxDaily, c.Trigger, c.TriggerStageID,
			c.SourcePipelineID, c.SourceStageID, c.CounterpartyPipelineID, c.CounterpartyStageID,
			c.ReturnStageID, delivery, pos)
		if err != nil {
			return err
		}
	}
	return nil
}
