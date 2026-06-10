package contracts

import (
	"context"

	"github.com/echayko/leadrula/backend/internal/database"
)

func leadCostRevenue(ctx context.Context, q database.Querier, leadID int64) (cost, revenue *float64, err error) {
	err = q.QueryRow(ctx, `SELECT cost::float8, revenue::float8 FROM leads WHERE id = $1`, leadID).Scan(&cost, &revenue)
	return cost, revenue, err
}

func accrualAmount(kind string, rev, profit *float64, cost, revenue *float64) float64 {
	if kind == "rev_share" {
		if revenue == nil || rev == nil || *rev <= 0 {
			return 0
		}
		return roundMoney(*revenue * *rev / 100)
	}
	if kind == "profit_share" {
		if revenue == nil || profit == nil || *profit <= 0 {
			return 0
		}
		c := 0.0
		if cost != nil {
			c = *cost
		}
		basis := *revenue - c
		if basis <= 0 {
			return 0
		}
		return roundMoney(basis * *profit / 100)
	}
	return 0
}

func roundMoney(v float64) float64 {
	return float64(int(v*100+0.5)) / 100
}
