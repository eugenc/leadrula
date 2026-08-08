package leads

import (
	"context"
	"errors"
	"math"
	"time"

	"github.com/echayko/leadrula/backend/internal/database"
	"github.com/jackc/pgx/v5"
)

func netProfitValue(cost, revenue *float64) *float64 {
	if revenue == nil {
		return nil
	}
	c := 0.0
	if cost != nil {
		c = *cost
	}
	v := *revenue - c
	return &v
}

func (r *Repository) EnrichLeadEconomics(ctx context.Context, accountType string, l *Lead) error {
	if accountType == "buyer" {
		l.Cost = nil
		l.Revenue = nil
		l.GrossProfit = nil
		l.NetProfit = nil
		var amount *float64
		err := r.pool.QueryRow(ctx,
			`SELECT ABS(amount::float8) FROM transactions
			 WHERE lead_id = $1 AND type = 'debit'
			 ORDER BY created_at DESC LIMIT 1`, l.ID).Scan(&amount)
		if err == nil && amount != nil {
			l.PurchasePrice = amount
		} else if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		return nil
	}

	l.PurchasePrice = nil
	l.NetProfit = netProfitValue(l.Cost, l.Revenue)

	if l.Status == "distributed" || l.Status == "closed" {
		var saleAmount, costBasis *float64
		err := r.pool.QueryRow(ctx,
			`SELECT ce.amount::float8, ce.cost_basis::float8
			 FROM compensation_earnings ce
			 JOIN contract_compensations cc ON cc.id = ce.compensation_id
			 WHERE ce.lead_id = $1 AND ce.kind = 'distribute' AND ce.amount > 0
			 ORDER BY ce.created_at DESC LIMIT 1`, l.ID).Scan(&saleAmount, &costBasis)
		if err == nil && saleAmount != nil {
			basis := 0.0
			if costBasis != nil {
				basis = *costBasis
			}
			gp := *saleAmount - basis
			l.GrossProfit = &gp
		} else if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
	}
	return nil
}

func (r *Repository) EnrichPendingReturnsBatch(ctx context.Context, items []Lead) error {
	if len(items) == 0 {
		return nil
	}
	ids := make([]int64, len(items))
	for i := range items {
		ids[i] = items[i].ID
	}
	rows, err := r.pool.Query(ctx,
		`SELECT DISTINCT ON (slr.lead_id) slr.lead_id, slr.execute_at, c.schedule_timezone
		 FROM scheduled_lead_returns slr
		 JOIN contracts c ON c.id = slr.contract_id
		 WHERE slr.lead_id = ANY($1) AND slr.status = 'pending'
		 ORDER BY slr.lead_id, slr.created_at DESC`, ids)
	if err != nil {
		return err
	}
	defer rows.Close()
	type pendingReturn struct {
		executeAt time.Time
		timezone  string
	}
	pending := map[int64]pendingReturn{}
	for rows.Next() {
		var leadID int64
		var executeAt time.Time
		var tz string
		if err := rows.Scan(&leadID, &executeAt, &tz); err != nil {
			return err
		}
		pending[leadID] = pendingReturn{executeAt: executeAt, timezone: tz}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	for i := range items {
		p, ok := pending[items[i].ID]
		if !ok {
			continue
		}
		at := p.executeAt
		items[i].PendingReturnAt = &at
		tz := p.timezone
		items[i].PendingReturnTimezone = &tz
	}
	return nil
}

func (r *Repository) EnrichPendingReturn(ctx context.Context, l *Lead) error {
	items := []Lead{*l}
	if err := r.EnrichPendingReturnsBatch(ctx, items); err != nil {
		return err
	}
	l.PendingReturnAt = items[0].PendingReturnAt
	l.PendingReturnTimezone = items[0].PendingReturnTimezone
	return nil
}

func (r *Repository) EnrichLeadEconomicsBatch(ctx context.Context, accountType string, items []Lead) error {
	if len(items) == 0 {
		return nil
	}
	if accountType == "buyer" {
		return r.enrichBuyerEconomicsBatch(ctx, items)
	}
	return r.enrichPublisherEconomicsBatch(ctx, items)
}

func (r *Repository) enrichBuyerEconomicsBatch(ctx context.Context, items []Lead) error {
	ids := make([]int64, len(items))
	for i := range items {
		items[i].Cost = nil
		items[i].Revenue = nil
		items[i].GrossProfit = nil
		items[i].NetProfit = nil
		ids[i] = items[i].ID
	}
	rows, err := r.pool.Query(ctx,
		`SELECT DISTINCT ON (lead_id) lead_id, ABS(amount::float8)
		 FROM transactions
		 WHERE lead_id = ANY($1) AND type = 'debit'
		 ORDER BY lead_id, created_at DESC`, ids)
	if err != nil {
		return err
	}
	defer rows.Close()
	prices := map[int64]float64{}
	for rows.Next() {
		var leadID int64
		var amount float64
		if err := rows.Scan(&leadID, &amount); err != nil {
			return err
		}
		prices[leadID] = amount
	}
	if err := rows.Err(); err != nil {
		return err
	}
	for i := range items {
		if amount, ok := prices[items[i].ID]; ok {
			items[i].PurchasePrice = &amount
		}
	}
	return nil
}

func (r *Repository) enrichPublisherEconomicsBatch(ctx context.Context, items []Lead) error {
	var ids []int64
	for i := range items {
		items[i].PurchasePrice = nil
		items[i].NetProfit = netProfitValue(items[i].Cost, items[i].Revenue)
		if items[i].Status == "distributed" || items[i].Status == "closed" {
			ids = append(ids, items[i].ID)
		}
	}
	if len(ids) == 0 {
		return nil
	}
	rows, err := r.pool.Query(ctx,
		`SELECT DISTINCT ON (ce.lead_id) ce.lead_id, ce.amount::float8, ce.cost_basis::float8
		 FROM compensation_earnings ce
		 JOIN contract_compensations cc ON cc.id = ce.compensation_id
		 WHERE ce.lead_id = ANY($1) AND ce.kind = 'distribute' AND ce.amount > 0
		 ORDER BY ce.lead_id, ce.created_at DESC`, ids)
	if err != nil {
		return err
	}
	defer rows.Close()
	type earning struct {
		saleAmount float64
		costBasis  float64
	}
	earnings := map[int64]earning{}
	for rows.Next() {
		var leadID int64
		var saleAmount float64
		var costBasis *float64
		if err := rows.Scan(&leadID, &saleAmount, &costBasis); err != nil {
			return err
		}
		basis := 0.0
		if costBasis != nil {
			basis = *costBasis
		}
		earnings[leadID] = earning{saleAmount: saleAmount, costBasis: basis}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	for i := range items {
		e, ok := earnings[items[i].ID]
		if !ok {
			continue
		}
		gp := e.saleAmount - e.costBasis
		items[i].GrossProfit = &gp
	}
	return nil
}

func CostBasisFromLead(l *Lead) *float64 {
	return costBasisFromLead(l)
}

func costBasisFromLead(l *Lead) *float64 {
	if l.Cost == nil {
		return nil
	}
	v := *l.Cost
	return &v
}

func roundMoney(v float64) float64 {
	return math.Round(v*100) / 100
}

// SetCostAfterBuyerDistribution sets or clears acquisition cost on the lead after a paid distribution.
func (r *Repository) SetCostAfterBuyerDistribution(ctx context.Context, q database.Querier, leadID int64, buyerType string, ratePerLead float64) error {
	if buyerType == "publisher" && ratePerLead > 0 {
		amount := roundMoney(ratePerLead)
		return r.SetMoneyField(ctx, q, leadID, "cost", &amount)
	}
	return r.SetMoneyField(ctx, q, leadID, "cost", nil)
}
