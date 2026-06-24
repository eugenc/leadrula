package calls

import (
	"context"
	"errors"

	"github.com/echayko/leadrula/backend/internal/database"
	"github.com/jackc/pgx/v5"
)

// contractBasePrice returns the contract's per-call base rate (dollars) and the
// compensation id used for publisher earnings. Looks at per_connected_call first,
// then per_lead, for flat_rate/bid compensations.
func (s *Service) contractBasePrice(ctx context.Context, q database.Querier, contractID int64) (float64, int64, error) {
	var price float64
	var compID int64
	err := q.QueryRow(ctx,
		`SELECT COALESCE(cc.flat_amount, cc.bid_max, 0)::float8, cc.id
		 FROM contract_compensations cc
		 JOIN contracts c ON c.id = cc.contract_id
		 WHERE cc.contract_id = $1
		   AND cc.trigger IN ('per_connected_call', 'per_lead')
		   AND cc.kind IN ('flat_rate', 'bid')
		   AND c.deleted_at IS NULL
		 ORDER BY CASE WHEN cc.trigger = 'per_connected_call' THEN 0 ELSE 1 END, cc.position, cc.id
		 LIMIT 1`,
		contractID).Scan(&price, &compID)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, 0, nil
	}
	if err != nil {
		return 0, 0, err
	}
	return price, compID, nil
}

// resolvePrice applies the override → RTB bid precedence over the contract base.
func resolvePrice(base float64, rateOverride, rtbBid *float64) float64 {
	price := base
	if rateOverride != nil {
		price = *rateOverride
	}
	if rtbBid != nil && *rtbBid > price {
		price = *rtbBid
	}
	if price < 0 {
		return 0
	}
	return price
}
