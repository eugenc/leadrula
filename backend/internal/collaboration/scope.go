package collaboration

import (
	"context"
	"fmt"

	"github.com/echayko/leadrula/backend/internal/auth"
	"github.com/jackc/pgx/v5/pgxpool"
)

const allowedPipelineIDsSQL = `
SELECT id FROM (
  SELECT c.buyer_pipeline_id AS id FROM contracts c
    WHERE c.publisher_id = $1 AND c.buyer_id = $2 AND c.status = 'active'
      AND c.buyer_pipeline_id IS NOT NULL AND c.deleted_at IS NULL
  UNION
  SELECT cp.buyer_pipeline_id FROM contract_participations cp
    JOIN contracts c ON c.id = cp.contract_id
    WHERE c.publisher_id = $1 AND cp.buyer_id = $2 AND cp.status = 'active'
      AND cp.buyer_pipeline_id IS NOT NULL AND c.deleted_at IS NULL
  UNION
  SELECT id FROM pipelines WHERE account_id = $2 AND collaboration_publisher_id = $1
  UNION
  SELECT DISTINCT l.pipeline_id FROM leads l
    WHERE l.owner_account_id = $2 AND l.publisher_id = $1
      AND l.pipeline_id IS NOT NULL AND l.deleted_at IS NULL
) t WHERE id IS NOT NULL`

// AllowedPipelineIDs returns buyer pipeline ids visible to a publisher during collaboration.
func AllowedPipelineIDs(ctx context.Context, pool *pgxpool.Pool, publisherID, buyerID int64) ([]int64, error) {
	rows, err := pool.Query(ctx, allowedPipelineIDsSQL, publisherID, buyerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}

func (r *Repository) AllowedPipelineIDs(ctx context.Context, publisherID, buyerID int64) ([]int64, error) {
	return AllowedPipelineIDs(ctx, r.pool, publisherID, buyerID)
}

// AppendLeadScope adds publisher collaboration filters when impersonating.
// The first arg in args must be the buyer account id (owner_account_id).
func AppendLeadScope(p *auth.Principal, where string, args []any) (string, []any) {
	pubID, ok := p.CollaborationPublisherID()
	if !ok {
		return where, args
	}
	args = append(args, pubID)
	pubArg := len(args)
	where += fmt.Sprintf(` AND l.publisher_id = $%d AND (
		l.contract_id IS NULL OR EXISTS (
			SELECT 1 FROM contracts c
			WHERE c.id = l.contract_id AND c.publisher_id = $%d AND c.buyer_id = $1
			  AND c.status = 'active' AND c.deleted_at IS NULL
		))`, pubArg, pubArg)
	return where, args
}
