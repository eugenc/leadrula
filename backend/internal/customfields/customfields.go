// Package customfields manages admin-defined custom fields per account.
package customfields

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
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
	Format    *string         `json:"format"`
	Options   json.RawMessage `json:"options"`
	Position  int             `json:"position"`
	IsActive  bool            `json:"is_active"`
	FolderID  *int64          `json:"folder_id"`
	CreatedAt time.Time       `json:"created_at"`
}

const customFieldCols = `id, public_id, account_id, name, field_key, type, format, options, position, is_active, folder_id, created_at`

func scanField(row pgx.Row, f *CustomField) error {
	return row.Scan(&f.ID, &f.PublicID, &f.AccountID, &f.Name, &f.FieldKey, &f.Type,
		&f.Format, &f.Options, &f.Position, &f.IsActive, &f.FolderID, &f.CreatedAt)
}

type Service struct {
	pool *pgxpool.Pool
}

func NewService(pool *pgxpool.Pool) *Service { return &Service{pool: pool} }

// ── Custom fields ─────────────────────────────────────────────────

func (s *Service) ListFields(ctx context.Context, accountID int64) ([]CustomField, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT `+customFieldCols+` FROM custom_fields WHERE account_id = $1 ORDER BY position, id`, accountID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []CustomField
	for rows.Next() {
		var f CustomField
		if err := scanField(rows, &f); err != nil {
			return nil, err
		}
		out = append(out, f)
	}
	return out, rows.Err()
}

func (s *Service) CreateField(ctx context.Context, accountID int64, name, fieldKey, ftype string, options json.RawMessage, format *string) (*CustomField, error) {
	if !validType(ftype) {
		return nil, httpx.Validation("invalid field type")
	}
	if len(options) == 0 {
		options = json.RawMessage("[]")
	}
	resolvedFormat, err := resolveFormat(ftype, format)
	if err != nil {
		return nil, err
	}
	f := &CustomField{}
	err = scanField(s.pool.QueryRow(ctx,
		`INSERT INTO custom_fields(account_id, name, field_key, type, format, options, position)
		 VALUES ($1,$2,$3,$4,$5,$6, COALESCE((SELECT MAX(position)+1 FROM custom_fields WHERE account_id=$1),0))
		 RETURNING `+customFieldCols,
		accountID, name, fieldKey, ftype, resolvedFormat, options), f)
	if err != nil {
		if database.IsUniqueViolation(err) {
			return nil, httpx.Conflict("field_key already exists")
		}
		return nil, err
	}
	return f, nil
}

func (s *Service) UpdateField(ctx context.Context, accountID, id int64, name, fieldKey *string, options json.RawMessage, format *string, position *int, isActive *bool) (*CustomField, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	var oldKey, ftype string
	err = tx.QueryRow(ctx, `SELECT field_key, type FROM custom_fields WHERE id = $1 AND account_id = $2`, id, accountID).Scan(&oldKey, &ftype)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, httpx.NotFound("custom field not found")
	}
	if err != nil {
		return nil, err
	}

	if fieldKey != nil && *fieldKey != oldKey {
		if err := migrateStageRuleCustomFieldKey(ctx, tx, accountID, oldKey, *fieldKey); err != nil {
			return nil, err
		}
	}

	var optArg interface{}
	if len(options) > 0 {
		optArg = []byte(options)
	}
	var formatArg interface{}
	if format != nil {
		if ftype != "date" && ftype != "datetime" {
			return nil, httpx.Validation("format only applies to date fields")
		}
		if *format == "" {
			return nil, httpx.Validation("invalid date format")
		}
		if !ValidFormat(ftype, *format) {
			return nil, httpx.Validation("invalid date format")
		}
		formatArg = *format
	}
	f := &CustomField{}
	err = scanField(tx.QueryRow(ctx,
		`UPDATE custom_fields SET
		   name = COALESCE($3, name),
		   field_key = COALESCE($4, field_key),
		   options = COALESCE($5, options),
		   format = COALESCE($6, format),
		   position = COALESCE($7, position),
		   is_active = COALESCE($8, is_active)
		 WHERE id = $1 AND account_id = $2
		 RETURNING `+customFieldCols,
		id, accountID, name, fieldKey, optArg, formatArg, position, isActive), f)
	if err != nil {
		if database.IsUniqueViolation(err) {
			return nil, httpx.Conflict("field_key already exists")
		}
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return f, nil
}

// migrateStageRuleCustomFieldKey rewrites custom:{key} references in stage rule
// conditions/actions when a field key is renamed so existing filters keep working.
func migrateStageRuleCustomFieldKey(ctx context.Context, q database.Querier, accountID int64, oldKey, newKey string) error {
	oldField := "custom:" + oldKey
	newField := "custom:" + newKey

	rows, err := q.Query(ctx,
		`SELECT sr.id, sr.conditions, sr.actions
		 FROM stage_rules sr
		 JOIN pipeline_stages ps ON ps.id = sr.stage_id
		 JOIN pipelines p ON p.id = ps.pipeline_id
		 WHERE p.account_id = $1`, accountID)
	if err != nil {
		return err
	}
	defer rows.Close()

	type ruleRow struct {
		id         int64
		conditions json.RawMessage
		actions    json.RawMessage
	}
	var rules []ruleRow
	for rows.Next() {
		var r ruleRow
		if err := rows.Scan(&r.id, &r.conditions, &r.actions); err != nil {
			return err
		}
		rules = append(rules, r)
	}
	if err := rows.Err(); err != nil {
		return err
	}

	for _, r := range rules {
		conds, condChanged, err := patchRuleJSONFieldRefs(r.conditions, oldField, newField)
		if err != nil {
			return err
		}
		acts, actChanged, err := patchRuleJSONFieldRefs(r.actions, oldField, newField)
		if err != nil {
			return err
		}
		if !condChanged && !actChanged {
			continue
		}
		if _, err := q.Exec(ctx,
			`UPDATE stage_rules SET conditions = $2, actions = $3 WHERE id = $1`,
			r.id, conds, acts); err != nil {
			return err
		}
	}
	return nil
}

func patchRuleJSONFieldRefs(raw json.RawMessage, oldField, newField string) (json.RawMessage, bool, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return raw, false, nil
	}
	var items []map[string]json.RawMessage
	if err := json.Unmarshal(raw, &items); err != nil {
		return raw, false, err
	}
	changed := false
	for i, item := range items {
		fieldRaw, ok := item["field"]
		if !ok {
			continue
		}
		var field string
		if err := json.Unmarshal(fieldRaw, &field); err != nil {
			continue
		}
		if field != oldField {
			continue
		}
		b, err := json.Marshal(newField)
		if err != nil {
			return raw, false, err
		}
		items[i]["field"] = b
		changed = true
	}
	if !changed {
		return raw, false, nil
	}
	out, err := json.Marshal(items)
	return out, true, err
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

func validType(t string) bool {
	switch t {
	case "text", "number", "date", "datetime", "dropdown", "checkbox":
		return true
	}
	return false
}

// FieldTypesByAccount returns custom field id (string) → type for an account.
func FieldTypesByAccount(ctx context.Context, q database.Querier, accountID int64) (map[string]string, error) {
	rows, err := q.Query(ctx, `SELECT id, type FROM custom_fields WHERE account_id = $1`, accountID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]string{}
	for rows.Next() {
		var id int64
		var ftype string
		if err := rows.Scan(&id, &ftype); err != nil {
			return nil, err
		}
		out[fmt.Sprintf("%d", id)] = ftype
	}
	return out, rows.Err()
}
