package collaboration

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

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

func scanCollab(row pgx.Row) (*Collaboration, error) {
	c := &Collaboration{}
	err := row.Scan(
		&c.ID, &c.PublisherID, &c.BuyerID, &c.Status, &c.Version, &c.AutoGranted,
		&c.TargetPublisherUserID, &c.RequestedByUserID, &c.RevokedAt, &c.CreatedAt, &c.UpdatedAt,
	)
	return c, err
}

const collabCols = `id, publisher_id, buyer_id, status, version, auto_granted,
	target_publisher_user_id, requested_by_user_id, revoked_at, created_at, updated_at`

func (r *Repository) GetByPair(ctx context.Context, publisherID, buyerID int64) (*Collaboration, error) {
	q := `SELECT ` + collabCols + ` FROM buyer_collaborations WHERE publisher_id = $1 AND buyer_id = $2`
	c, err := scanCollab(r.pool.QueryRow(ctx, q, publisherID, buyerID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return c, nil
}

func (r *Repository) GetByBuyer(ctx context.Context, buyerID int64) (*Collaboration, error) {
	q := `SELECT ` + collabCols + ` FROM buyer_collaborations WHERE buyer_id = $1 ORDER BY updated_at DESC LIMIT 1`
	c, err := scanCollab(r.pool.QueryRow(ctx, q, buyerID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return c, nil
}

func (r *Repository) ListByPublisher(ctx context.Context, publisherID int64) ([]BuyerCollabSummary, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT buyer_id, status, version FROM buyer_collaborations WHERE publisher_id = $1`, publisherID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []BuyerCollabSummary
	for rows.Next() {
		var s BuyerCollabSummary
		if err := rows.Scan(&s.BuyerID, &s.Status, &s.Version); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

func (r *Repository) CreateActive(ctx context.Context, q database.Querier, publisherID, buyerID int64, requestedBy int64, autoGranted bool) (*Collaboration, error) {
	const ins = `INSERT INTO buyer_collaborations(publisher_id, buyer_id, status, version, auto_granted, requested_by_user_id)
		VALUES ($1, $2, 'active', 1, $3, $4)
		RETURNING ` + collabCols
	return scanCollab(q.QueryRow(ctx, ins, publisherID, buyerID, autoGranted, requestedBy))
}

func (r *Repository) CreatePendingBuyer(ctx context.Context, publisherID, buyerID, requestedBy int64) (*Collaboration, error) {
	const ins = `INSERT INTO buyer_collaborations(publisher_id, buyer_id, status, version, auto_granted, requested_by_user_id)
		VALUES ($1, $2, 'pending_buyer', 1, false, $3)
		ON CONFLICT (publisher_id, buyer_id) DO NOTHING
		RETURNING ` + collabCols
	c, err := scanCollab(r.pool.QueryRow(ctx, ins, publisherID, buyerID, requestedBy))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrConflict
		}
		return nil, err
	}
	return c, nil
}

func (r *Repository) CreatePendingPublisher(ctx context.Context, publisherID, buyerID, targetUserID, requestedBy int64) (*Collaboration, error) {
	const ins = `INSERT INTO buyer_collaborations(publisher_id, buyer_id, status, version, auto_granted,
		target_publisher_user_id, requested_by_user_id)
		VALUES ($1, $2, 'pending_publisher', 1, false, $3, $4)
		ON CONFLICT (publisher_id, buyer_id) DO NOTHING
		RETURNING ` + collabCols
	c, err := scanCollab(r.pool.QueryRow(ctx, ins, publisherID, buyerID, targetUserID, requestedBy))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrConflict
		}
		return nil, err
	}
	return c, nil
}

func (r *Repository) Activate(ctx context.Context, collabID int64) (*Collaboration, error) {
	const q = `UPDATE buyer_collaborations SET status = 'active', updated_at = now(),
		target_publisher_user_id = NULL WHERE id = $1 AND status IN ('pending_buyer', 'pending_publisher')
		RETURNING ` + collabCols
	c, err := scanCollab(r.pool.QueryRow(ctx, q, collabID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return c, nil
}

func (r *Repository) RejectPending(ctx context.Context, collabID int64) error {
	ct, err := r.pool.Exec(ctx,
		`DELETE FROM buyer_collaborations WHERE id = $1 AND status IN ('pending_buyer', 'pending_publisher')`, collabID)
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *Repository) Revoke(ctx context.Context, collabID int64, revokedBy int64) (*Collaboration, error) {
	const q = `UPDATE buyer_collaborations SET status = 'revoked', version = version + 1,
		revoked_at = now(), revoked_by_user_id = $2, updated_at = now()
		WHERE id = $1 AND status = 'active'
		RETURNING ` + collabCols
	c, err := scanCollab(r.pool.QueryRow(ctx, q, collabID, revokedBy))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return c, nil
}

func (r *Repository) ResetToPendingBuyer(ctx context.Context, collabID, requestedBy int64) (*Collaboration, error) {
	const q = `UPDATE buyer_collaborations SET status = 'pending_buyer', requested_by_user_id = $2,
		target_publisher_user_id = NULL, updated_at = now()
		WHERE id = $1 AND status = 'revoked'
		RETURNING ` + collabCols
	c, err := scanCollab(r.pool.QueryRow(ctx, q, collabID, requestedBy))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return c, nil
}

func (r *Repository) ResetToPendingPublisher(ctx context.Context, collabID, targetUserID, requestedBy int64) (*Collaboration, error) {
	const q = `UPDATE buyer_collaborations SET status = 'pending_publisher', requested_by_user_id = $2,
		target_publisher_user_id = $3, updated_at = now()
		WHERE id = $1 AND status = 'revoked'
		RETURNING ` + collabCols
	c, err := scanCollab(r.pool.QueryRow(ctx, q, collabID, requestedBy, targetUserID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return c, nil
}

func (r *Repository) ValidateActive(ctx context.Context, publisherID, buyerID, tokenVersion int64) error {
	var status string
	var version int64
	err := r.pool.QueryRow(ctx,
		`SELECT status, version FROM buyer_collaborations WHERE publisher_id = $1 AND buyer_id = $2`,
		publisherID, buyerID).Scan(&status, &version)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		return err
	}
	if status != StatusActive || version != tokenVersion {
		return ErrNotFound
	}
	return nil
}

func (r *Repository) InsertAudit(ctx context.Context, q database.Querier, eventType string, publisherID, buyerID, actorUserID int64, metadata map[string]any) error {
	raw, err := json.Marshal(metadata)
	if err != nil {
		return err
	}
	var actor *int64
	if actorUserID > 0 {
		actor = &actorUserID
	}
	_, err = q.Exec(ctx,
		`INSERT INTO collaboration_audit_log(event_type, publisher_id, buyer_id, actor_user_id, metadata)
		 VALUES ($1, $2, $3, $4, $5)`,
		eventType, publisherID, buyerID, actor, raw)
	return err
}

func (r *Repository) ListAudit(ctx context.Context, publisherID, buyerID int64, limit int) ([]AuditEntry, error) {
	if limit <= 0 {
		limit = 20
	}
	rows, err := r.pool.Query(ctx,
		`SELECT l.id, l.event_type, l.actor_user_id, COALESCE(u.full_name, ''), l.metadata, l.created_at
		 FROM collaboration_audit_log l
		 LEFT JOIN users u ON u.id = l.actor_user_id
		 WHERE l.publisher_id = $1 AND l.buyer_id = $2
		 ORDER BY l.created_at DESC LIMIT $3`, publisherID, buyerID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanAuditRows(rows)
}

func scanAuditRows(rows pgx.Rows) ([]AuditEntry, error) {
	var out []AuditEntry
	for rows.Next() {
		var e AuditEntry
		var raw []byte
		var actorID *int64
		if err := rows.Scan(&e.ID, &e.EventType, &actorID, &e.ActorName, &raw, &e.CreatedAt); err != nil {
			return nil, err
		}
		e.ActorUserID = actorID
		_ = json.Unmarshal(raw, &e.Metadata)
		if e.Metadata == nil {
			e.Metadata = map[string]any{}
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

const auditSelect = `SELECT l.id, l.event_type, l.actor_user_id, COALESCE(u.full_name, ''), l.metadata, l.created_at
 FROM collaboration_audit_log l
 LEFT JOIN users u ON u.id = l.actor_user_id`

func (r *Repository) auditWhere(p AuditListParams) (string, []any) {
	args := []any{p.BuyerID}
	where := "l.buyer_id = $1"
	add := func(cond string, val any) {
		args = append(args, val)
		where += fmt.Sprintf(" AND %s $%d", cond, len(args))
	}
	if p.From != nil {
		add("l.created_at >=", *p.From)
	}
	if p.To != nil {
		add("l.created_at <=", *p.To)
	}
	if p.ActorUserID != nil {
		add("l.actor_user_id =", *p.ActorUserID)
	}
	return where, args
}

func (r *Repository) ListAuditForBuyer(ctx context.Context, p AuditListParams) (*AuditListResult, error) {
	page := p.Page
	if page < 1 {
		page = 1
	}
	limit := p.Limit
	if limit <= 0 {
		limit = 25
	}
	if limit > 100 {
		limit = 100
	}

	where, args := r.auditWhere(p)

	var total int
	if err := r.pool.QueryRow(ctx, `SELECT COUNT(*) FROM collaboration_audit_log l WHERE `+where, args...).Scan(&total); err != nil {
		return nil, err
	}

	offset := (page - 1) * limit
	qArgs := append([]any{}, args...)
	qArgs = append(qArgs, limit, offset)
	q := auditSelect + ` WHERE ` + where + fmt.Sprintf(` ORDER BY l.created_at DESC LIMIT $%d OFFSET $%d`, len(args)+1, len(args)+2)

	rows, err := r.pool.Query(ctx, q, qArgs...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items, err := scanAuditRows(rows)
	if err != nil {
		return nil, err
	}
	if items == nil {
		items = []AuditEntry{}
	}
	return &AuditListResult{Items: items, Total: total, Page: page, Limit: limit}, nil
}

func (r *Repository) ListAuditActors(ctx context.Context, buyerID int64) ([]AuditActor, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT DISTINCT u.id, u.full_name
		 FROM collaboration_audit_log l
		 JOIN users u ON u.id = l.actor_user_id
		 WHERE l.buyer_id = $1
		 ORDER BY u.full_name`, buyerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []AuditActor
	for rows.Next() {
		var a AuditActor
		if err := rows.Scan(&a.ID, &a.Name); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	if out == nil {
		out = []AuditActor{}
	}
	return out, rows.Err()
}

func (r *Repository) AccountName(ctx context.Context, accountID int64) (string, error) {
	var name string
	err := r.pool.QueryRow(ctx, `SELECT name FROM accounts WHERE id = $1`, accountID).Scan(&name)
	return name, err
}

func (r *Repository) GetAccountProfile(ctx context.Context, accountID int64) (publicID, name, website string, err error) {
	err = r.pool.QueryRow(ctx,
		`SELECT public_id, name, COALESCE(website, '') FROM accounts WHERE id = $1`, accountID).
		Scan(&publicID, &name, &website)
	return
}

func (r *Repository) UserName(ctx context.Context, userID int64) (string, error) {
	var name string
	err := r.pool.QueryRow(ctx, `SELECT full_name FROM users WHERE id = $1`, userID).Scan(&name)
	return name, err
}

func (r *Repository) PublisherAccountID(ctx context.Context) (int64, error) {
	var id int64
	err := r.pool.QueryRow(ctx, `SELECT id FROM accounts WHERE type = 'publisher' LIMIT 1`).Scan(&id)
	return id, err
}

func (r *Repository) FindPublisherUserByEmail(ctx context.Context, publisherID int64, email string) (userID int64, role string, active bool, err error) {
	err = r.pool.QueryRow(ctx,
		`SELECT u.id, u.role, u.is_active FROM users u
		 WHERE u.email = $1 AND u.account_id = $2`, email, publisherID).Scan(&userID, &role, &active)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, "", false, ErrNotFound
	}
	return
}

func (r *Repository) GetAccountPublicID(ctx context.Context, accountID int64) (string, error) {
	var pubID string
	err := r.pool.QueryRow(ctx, `SELECT public_id FROM accounts WHERE id = $1`, accountID).Scan(&pubID)
	return pubID, err
}

func (r *Repository) GetAccountByPublicID(ctx context.Context, publicID string) (id int64, accountType string, err error) {
	err = r.pool.QueryRow(ctx, `SELECT id, type FROM accounts WHERE public_id = $1`, publicID).Scan(&id, &accountType)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, "", ErrNotFound
	}
	return
}

func (r *Repository) GetUserBasic(ctx context.Context, userID int64) (email, fullName string, err error) {
	err = r.pool.QueryRow(ctx, `SELECT email, full_name FROM users WHERE id = $1`, userID).Scan(&email, &fullName)
	return
}

func (r *Repository) LoadBuyerPrincipal(ctx context.Context, buyerAccountID int64) (accountPublicID string, err error) {
	err = r.pool.QueryRow(ctx,
		`SELECT public_id FROM accounts WHERE id = $1 AND type = 'buyer'`, buyerAccountID).Scan(&accountPublicID)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrNotFound
	}
	return
}
