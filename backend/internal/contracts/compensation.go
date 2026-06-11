package contracts

import (
	"context"
	"errors"
	"strings"

	"github.com/echayko/leadrula/backend/pkg/httpx"
	"github.com/jackc/pgx/v5"
)

// Compensation is one pay/delivery row on a contract.
type Compensation struct {
	ID                       int64    `json:"id"`
	ContractID               int64    `json:"contract_id"`
	Kind                     string   `json:"kind"`
	FlatAmount               *float64 `json:"flat_amount,omitempty"`
	BidMin                   *float64 `json:"bid_min,omitempty"`
	BidMax                   *float64 `json:"bid_max,omitempty"`
	RevPercent               *float64 `json:"rev_percent,omitempty"`
	ProfitPercent            *float64 `json:"profit_percent,omitempty"`
	CapPeriod                string   `json:"cap_period"`
	CapTotal                 *int     `json:"cap_total,omitempty"`
	CapMaxDaily              *int     `json:"cap_max_daily,omitempty"`
	Trigger                  string   `json:"trigger"`
	TriggerStageID           *int64   `json:"trigger_stage_id,omitempty"`
	SourcePipelineID         *int64   `json:"source_pipeline_id,omitempty"`
	SourceStageID            *int64   `json:"source_stage_id,omitempty"`
	CounterpartyPipelineID   *int64   `json:"counterparty_pipeline_id,omitempty"`
	CounterpartyStageID      *int64   `json:"counterparty_stage_id,omitempty"`
	ReturnStageID            *int64   `json:"return_stage_id,omitempty"`
	Delivery                 string   `json:"delivery"`
	Position                 int      `json:"position"`
	PayoutFrequency          *string  `json:"payout_frequency,omitempty"`
	PayoutWeekday            *int     `json:"payout_weekday,omitempty"`
	PayoutMonthDay           *int     `json:"payout_month_day,omitempty"`
}

const compensationCols = `id, contract_id, kind,
	flat_amount::float8, bid_min::float8, bid_max::float8, rev_percent::float8, profit_percent::float8,
	cap_period, cap_total, cap_max_daily, trigger, trigger_stage_id,
	source_pipeline_id, source_stage_id, counterparty_pipeline_id, counterparty_stage_id,
	return_stage_id, delivery, position,
	payout_frequency, payout_weekday, payout_month_day`

func scanCompensation(row pgx.Row) (*Compensation, error) {
	c := &Compensation{}
	err := row.Scan(
		&c.ID, &c.ContractID, &c.Kind,
		&c.FlatAmount, &c.BidMin, &c.BidMax, &c.RevPercent, &c.ProfitPercent,
		&c.CapPeriod, &c.CapTotal, &c.CapMaxDaily, &c.Trigger, &c.TriggerStageID,
		&c.SourcePipelineID, &c.SourceStageID, &c.CounterpartyPipelineID, &c.CounterpartyStageID,
		&c.ReturnStageID, &c.Delivery, &c.Position,
		&c.PayoutFrequency, &c.PayoutWeekday, &c.PayoutMonthDay,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, httpx.NotFound("compensation not found")
		}
		return nil, err
	}
	return c, nil
}

func (s *Service) ListCompensations(ctx context.Context, publisherID, contractID int64) ([]Compensation, error) {
	if _, err := s.Get(ctx, publisherID, contractID); err != nil {
		return nil, err
	}
	rows, err := s.pool.Query(ctx,
		`SELECT `+compensationCols+` FROM contract_compensations
		 WHERE contract_id = $1 ORDER BY position, id`, contractID)
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

func (s *Service) ListCompensationsForBuyer(ctx context.Context, buyerID, contractID int64) ([]Compensation, error) {
	if _, err := s.GetForBuyerContract(ctx, buyerID, contractID); err != nil {
		return nil, err
	}
	rows, err := s.pool.Query(ctx,
		`SELECT `+compensationCols+` FROM contract_compensations
		 WHERE contract_id = $1 AND participation_id IS NULL ORDER BY position, id`, contractID)
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

type CompensationParams struct {
	Kind                   string   `json:"kind"`
	FlatAmount             *float64 `json:"flat_amount"`
	BidMin                 *float64 `json:"bid_min"`
	BidMax                 *float64 `json:"bid_max"`
	RevPercent             *float64 `json:"rev_percent"`
	ProfitPercent          *float64 `json:"profit_percent"`
	CapPeriod              string   `json:"cap_period"`
	CapTotal               *int     `json:"cap_total"`
	CapMaxDaily            *int     `json:"cap_max_daily"`
	Trigger                string   `json:"trigger"`
	TriggerStageID         *int64   `json:"trigger_stage_id"`
	SourcePipelineID       *int64   `json:"source_pipeline_id"`
	SourceStageID          *int64   `json:"source_stage_id"`
	CounterpartyPipelineID *int64   `json:"counterparty_pipeline_id"`
	CounterpartyStageID    *int64   `json:"counterparty_stage_id"`
	ReturnStageID          *int64   `json:"return_stage_id"`
	Delivery               string   `json:"delivery"`
	Position               int      `json:"position"`
	PayoutFrequency        *string  `json:"payout_frequency"`
	PayoutWeekday          *int     `json:"payout_weekday"`
	PayoutMonthDay         *int     `json:"payout_month_day"`
}

func (s *Service) AddCompensation(ctx context.Context, publisherID, contractID int64, p CompensationParams) (*Compensation, error) {
	if _, err := s.Get(ctx, publisherID, contractID); err != nil {
		return nil, err
	}
	p = normalizeCompensationPipeline(p)
	if err := validateCompensationParams(p); err != nil {
		return nil, err
	}
	return scanCompensation(s.pool.QueryRow(ctx,
		`INSERT INTO contract_compensations(
		    contract_id, kind, flat_amount, bid_min, bid_max, rev_percent, profit_percent,
		    cap_period, cap_total, cap_max_daily, trigger, trigger_stage_id,
		    source_pipeline_id, source_stage_id, counterparty_pipeline_id, counterparty_stage_id,
		    return_stage_id, delivery, position,
		    payout_frequency, payout_weekday, payout_month_day)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,$21,$22)
		 RETURNING `+compensationCols,
		contractID, p.Kind, p.FlatAmount, p.BidMin, p.BidMax, p.RevPercent, p.ProfitPercent,
		p.CapPeriod, p.CapTotal, p.CapMaxDaily, p.Trigger, p.TriggerStageID,
		p.SourcePipelineID, p.SourceStageID, p.CounterpartyPipelineID, p.CounterpartyStageID,
		p.ReturnStageID, p.Delivery, p.Position,
		p.PayoutFrequency, p.PayoutWeekday, p.PayoutMonthDay))
}

func (s *Service) UpdateCompensation(ctx context.Context, publisherID, contractID, compID int64, p CompensationParams) (*Compensation, error) {
	if _, err := s.Get(ctx, publisherID, contractID); err != nil {
		return nil, err
	}
	p = normalizeCompensationPipeline(p)
	if err := validateCompensationParams(p); err != nil {
		return nil, err
	}
	return scanCompensation(s.pool.QueryRow(ctx,
		`UPDATE contract_compensations SET
		    kind = $3, flat_amount = $4, bid_min = $5, bid_max = $6,
		    rev_percent = $7, profit_percent = $8,
		    cap_period = $9, cap_total = $10, cap_max_daily = $11,
		    trigger = $12, trigger_stage_id = $13,
		    source_pipeline_id = $14, source_stage_id = $15,
		    counterparty_pipeline_id = $16, counterparty_stage_id = $17,
		    return_stage_id = $18, delivery = $19, position = $20,
		    payout_frequency = $21, payout_weekday = $22, payout_month_day = $23
		 WHERE id = $1 AND contract_id = $2
		 RETURNING `+compensationCols,
		compID, contractID,
		p.Kind, p.FlatAmount, p.BidMin, p.BidMax, p.RevPercent, p.ProfitPercent,
		p.CapPeriod, p.CapTotal, p.CapMaxDaily, p.Trigger, p.TriggerStageID,
		p.SourcePipelineID, p.SourceStageID, p.CounterpartyPipelineID, p.CounterpartyStageID,
		p.ReturnStageID, p.Delivery, p.Position,
		p.PayoutFrequency, p.PayoutWeekday, p.PayoutMonthDay))
}

func (s *Service) DeleteCompensation(ctx context.Context, publisherID, contractID, compID int64) error {
	if _, err := s.Get(ctx, publisherID, contractID); err != nil {
		return err
	}
	ct, err := s.pool.Exec(ctx,
		`DELETE FROM contract_compensations WHERE id = $1 AND contract_id = $2`, compID, contractID)
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return httpx.NotFound("compensation not found")
	}
	return nil
}

func (s *Service) CompensationBelongsToContract(ctx context.Context, compID, contractID int64) error {
	var ok bool
	err := s.pool.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM contract_compensations WHERE id = $1 AND contract_id = $2)`,
		compID, contractID).Scan(&ok)
	if err != nil {
		return err
	}
	if !ok {
		return httpx.NotFound("compensation not found")
	}
	return nil
}

var allowedCompKinds = map[string]bool{
	"flat_rate": true, "bid": true, "rev_share": true, "profit_share": true,
}

var allowedCompTriggers = map[string]bool{
	"per_lead": true, "buyer_stage": true, "manual": true,
}

var allowedCompDelivery = map[string]bool{
	"leads": true, "leads_pipeline": true, "webhook": true,
}

func normalizeCompensationPipeline(p CompensationParams) CompensationParams {
	delivery := strings.TrimSpace(p.Delivery)
	if delivery == "" {
		delivery = "leads_pipeline"
	}
	p.Delivery = delivery
	if delivery == "leads" {
		p.SourcePipelineID = nil
		p.SourceStageID = nil
		p.CounterpartyPipelineID = nil
		p.CounterpartyStageID = nil
		p.ReturnStageID = nil
	}
	return p
}

func validateCompensationParams(p CompensationParams) error {
	kind := strings.TrimSpace(p.Kind)
	if !allowedCompKinds[kind] {
		return httpx.Validation("kind must be flat_rate, bid, rev_share, or profit_share")
	}
	trigger := strings.TrimSpace(p.Trigger)
	if trigger == "" {
		trigger = defaultTriggerForKind(kind)
	}
	if !allowedCompTriggers[trigger] {
		return httpx.Validation("trigger must be per_lead, buyer_stage, or manual")
	}
	period := strings.TrimSpace(p.CapPeriod)
	if period == "" {
		period = "one_time"
	}
	if err := validateCapLimits(period, p.CapTotal, p.CapMaxDaily); err != nil {
		return err
	}
	delivery := strings.TrimSpace(p.Delivery)
	if delivery == "" {
		delivery = "leads_pipeline"
	}
	if !allowedCompDelivery[delivery] {
		return httpx.Validation("delivery must be leads, leads_pipeline, or webhook")
	}
	switch kind {
	case "flat_rate":
		if p.FlatAmount == nil || *p.FlatAmount < 0 {
			return httpx.Validation("flat_amount is required for flat_rate")
		}
		if trigger != "per_lead" {
			return httpx.Validation("flat_rate compensation must use per_lead trigger")
		}
	case "bid":
		if p.BidMin == nil || p.BidMax == nil || *p.BidMin < 0 || *p.BidMax < *p.BidMin {
			return httpx.Validation("bid_min and bid_max are required for bid with bid_min <= bid_max")
		}
		if trigger != "per_lead" {
			return httpx.Validation("bid compensation must use per_lead trigger")
		}
	case "rev_share":
		if p.RevPercent == nil || *p.RevPercent < 0 || *p.RevPercent > 100 {
			return httpx.Validation("rev_percent between 0 and 100 is required for rev_share")
		}
	case "profit_share":
		if p.ProfitPercent == nil || *p.ProfitPercent < 0 || *p.ProfitPercent > 100 {
			return httpx.Validation("profit_percent between 0 and 100 is required for profit_share")
		}
	}
	return validatePayoutParams(p.PayoutFrequency, p.PayoutWeekday, p.PayoutMonthDay)
}

func defaultTriggerForKind(kind string) string {
	if kind == "flat_rate" || kind == "bid" {
		return "per_lead"
	}
	return "buyer_stage"
}

// DebitAmount returns the per-lead charge for routing (flat_rate amount or bid_max).
func (c *Compensation) DebitAmount() float64 {
	switch c.Kind {
	case "flat_rate":
		if c.FlatAmount != nil {
			return *c.FlatAmount
		}
	case "bid":
		if c.BidMax != nil {
			return *c.BidMax
		}
	}
	return 0
}
