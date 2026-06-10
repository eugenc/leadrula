package contracts

import (
	"context"
	"errors"

	"github.com/echayko/leadrula/backend/internal/database"
	"github.com/echayko/leadrula/backend/pkg/httpx"
	"github.com/jackc/pgx/v5"
)

// TryAccrueOnBuyerStage records rev/profit share when a lead enters a configured stage.
func TryAccrueOnBuyerStage(ctx context.Context, q database.Querier, contractID, leadID, stageID int64) error {
	cost, revenue, err := leadCostRevenue(ctx, q, leadID)
	if err != nil {
		return err
	}
	rows, err := q.Query(ctx,
		`SELECT id, kind, rev_percent, profit_percent
		 FROM contract_compensations
		 WHERE contract_id = $1 AND trigger = 'buyer_stage' AND trigger_stage_id = $2
		   AND kind IN ('rev_share', 'profit_share')`,
		contractID, stageID)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var compID int64
		var kind string
		var rev, profit *float64
		if err := rows.Scan(&compID, &kind, &rev, &profit); err != nil {
			return err
		}
		amount := accrualAmount(kind, rev, profit, cost, revenue)
		if amount <= 0 {
			continue
		}
		if err := insertAccrual(ctx, q, compID, leadID, amount, "stage"); err != nil {
			return err
		}
		if err := RecordEarningStage(ctx, q, compID, leadID, amount); err != nil {
			return err
		}
	}
	return rows.Err()
}

// AccrueManual records rev/profit share from a manual trigger (non-pipeline leads).
func (s *Service) AccrueManual(ctx context.Context, publisherID, contractID, compensationID, leadID int64) error {
	return AccrueManual(ctx, s.pool, publisherID, contractID, compensationID, leadID)
}

func AccrueManual(ctx context.Context, q database.Querier, publisherID, contractID, compensationID, leadID int64) error {
	if _, err := loadContractOwner(ctx, q, contractID, publisherID); err != nil {
		return err
	}
	var kind, trigger string
	var rev, profit *float64
	err := q.QueryRow(ctx,
		`SELECT kind, trigger, rev_percent, profit_percent
		 FROM contract_compensations WHERE id = $1 AND contract_id = $2`,
		compensationID, contractID).Scan(&kind, &trigger, &rev, &profit)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return httpx.NotFound("compensation not found")
		}
		return err
	}
	if trigger != "manual" {
		return httpx.Validation("compensation is not configured for manual trigger")
	}
	if kind != "rev_share" && kind != "profit_share" {
		return httpx.Validation("manual accrual applies to rev_share or profit_share only")
	}
	cost, revenue, err := leadCostRevenue(ctx, q, leadID)
	if err != nil {
		return err
	}
	amount := accrualAmount(kind, rev, profit, cost, revenue)
	if amount <= 0 {
		return httpx.Validation("lead has no revenue basis for accrual")
	}
	if err := insertAccrual(ctx, q, compensationID, leadID, amount, "manual"); err != nil {
		return err
	}
	return RecordEarningStage(ctx, q, compensationID, leadID, amount)
}

func insertAccrual(ctx context.Context, q database.Querier, compID, leadID int64, amount float64, source string) error {
	_, err := q.Exec(ctx,
		`INSERT INTO contract_compensation_accruals(compensation_id, lead_id, amount, trigger_source)
		 VALUES ($1,$2,$3,$4)
		 ON CONFLICT (compensation_id, lead_id, trigger_source) DO NOTHING`,
		compID, leadID, amount, source)
	return err
}

func loadContractOwner(ctx context.Context, q database.Querier, contractID, publisherID int64) (*Contract, error) {
	var id int64
	err := q.QueryRow(ctx,
		`SELECT id FROM contracts WHERE id = $1 AND publisher_id = $2 AND deleted_at IS NULL`,
		contractID, publisherID).Scan(&id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, httpx.NotFound("contract not found")
		}
		return nil, err
	}
	return &Contract{ID: id}, nil
}
