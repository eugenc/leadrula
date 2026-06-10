package dashboard

import (
	"context"
	"encoding/json"
	"time"

	"github.com/echayko/leadrula/backend/pkg/httpx"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const viewCols = `id, public_id, account_id, owner_user_id, name, widgets, period, goals, position, created_by, created_at, updated_at`

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

func scanView(row pgx.Row) (*View, error) {
	v := &View{}
	var widgetsRaw, goalsRaw []byte
	var ownerID *int64
	err := row.Scan(
		&v.ID, &v.PublicID, &v.AccountID, &ownerID, &v.Name, &widgetsRaw, &v.Period, &goalsRaw,
		&v.Position, &v.CreatedBy, &v.CreatedAt, &v.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	v.OwnerUserID = ownerID
	v.Shared = ownerID == nil
	if len(widgetsRaw) > 0 {
		_ = json.Unmarshal(widgetsRaw, &v.Widgets)
	}
	if v.Widgets == nil {
		v.Widgets = []string{}
	}
	if len(goalsRaw) > 0 {
		_ = json.Unmarshal(goalsRaw, &v.Goals)
	}
	return v, nil
}

func (r *Repository) ListSharedViews(ctx context.Context, accountID int64) ([]View, error) {
	q := `SELECT ` + viewCols + ` FROM dashboard_views
		WHERE account_id = $1 AND owner_user_id IS NULL
		ORDER BY position, name`
	rows, err := r.pool.Query(ctx, q, accountID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []View
	for rows.Next() {
		v, err := scanView(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *v)
	}
	return out, rows.Err()
}

func (r *Repository) GetByPublicID(ctx context.Context, accountID int64, publicID string) (*View, error) {
	q := `SELECT ` + viewCols + ` FROM dashboard_views WHERE account_id = $1 AND public_id = $2::uuid`
	v, err := scanView(r.pool.QueryRow(ctx, q, accountID, publicID))
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, httpx.NotFound("view not found")
		}
		return nil, err
	}
	return v, nil
}

type CreateParams struct {
	AccountID   int64
	OwnerUserID *int64
	Name        string
	Widgets     []string
	Period      string
	Goals       Goals
	Position    int
	CreatedBy   int64
}

func (r *Repository) Create(ctx context.Context, p CreateParams) (*View, error) {
	widgetsJSON, err := json.Marshal(p.Widgets)
	if err != nil {
		return nil, err
	}
	goalsJSON, err := json.Marshal(p.Goals)
	if err != nil {
		return nil, err
	}
	q := `INSERT INTO dashboard_views(account_id, owner_user_id, name, widgets, period, goals, position, created_by)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
		RETURNING ` + viewCols
	return scanView(r.pool.QueryRow(ctx, q,
		p.AccountID, p.OwnerUserID, p.Name, widgetsJSON, p.Period, goalsJSON, p.Position, p.CreatedBy,
	))
}

type UpdateParams struct {
	Name        *string
	Widgets     []string
	Period      *string
	Goals       *Goals
	Position    *int
	SetWidgets  bool
	SetGoals    bool
}

func (r *Repository) Update(ctx context.Context, id int64, p UpdateParams) (*View, error) {
	var widgetsJSON, goalsJSON []byte
	var err error
	if p.SetWidgets {
		widgetsJSON, err = json.Marshal(p.Widgets)
		if err != nil {
			return nil, err
		}
	}
	if p.SetGoals && p.Goals != nil {
		goalsJSON, err = json.Marshal(p.Goals)
		if err != nil {
			return nil, err
		}
	}
	q := `UPDATE dashboard_views SET
		name = COALESCE($2, name),
		widgets = CASE WHEN $3::boolean THEN $4 ELSE widgets END,
		period = COALESCE($5, period),
		goals = CASE WHEN $6::boolean THEN $7 ELSE goals END,
		position = COALESCE($8, position),
		updated_at = $9
		WHERE id = $1
		RETURNING ` + viewCols
	return scanView(r.pool.QueryRow(ctx, q,
		id, p.Name, p.SetWidgets, widgetsJSON, p.Period, p.SetGoals, goalsJSON, p.Position, time.Now(),
	))
}

func (r *Repository) Delete(ctx context.Context, id int64) error {
	ct, err := r.pool.Exec(ctx, `DELETE FROM dashboard_views WHERE id = $1`, id)
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return httpx.NotFound("view not found")
	}
	return nil
}
