// Package customfields manages admin-defined custom fields and the
// disqualification-reason picklist, both scoped per account.
package customfields

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/echayko/leadrula/backend/internal/database"
	"github.com/echayko/leadrula/backend/pkg/httpx"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type CustomField struct {
	ID        int64           `json:"id"`
	PublicID  string          `json:"public_id"`
	AccountID int64           `json:"-"`
	Name      string          `json:"name"`
	FieldKey  string          `json:"field_key"`
	Type      string          `json:"type"`
	Options   json.RawMessage `json:"options"`
	Position  int             `json:"position"`
	IsActive  bool            `json:"is_active"`
	CreatedAt time.Time       `json:"created_at"`
}

type DisqReason struct {
	ID        int64     `json:"id"`
	AccountID int64     `json:"-"`
	Label     string    `json:"label"`
	Position  int       `json:"position"`
	IsActive  bool      `json:"is_active"`
	CreatedAt time.Time `json:"created_at"`
}

type Service struct {
	pool *pgxpool.Pool
}

func NewService(pool *pgxpool.Pool) *Service { return &Service{pool: pool} }

// ── Custom fields ─────────────────────────────────────────────────

func (s *Service) ListFields(ctx context.Context, accountID int64) ([]CustomField, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT id, public_id, account_id, name, field_key, type, options, position, is_active, created_at
		 FROM custom_fields WHERE account_id = $1 ORDER BY position, id`, accountID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []CustomField
	for rows.Next() {
		var f CustomField
		if err := rows.Scan(&f.ID, &f.PublicID, &f.AccountID, &f.Name, &f.FieldKey, &f.Type, &f.Options, &f.Position, &f.IsActive, &f.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, f)
	}
	return out, rows.Err()
}

func (s *Service) CreateField(ctx context.Context, accountID int64, name, fieldKey, ftype string, options json.RawMessage) (*CustomField, error) {
	if !validType(ftype) {
		return nil, httpx.Validation("invalid field type")
	}
	if len(options) == 0 {
		options = json.RawMessage("[]")
	}
	f := &CustomField{}
	err := s.pool.QueryRow(ctx,
		`INSERT INTO custom_fields(account_id, name, field_key, type, options, position)
		 VALUES ($1,$2,$3,$4,$5, COALESCE((SELECT MAX(position)+1 FROM custom_fields WHERE account_id=$1),0))
		 RETURNING id, public_id, account_id, name, field_key, type, options, position, is_active, created_at`,
		accountID, name, fieldKey, ftype, options).Scan(
		&f.ID, &f.PublicID, &f.AccountID, &f.Name, &f.FieldKey, &f.Type, &f.Options, &f.Position, &f.IsActive, &f.CreatedAt)
	if err != nil {
		if database.IsUniqueViolation(err) {
			return nil, httpx.Conflict("field_key already exists")
		}
		return nil, err
	}
	return f, nil
}

func (s *Service) UpdateField(ctx context.Context, accountID, id int64, name *string, options json.RawMessage, position *int, isActive *bool) (*CustomField, error) {
	var optArg interface{}
	if len(options) > 0 {
		optArg = []byte(options)
	}
	f := &CustomField{}
	err := s.pool.QueryRow(ctx,
		`UPDATE custom_fields SET
		   name = COALESCE($3, name),
		   options = COALESCE($4, options),
		   position = COALESCE($5, position),
		   is_active = COALESCE($6, is_active)
		 WHERE id = $1 AND account_id = $2
		 RETURNING id, public_id, account_id, name, field_key, type, options, position, is_active, created_at`,
		id, accountID, name, optArg, position, isActive).Scan(
		&f.ID, &f.PublicID, &f.AccountID, &f.Name, &f.FieldKey, &f.Type, &f.Options, &f.Position, &f.IsActive, &f.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, httpx.NotFound("custom field not found")
	}
	return f, err
}

// DeleteField removes a field only if no lead has a value for it.
func (s *Service) DeleteField(ctx context.Context, accountID, id int64) error {
	var inUse bool
	if err := s.pool.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM lead_custom_values WHERE custom_field_id = $1)`, id).Scan(&inUse); err != nil {
		return err
	}
	if inUse {
		return httpx.BusinessRule("cannot delete a field that has values; deactivate it instead")
	}
	ct, err := s.pool.Exec(ctx, `DELETE FROM custom_fields WHERE id = $1 AND account_id = $2`, id, accountID)
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return httpx.NotFound("custom field not found")
	}
	return nil
}

// ── Disqualification reasons ──────────────────────────────────────

func (s *Service) ListReasons(ctx context.Context, accountID int64) ([]DisqReason, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT id, account_id, label, position, is_active, created_at
		 FROM disqualification_reasons WHERE account_id = $1 ORDER BY position, id`, accountID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []DisqReason
	for rows.Next() {
		var d DisqReason
		if err := rows.Scan(&d.ID, &d.AccountID, &d.Label, &d.Position, &d.IsActive, &d.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

func (s *Service) CreateReason(ctx context.Context, accountID int64, label string) (*DisqReason, error) {
	d := &DisqReason{}
	err := s.pool.QueryRow(ctx,
		`INSERT INTO disqualification_reasons(account_id, label, position)
		 VALUES ($1, $2, COALESCE((SELECT MAX(position)+1 FROM disqualification_reasons WHERE account_id=$1),0))
		 RETURNING id, account_id, label, position, is_active, created_at`,
		accountID, label).Scan(&d.ID, &d.AccountID, &d.Label, &d.Position, &d.IsActive, &d.CreatedAt)
	return d, err
}

func (s *Service) UpdateReason(ctx context.Context, accountID, id int64, label *string, position *int, isActive *bool) (*DisqReason, error) {
	d := &DisqReason{}
	err := s.pool.QueryRow(ctx,
		`UPDATE disqualification_reasons SET
		   label = COALESCE($3, label),
		   position = COALESCE($4, position),
		   is_active = COALESCE($5, is_active)
		 WHERE id = $1 AND account_id = $2
		 RETURNING id, account_id, label, position, is_active, created_at`,
		id, accountID, label, position, isActive).Scan(&d.ID, &d.AccountID, &d.Label, &d.Position, &d.IsActive, &d.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, httpx.NotFound("reason not found")
	}
	return d, err
}

// DeleteReason removes a reason only if it has never been recorded on a lead.
func (s *Service) DeleteReason(ctx context.Context, accountID, id int64) error {
	var inUse bool
	if err := s.pool.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM leads WHERE disqualification_reason_id = $1)`, id).Scan(&inUse); err != nil {
		return err
	}
	if inUse {
		return httpx.BusinessRule("cannot delete a reason in use; deactivate it instead")
	}
	ct, err := s.pool.Exec(ctx, `DELETE FROM disqualification_reasons WHERE id = $1 AND account_id = $2`, id, accountID)
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return httpx.NotFound("reason not found")
	}
	return nil
}

func validType(t string) bool {
	switch t {
	case "text", "number", "date", "datetime", "dropdown", "checkbox":
		return true
	}
	return false
}
