package partnerships

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

// PartnerPublisher is a publisher linked via publisher_partnerships.
type PartnerPublisher struct {
	ID        int64  `json:"id"`
	Name      string `json:"name"`
	HandlerID string `json:"handler_id"`
}

func ListPartnerPublishers(ctx context.Context, pool *pgxpool.Pool, publisherID int64) ([]PartnerPublisher, error) {
	rows, err := pool.Query(ctx,
		`SELECT a.id, a.name, a.handler_id
		 FROM publisher_partnerships pp
		 JOIN accounts a ON (
		   (pp.publisher_a_id = $1 AND a.id = pp.publisher_b_id)
		   OR (pp.publisher_b_id = $1 AND a.id = pp.publisher_a_id)
		 )
		 WHERE (pp.publisher_a_id = $1 OR pp.publisher_b_id = $1) AND pp.status = 'active'
		 ORDER BY a.name`, publisherID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []PartnerPublisher
	for rows.Next() {
		var p PartnerPublisher
		if err := rows.Scan(&p.ID, &p.Name, &p.HandlerID); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}
