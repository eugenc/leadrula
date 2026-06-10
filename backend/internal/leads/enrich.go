package leads

import (
	"context"
	"errors"
	"math"

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

func (r *Repository) EnrichLeadEconomicsBatch(ctx context.Context, accountType string, items []Lead) error {
	for i := range items {
		if err := r.EnrichLeadEconomics(ctx, accountType, &items[i]); err != nil {
			return err
		}
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
