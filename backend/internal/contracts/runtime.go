package contracts

import (
	"context"
	"errors"

	"github.com/echayko/leadrula/backend/internal/database"
	"github.com/echayko/leadrula/backend/pkg/httpx"
	"github.com/jackc/pgx/v5"
)

// Target is the minimal contract info the intake/routing flow needs.
type Target struct {
	ID              int64
	BuyerID         int64
	BuyerPipelineID int64
	RatePerLead     float64
	CompensationID  int64
}

const perLeadCompSQL = `
SELECT cc.id, c.buyer_id, COALESCE(cc.counterparty_pipeline_id, c.buyer_pipeline_id),
       COALESCE(cc.flat_amount, cc.bid_max, 0)::float8
FROM contract_compensations cc
JOIN contracts c ON c.id = cc.contract_id
WHERE cc.contract_id = $1 AND cc.trigger = 'per_lead'
  AND cc.kind IN ('flat_rate', 'bid')
  AND c.deleted_at IS NULL
ORDER BY cc.position, cc.id
LIMIT 1`

func loadPerLeadTarget(ctx context.Context, q database.Querier, contractID int64) (*Target, error) {
	t := &Target{}
	err := q.QueryRow(ctx, perLeadCompSQL, contractID).Scan(
		&t.CompensationID, &t.BuyerID, &t.BuyerPipelineID, &t.RatePerLead)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return loadLegacyTarget(ctx, q, contractID)
		}
		return nil, err
	}
	t.ID = contractID
	return t, nil
}

func loadLegacyTarget(ctx context.Context, q database.Querier, contractID int64) (*Target, error) {
	t := &Target{}
	err := q.QueryRow(ctx,
		`SELECT id, buyer_id, buyer_pipeline_id, rate_per_lead::float8
		 FROM contracts WHERE id = $1 AND deleted_at IS NULL`, contractID).Scan(
		&t.ID, &t.BuyerID, &t.BuyerPipelineID, &t.RatePerLead)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, httpx.NotFound("contract not found")
		}
		return nil, err
	}
	return t, nil
}

func loadPerLeadTargetByComp(ctx context.Context, q database.Querier, compID int64) (*Target, error) {
	t := &Target{}
	err := q.QueryRow(ctx,
		`SELECT cc.contract_id, c.buyer_id, COALESCE(cc.counterparty_pipeline_id, c.buyer_pipeline_id),
		        COALESCE(cc.flat_amount, cc.bid_max, 0)::float8, cc.id
		 FROM contract_compensations cc
		 JOIN contracts c ON c.id = cc.contract_id
		 WHERE cc.id = $1 AND cc.trigger = 'per_lead' AND c.deleted_at IS NULL`,
		compID).Scan(&t.ID, &t.BuyerID, &t.BuyerPipelineID, &t.RatePerLead, &t.CompensationID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, httpx.NotFound("compensation not found")
		}
		return nil, err
	}
	return t, nil
}

// GetTarget loads routing target; uses route compensation when set.
func GetTarget(ctx context.Context, q database.Querier, contractID int64, routeCompensationID *int64) (*Target, error) {
	if routeCompensationID != nil && *routeCompensationID != 0 {
		return loadPerLeadTargetByComp(ctx, q, *routeCompensationID)
	}
	return loadPerLeadTarget(ctx, q, contractID)
}

// GetTargetByContract is backward-compatible entry without route compensation.
func GetTargetByContract(ctx context.Context, q database.Querier, contractID int64) (*Target, error) {
	return GetTarget(ctx, q, contractID, nil)
}
