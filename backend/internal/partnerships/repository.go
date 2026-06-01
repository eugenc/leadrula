package partnerships

import (
	"context"
	"errors"

	"github.com/echayko/leadrula/backend/internal/database"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrNotFound = errors.New("not found")
var ErrConflict = errors.New("conflict")

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

func (r *Repository) Pool() *pgxpool.Pool { return r.pool }

const partnershipCols = `id, publisher_id, buyer_id, status, requested_by, requested_by_user_id, created_at, updated_at`

func scanPartnership(row pgx.Row) (*Partnership, error) {
	p := &Partnership{}
	err := row.Scan(
		&p.ID, &p.PublisherID, &p.BuyerID, &p.Status, &p.RequestedBy,
		&p.RequestedByUserID, &p.CreatedAt, &p.UpdatedAt,
	)
	return p, err
}

func (r *Repository) GetByID(ctx context.Context, id int64) (*Partnership, error) {
	q := `SELECT ` + partnershipCols + ` FROM partnerships WHERE id = $1`
	p, err := scanPartnership(r.pool.QueryRow(ctx, q, id))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return p, nil
}

func (r *Repository) GetByPair(ctx context.Context, publisherID, buyerID int64) (*Partnership, error) {
	q := `SELECT ` + partnershipCols + ` FROM partnerships WHERE publisher_id = $1 AND buyer_id = $2`
	p, err := scanPartnership(r.pool.QueryRow(ctx, q, publisherID, buyerID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return p, nil
}

func (r *Repository) HasActive(ctx context.Context, publisherID, buyerID int64) (bool, error) {
	var ok bool
	err := r.pool.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM partnerships WHERE publisher_id = $1 AND buyer_id = $2 AND status = 'active')`,
		publisherID, buyerID).Scan(&ok)
	return ok, err
}

func (r *Repository) PublisherAccountID(ctx context.Context) (int64, error) {
	var id int64
	err := r.pool.QueryRow(ctx, `SELECT id FROM accounts WHERE type = 'publisher' LIMIT 1`).Scan(&id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, ErrNotFound
		}
		return 0, err
	}
	return id, nil
}

func (r *Repository) AccountName(ctx context.Context, accountID int64) (string, error) {
	var name string
	err := r.pool.QueryRow(ctx, `SELECT name FROM accounts WHERE id = $1`, accountID).Scan(&name)
	return name, err
}

func (r *Repository) AccountHandlerID(ctx context.Context, accountID int64) (string, error) {
	var hid string
	err := r.pool.QueryRow(ctx, `SELECT handler_id FROM accounts WHERE id = $1`, accountID).Scan(&hid)
	return hid, err
}

func (r *Repository) UpsertActive(ctx context.Context, publisherID, buyerID, requestedBy int64) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO partnerships(publisher_id, buyer_id, status, requested_by, requested_by_user_id)
		 VALUES ($1, $2, 'active', 'publisher', $3)
		 ON CONFLICT (publisher_id, buyer_id) DO UPDATE SET
		   status = 'active', requested_by = 'publisher', requested_by_user_id = EXCLUDED.requested_by_user_id,
		   updated_at = now()`, publisherID, buyerID, requestedBy)
	return err
}

func (r *Repository) CreatePendingBuyer(ctx context.Context, publisherID, buyerID, requestedBy int64) (*Partnership, error) {
	const ins = `INSERT INTO partnerships(publisher_id, buyer_id, status, requested_by, requested_by_user_id)
		VALUES ($1, $2, 'pending_buyer', 'publisher', $3)
		ON CONFLICT (publisher_id, buyer_id) DO NOTHING
		RETURNING ` + partnershipCols
	p, err := scanPartnership(r.pool.QueryRow(ctx, ins, publisherID, buyerID, requestedBy))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrConflict
		}
		return nil, err
	}
	return p, nil
}

func (r *Repository) CreatePendingPublisher(ctx context.Context, publisherID, buyerID, requestedBy int64) (*Partnership, error) {
	const ins = `INSERT INTO partnerships(publisher_id, buyer_id, status, requested_by, requested_by_user_id)
		VALUES ($1, $2, 'pending_publisher', 'buyer', $3)
		ON CONFLICT (publisher_id, buyer_id) DO NOTHING
		RETURNING ` + partnershipCols
	p, err := scanPartnership(r.pool.QueryRow(ctx, ins, publisherID, buyerID, requestedBy))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrConflict
		}
		return nil, err
	}
	return p, nil
}

func (r *Repository) Activate(ctx context.Context, id int64) (*Partnership, error) {
	const q = `UPDATE partnerships SET status = 'active', updated_at = now()
		WHERE id = $1 AND status IN ('pending_buyer', 'pending_publisher')
		RETURNING ` + partnershipCols
	p, err := scanPartnership(r.pool.QueryRow(ctx, q, id))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return p, nil
}

func (r *Repository) Reject(ctx context.Context, id int64) error {
	ct, err := r.pool.Exec(ctx,
		`UPDATE partnerships SET status = 'rejected', updated_at = now()
		 WHERE id = $1 AND status IN ('pending_buyer', 'pending_publisher')`, id)
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *Repository) ResetToPendingBuyer(ctx context.Context, id, requestedBy int64) (*Partnership, error) {
	const q = `UPDATE partnerships SET status = 'pending_buyer', requested_by = 'publisher',
		requested_by_user_id = $2, updated_at = now()
		WHERE id = $1 AND status IN ('rejected', 'revoked')
		RETURNING ` + partnershipCols
	p, err := scanPartnership(r.pool.QueryRow(ctx, q, id, requestedBy))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return p, nil
}

func (r *Repository) ResetToPendingPublisher(ctx context.Context, id, requestedBy int64) (*Partnership, error) {
	const q = `UPDATE partnerships SET status = 'pending_publisher', requested_by = 'buyer',
		requested_by_user_id = $2, updated_at = now()
		WHERE id = $1 AND status IN ('rejected', 'revoked')
		RETURNING ` + partnershipCols
	p, err := scanPartnership(r.pool.QueryRow(ctx, q, id, requestedBy))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return p, nil
}

func (r *Repository) ListForPublisher(ctx context.Context, publisherID int64) ([]ListItem, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT p.id, p.status, p.requested_by, a.name, a.handler_id, p.created_at
		 FROM partnerships p
		 JOIN accounts a ON a.id = p.buyer_id
		 WHERE p.publisher_id = $1
		 ORDER BY p.updated_at DESC`, publisherID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanListItems(rows)
}

func (r *Repository) ListForBuyer(ctx context.Context, buyerID int64) ([]ListItem, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT p.id, p.status, p.requested_by, a.name, a.handler_id, p.created_at
		 FROM partnerships p
		 JOIN accounts a ON a.id = p.publisher_id
		 WHERE p.buyer_id = $1
		 ORDER BY p.updated_at DESC`, buyerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanListItems(rows)
}

func scanListItems(rows pgx.Rows) ([]ListItem, error) {
	var out []ListItem
	for rows.Next() {
		var item ListItem
		if err := rows.Scan(&item.ID, &item.Status, &item.RequestedBy, &item.PartnerName, &item.PartnerHandlerID, &item.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (r *Repository) AdminUserIDs(ctx context.Context, q database.Querier, accountID int64) ([]int64, error) {
	rows, err := q.Query(ctx, `SELECT id FROM users WHERE account_id = $1 AND role = 'admin' AND is_active`, accountID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}
