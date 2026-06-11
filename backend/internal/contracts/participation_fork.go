package contracts

import (
	"context"
	"encoding/json"

	"github.com/echayko/leadrula/backend/internal/database"
	"github.com/echayko/leadrula/backend/internal/handlerid"
	"github.com/echayko/leadrula/backend/pkg/httpx"
	"github.com/jackc/pgx/v5"
)

type counterProposal struct {
	Compensations []CompensationParams `json:"compensations"`
	LeadCriteria  *LeadCriteria        `json:"lead_criteria"`
}

func (s *Service) AcceptCounter(ctx context.Context, publisherID, participationID int64) (*Contract, error) {
	part, err := s.GetParticipationForPublisher(ctx, publisherID, participationID)
	if err != nil {
		return nil, err
	}
	if part.Status != "counter_pending" {
		return nil, httpx.Validation("participation has no pending counter-offer")
	}
	if len(part.CounterProposal) == 0 {
		return nil, httpx.Validation("counter proposal is empty")
	}
	var proposal counterProposal
	if err := json.Unmarshal(part.CounterProposal, &proposal); err != nil {
		return nil, httpx.Validation("invalid counter proposal")
	}
	for i := range proposal.Compensations {
		if err := validateCompensationParams(proposal.Compensations[i]); err != nil {
			return nil, err
		}
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	orig, err := s.Get(ctx, publisherID, part.ContractID)
	if err != nil {
		return nil, err
	}

	newContract, err := s.cloneContractForCounter(ctx, tx, orig, part.BuyerID)
	if err != nil {
		return nil, err
	}

	newPart, err := scanParticipation(tx.QueryRow(ctx,
		`INSERT INTO contract_participations(contract_id, buyer_id, status)
		 VALUES ($1,$2,'pending')
		 RETURNING `+participationReturningCols,
		newContract.ID, part.BuyerID))
	if err != nil {
		return nil, err
	}

	params := CreateParams{Compensations: proposal.Compensations, LeadCriteria: proposal.LeadCriteria}
	if err := s.insertParticipationCompensations(ctx, tx, newPart.ID, newContract.ID, params); err != nil {
		return nil, err
	}
	if proposal.LeadCriteria != nil {
		if err := saveParticipationCriteriaTx(ctx, tx, newPart.ID, newContract.ID, proposal.LeadCriteria); err != nil {
			return nil, err
		}
	}

	forkID := newContract.ID
	if _, err := tx.Exec(ctx,
		`UPDATE contract_participations SET
		   status = 'superseded', superseded_by_contract_id = $2,
		   publisher_responded_at = now(), updated_at = now()
		 WHERE id = $1`, participationID, forkID); err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	s.notifyParticipation(ctx, s.pool, part.BuyerID, "contract_forked", map[string]any{
		"contract_id":       forkID,
		"participation_id":  newPart.ID,
		"original_contract": part.ContractID,
	})
	return s.Get(ctx, publisherID, forkID)
}

func (s *Service) cloneContractForCounter(ctx context.Context, tx pgx.Tx, orig *Contract, buyerID int64) (*Contract, error) {
	name := orig.Name + " (counter)"
	for range 10 {
		hid := handlerid.GenerateContract()
		c, err := scanContract(tx.QueryRow(ctx,
			`INSERT INTO contracts(
			    publisher_id, buyer_id, name, description, lead_type, contract_type,
			    cap_period, cap_total, cap_max_daily, handler_id, status,
			    allowed_delivery_modes, distribution_strategy, parent_contract_id)
			 VALUES ($1,NULL,$2,$3,$4,$5,$6,$7,$8,$9,'active',$10,$11,$12)
			 RETURNING `+contractCols,
			orig.PublisherID, name, orig.Description, orig.LeadType, orig.ContractType,
			orig.CapPeriod, orig.CapTotal, orig.CapMaxDaily, hid,
			orig.AllowedDeliveryModes, orig.DistributionStrategy, orig.ID))
		if err == nil {
			_ = buyerID
			return c, nil
		}
		if database.IsUniqueViolation(err) {
			continue
		}
		return nil, err
	}
	return nil, httpx.Conflict("could not generate unique contract handler id")
}

func (s *Service) insertParticipationCompensations(ctx context.Context, tx pgx.Tx, participationID, contractID int64, p CreateParams) error {
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
		period := c.CapPeriod
		if period == "" {
			period = "one_time"
		}
		delivery := c.Delivery
		if delivery == "" {
			delivery = "leads_pipeline"
		}
		_, err := tx.Exec(ctx,
			`INSERT INTO contract_compensations(
			    contract_id, participation_id, kind, flat_amount, bid_min, bid_max, rev_percent, profit_percent,
			    cap_period, cap_total, cap_max_daily, trigger, trigger_stage_id,
			    source_pipeline_id, source_stage_id, counterparty_pipeline_id, counterparty_stage_id,
			    return_stage_id, delivery, position,
			    payout_frequency, payout_weekday, payout_month_day)
			 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,$21,$22)`,
			contractID, participationID, c.Kind, c.FlatAmount, c.BidMin, c.BidMax, c.RevPercent, c.ProfitPercent,
			period, c.CapTotal, c.CapMaxDaily, c.Trigger, c.TriggerStageID,
			c.SourcePipelineID, c.SourceStageID, c.CounterpartyPipelineID, c.CounterpartyStageID,
			c.ReturnStageID, delivery, pos,
			c.PayoutFrequency, c.PayoutWeekday, c.PayoutMonthDay)
		if err != nil {
			return err
		}
	}
	return nil
}

func saveParticipationCriteriaTx(ctx context.Context, tx pgx.Tx, participationID, contractID int64, c *LeadCriteria) error {
	if _, err := tx.Exec(ctx, `DELETE FROM contract_required_fields WHERE participation_id = $1`, participationID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `DELETE FROM contract_field_map WHERE participation_id = $1`, participationID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `DELETE FROM contract_filter_rules WHERE participation_id = $1`, participationID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `DELETE FROM contract_quality_rules WHERE participation_id = $1`, participationID); err != nil {
		return err
	}
	// Reuse contract-level save by temporarily using contract_id tables — insert with participation_id
	return saveLeadCriteriaWithParticipation(ctx, tx, contractID, participationID, c)
}
