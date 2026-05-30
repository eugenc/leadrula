package leads

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/echayko/leadrula/backend/pkg/httpx"
	"github.com/jackc/pgx/v5"
)

const savedViewCols = `id, public_id, account_id, owner_user_id, name, placement, filters, columns, sort, sort_dir,
	is_builtin, builtin_key, position, created_by, created_at, updated_at`

func scanSavedView(row pgx.Row) (*SavedView, error) {
	v := &SavedView{}
	var filtersRaw, columnsRaw []byte
	var ownerID *int64
	var sort, sortDir, builtinKey *string
	err := row.Scan(
		&v.ID, &v.PublicID, &v.AccountID, &ownerID, &v.Name, &v.Placement, &filtersRaw, &columnsRaw,
		&sort, &sortDir, &v.IsBuiltin, &builtinKey, &v.Position, &v.CreatedBy, &v.CreatedAt, &v.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	v.OwnerUserID = ownerID
	v.Shared = ownerID == nil
	if len(filtersRaw) > 0 {
		_ = json.Unmarshal(filtersRaw, &v.Filters)
	}
	if v.Filters == nil {
		v.Filters = []FilterCondition{}
	}
	if len(columnsRaw) > 0 {
		_ = json.Unmarshal(columnsRaw, &v.Columns)
	}
	if sort != nil {
		v.Sort = *sort
	}
	if sortDir != nil {
		v.SortDir = *sortDir
	}
	if builtinKey != nil {
		v.BuiltinKey = *builtinKey
	}
	return v, nil
}

func (r *Repository) ListSavedViews(ctx context.Context, accountID, userID int64) ([]SavedView, error) {
	q := `SELECT ` + savedViewCols + ` FROM lead_saved_views
		WHERE account_id = $1 AND (owner_user_id = $2 OR owner_user_id IS NULL)
		ORDER BY position, name`
	rows, err := r.pool.Query(ctx, q, accountID, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []SavedView
	for rows.Next() {
		v, err := scanSavedView(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *v)
	}
	return out, rows.Err()
}

func (r *Repository) CountUserSavedViews(ctx context.Context, userID int64) (int, error) {
	var n int
	err := r.pool.QueryRow(ctx, `SELECT COUNT(*) FROM lead_saved_views WHERE owner_user_id = $1`, userID).Scan(&n)
	return n, err
}

func (r *Repository) GetSavedViewByPublicID(ctx context.Context, accountID int64, publicID string) (*SavedView, error) {
	q := `SELECT ` + savedViewCols + ` FROM lead_saved_views WHERE account_id = $1 AND public_id = $2::uuid`
	v, err := scanSavedView(r.pool.QueryRow(ctx, q, accountID, publicID))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, httpx.NotFound("view not found")
	}
	return v, err
}

type CreateSavedViewParams struct {
	AccountID   int64
	OwnerUserID *int64
	Name        string
	Placement   string
	Filters     []FilterCondition
	Columns     []string
	Sort        string
	SortDir     string
	CreatedBy   int64
}

func (r *Repository) CreateSavedView(ctx context.Context, p CreateSavedViewParams) (*SavedView, error) {
	filtersJSON, err := json.Marshal(p.Filters)
	if err != nil {
		return nil, err
	}
	var columnsJSON []byte
	if len(p.Columns) > 0 {
		columnsJSON, err = json.Marshal(p.Columns)
		if err != nil {
			return nil, err
		}
	}
	var sort, sortDir *string
	if p.Sort != "" {
		sort = &p.Sort
	}
	if p.SortDir != "" {
		sortDir = &p.SortDir
	}
	q := `INSERT INTO lead_saved_views(account_id, owner_user_id, name, placement, filters, columns, sort, sort_dir, created_by)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
		RETURNING ` + savedViewCols
	return scanSavedView(r.pool.QueryRow(ctx, q,
		p.AccountID, p.OwnerUserID, p.Name, p.Placement, filtersJSON, columnsJSON, sort, sortDir, p.CreatedBy,
	))
}

type UpdateSavedViewParams struct {
	Name      *string
	Placement *string
	Filters   []FilterCondition
	Columns   []string
	Sort      *string
	SortDir   *string
	SetCols   bool
	SetFilters bool
}

func (r *Repository) UpdateSavedView(ctx context.Context, id int64, p UpdateSavedViewParams) (*SavedView, error) {
	var filtersJSON, columnsJSON []byte
	var err error
	if p.SetFilters {
		filtersJSON, err = json.Marshal(p.Filters)
		if err != nil {
			return nil, err
		}
	}
	if p.SetCols && len(p.Columns) > 0 {
		columnsJSON, err = json.Marshal(p.Columns)
		if err != nil {
			return nil, err
		}
	}
	q := `UPDATE lead_saved_views SET
		name = COALESCE($2, name),
		placement = COALESCE($3, placement),
		filters = CASE WHEN $4::boolean THEN $5 ELSE filters END,
		columns = CASE WHEN $6::boolean THEN $7 ELSE columns END,
		sort = COALESCE($8, sort),
		sort_dir = COALESCE($9, sort_dir),
		updated_at = $10
		WHERE id = $1
		RETURNING ` + savedViewCols
	return scanSavedView(r.pool.QueryRow(ctx, q,
		id, p.Name, p.Placement, p.SetFilters, filtersJSON, p.SetCols, columnsJSON,
		p.Sort, p.SortDir, time.Now(),
	))
}

func (r *Repository) DeleteSavedView(ctx context.Context, id int64) error {
	ct, err := r.pool.Exec(ctx, `DELETE FROM lead_saved_views WHERE id = $1`, id)
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return httpx.NotFound("view not found")
	}
	return nil
}

func legacyFlatToConditions(f map[string]any) []FilterCondition {
	var out []FilterCondition
	if s, ok := f["status"].(string); ok && s != "" {
		out = append(out, FilterCondition{Field: "status", Op: "equals", Value: mustJSON(s)})
	}
	if n, ok := asInt64(f["pipeline_id"]); ok && n != 0 {
		out = append(out, FilterCondition{Field: "pipeline_id", Op: "equals", Value: mustJSON(n)})
	}
	if n, ok := asInt64(f["stage_id"]); ok && n != 0 {
		out = append(out, FilterCondition{Field: "stage_id", Op: "equals", Value: mustJSON(n)})
	}
	if n, ok := asInt64(f["assigned"]); ok && n != 0 {
		out = append(out, FilterCondition{Field: "assigned_user_id", Op: "equals", Value: mustJSON(n)})
	}
	if s, ok := f["tag"].(string); ok && s != "" {
		out = append(out, FilterCondition{Field: "tags", Op: "contains", Value: mustJSON(s)})
	}
	if s, ok := f["source"].(string); ok && s != "" {
		out = append(out, FilterCondition{Field: "source", Op: "equals", Value: mustJSON(s)})
	} else if s, ok := f["campaign"].(string); ok && s != "" {
		out = append(out, FilterCondition{Field: "source", Op: "equals", Value: mustJSON(s)})
	}
	if s, ok := f["action_on"].(string); ok && s != "" {
		val := s
		if val == "today" {
			out = append(out, FilterCondition{Field: "action_at", Op: "on", Value: mustJSON("today")})
		} else {
			out = append(out, FilterCondition{Field: "action_at", Op: "on", Value: mustJSON(val)})
		}
	}
	if b, ok := f["action_overdue"].(bool); ok && b {
		out = append(out, FilterCondition{Field: "action_at", Op: "overdue"})
	}
	return out
}

func asInt64(v any) (int64, bool) {
	switch n := v.(type) {
	case float64:
		return int64(n), true
	case int64:
		return n, true
	case int:
		return int64(n), true
	default:
		return 0, false
	}
}

func columnsFromLegacy(raw any) []string {
	if raw == nil {
		return nil
	}
	switch cols := raw.(type) {
	case []any:
		out := make([]string, 0, len(cols))
		for _, c := range cols {
			if s, ok := c.(string); ok {
				out = append(out, s)
			}
		}
		return out
	case []string:
		return cols
	default:
		return nil
	}
}

func parseLegacyViewMap(v map[string]any) CreateSavedViewParams {
	p := CreateSavedViewParams{Name: fmt.Sprint(v["name"]), Placement: "list"}
	if filters, ok := v["filters"].(map[string]any); ok {
		p.Filters = legacyFlatToConditions(filters)
	}
	p.Columns = columnsFromLegacy(v["columns"])
	if s, ok := v["sort"].(string); ok {
		p.Sort = s
	}
	if s, ok := v["sort_dir"].(string); ok {
		p.SortDir = s
	}
	return p
}
