package customfields

import (
	"context"
	"errors"
	"sort"
	"time"

	"github.com/echayko/leadrula/backend/pkg/httpx"
	"github.com/jackc/pgx/v5"
)

const contactSystemKey = "contact"

var defaultContactBuiltinOrder = []string{
	"first_name", "last_name", "phone", "email", "address", "tags",
}

var contactLockedBuiltinOrder = []string{"first_name", "last_name"}

var contactReorderableBuiltinOrder = []string{"phone", "email", "address", "tags"}

func normalizeContactBuiltinOrder(order []string) []string {
	seen := make(map[string]bool, len(contactReorderableBuiltinOrder))
	tail := make([]string, 0, len(contactReorderableBuiltinOrder))
	for _, key := range order {
		if key == "first_name" || key == "last_name" {
			continue
		}
		if !isReorderableContactKey(key) || seen[key] {
			continue
		}
		seen[key] = true
		tail = append(tail, key)
	}
	for _, key := range contactReorderableBuiltinOrder {
		if !seen[key] {
			tail = append(tail, key)
		}
	}
	out := make([]string, 0, len(defaultContactBuiltinOrder))
	out = append(out, contactLockedBuiltinOrder...)
	out = append(out, tail...)
	return out
}

func isReorderableContactKey(key string) bool {
	for _, allowed := range contactReorderableBuiltinOrder {
		if key == allowed {
			return true
		}
	}
	return false
}

type CustomFieldFolder struct {
	ID                  int64     `json:"id"`
	PublicID            string    `json:"public_id"`
	AccountID           int64     `json:"-"`
	Name                string    `json:"name"`
	Position            int       `json:"position"`
	IsSystem            bool      `json:"is_system"`
	SystemKey           *string   `json:"system_key"`
	ContactBuiltinOrder []string  `json:"contact_builtin_order,omitempty"`
	CreatedAt           time.Time `json:"created_at"`
}

const folderCols = `id, public_id, account_id, name, position, is_system, system_key, contact_builtin_order, created_at`

func scanFolder(row pgx.Row, f *CustomFieldFolder) error {
	return row.Scan(
		&f.ID, &f.PublicID, &f.AccountID, &f.Name, &f.Position, &f.IsSystem, &f.SystemKey,
		&f.ContactBuiltinOrder, &f.CreatedAt,
	)
}

func isDefaultContactBuiltinOrder(order []string) bool {
	if len(order) != len(defaultContactBuiltinOrder) {
		return false
	}
	for i, key := range defaultContactBuiltinOrder {
		if order[i] != key {
			return false
		}
	}
	return true
}

func isAllowedContactKey(key string) bool {
	for _, allowed := range defaultContactBuiltinOrder {
		if key == allowed {
			return true
		}
	}
	return false
}

func validateContactBuiltinOrder(order []string) error {
	for _, key := range order {
		if !isAllowedContactKey(key) {
			return httpx.Validation("invalid contact field key: " + key)
		}
	}
	normalized := normalizeContactBuiltinOrder(order)
	if len(normalized) != len(defaultContactBuiltinOrder) {
		return httpx.Validation("contact_builtin_order must include all six contact fields")
	}
	if normalized[0] != "first_name" || normalized[1] != "last_name" {
		return httpx.Validation("first_name and last_name must remain first in contact order")
	}
	seen := make(map[string]bool, len(normalized))
	for _, key := range normalized {
		ok := false
		for _, allowed := range defaultContactBuiltinOrder {
			if key == allowed {
				ok = true
				break
			}
		}
		if !ok {
			return httpx.Validation("invalid contact field key: " + key)
		}
		if seen[key] {
			return httpx.Validation("duplicate contact field key: " + key)
		}
		seen[key] = true
	}
	return nil
}

func (s *Service) ensureContactFolder(ctx context.Context, accountID int64) error {
	var exists bool
	if err := s.pool.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM custom_field_folders WHERE account_id = $1 AND system_key = $2)`,
		accountID, contactSystemKey).Scan(&exists); err != nil {
		return err
	}
	if exists {
		return nil
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx,
		`UPDATE custom_field_folders SET position = position + 1 WHERE account_id = $1 AND is_system = false`,
		accountID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx,
		`INSERT INTO custom_field_folders(account_id, name, position, is_system, system_key)
		 VALUES ($1, 'Contact', 0, true, $2)`,
		accountID, contactSystemKey); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *Service) pinContactFolder(ctx context.Context, accountID int64) error {
	_, err := s.pool.Exec(ctx,
		`UPDATE custom_field_folders SET position = 0
		 WHERE account_id = $1 AND system_key = $2 AND position != 0`,
		accountID, contactSystemKey)
	return err
}

func (s *Service) ListFolders(ctx context.Context, accountID int64) ([]CustomFieldFolder, error) {
	if err := s.ensureContactFolder(ctx, accountID); err != nil {
		return nil, err
	}
	if err := s.pinContactFolder(ctx, accountID); err != nil {
		return nil, err
	}
	rows, err := s.pool.Query(ctx,
		`SELECT `+folderCols+` FROM custom_field_folders WHERE account_id = $1 ORDER BY position, id`, accountID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []CustomFieldFolder
	for rows.Next() {
		var f CustomFieldFolder
		if err := scanFolder(rows, &f); err != nil {
			return nil, err
		}
		out = append(out, f)
	}
	return out, rows.Err()
}

func (s *Service) CreateFolder(ctx context.Context, accountID int64, name string) (*CustomFieldFolder, error) {
	f := &CustomFieldFolder{}
	err := scanFolder(s.pool.QueryRow(ctx,
		`INSERT INTO custom_field_folders(account_id, name, position)
		 VALUES ($1,$2, COALESCE((SELECT MAX(position)+1 FROM custom_field_folders WHERE account_id=$1),0))
		 RETURNING `+folderCols,
		accountID, name), f)
	if err != nil {
		return nil, err
	}
	return f, nil
}

func (s *Service) folderIsSystem(ctx context.Context, accountID, id int64) (bool, error) {
	var isSystem bool
	err := s.pool.QueryRow(ctx,
		`SELECT is_system FROM custom_field_folders WHERE id = $1 AND account_id = $2`, id, accountID).Scan(&isSystem)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, httpx.NotFound("folder not found")
	}
	return isSystem, err
}

func (s *Service) UpdateFolder(ctx context.Context, accountID, id int64, name *string, position *int) (*CustomFieldFolder, error) {
	isSystem, err := s.folderIsSystem(ctx, accountID, id)
	if err != nil {
		return nil, err
	}
	if isSystem {
		if name != nil {
			return nil, httpx.BusinessRule("system folder cannot be renamed")
		}
		if position != nil {
			return nil, httpx.BusinessRule("system folder cannot be moved")
		}
	}
	f := &CustomFieldFolder{}
	err = scanFolder(s.pool.QueryRow(ctx,
		`UPDATE custom_field_folders SET
		   name = COALESCE($3, name),
		   position = COALESCE($4, position)
		 WHERE id = $1 AND account_id = $2
		 RETURNING `+folderCols,
		id, accountID, name, position), f)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, httpx.NotFound("folder not found")
	}
	if err != nil {
		return nil, err
	}
	return f, nil
}

// DeleteFolder removes a folder; its fields are unassigned via ON DELETE SET NULL.
func (s *Service) DeleteFolder(ctx context.Context, accountID, id int64) error {
	isSystem, err := s.folderIsSystem(ctx, accountID, id)
	if err != nil {
		return err
	}
	if isSystem {
		return httpx.BusinessRule("system folder cannot be deleted")
	}
	ct, err := s.pool.Exec(ctx, `DELETE FROM custom_field_folders WHERE id = $1 AND account_id = $2`, id, accountID)
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return httpx.NotFound("folder not found")
	}
	return nil
}

type FolderPosition struct {
	ID       int64 `json:"id"`
	Position int   `json:"position"`
}

type FieldPlacement struct {
	ID       int64  `json:"id"`
	FolderID *int64 `json:"folder_id"`
	Position int    `json:"position"`
}

type Layout struct {
	Folders             []FolderPosition `json:"folders"`
	Fields              []FieldPlacement `json:"fields"`
	ContactBuiltinOrder []string         `json:"contact_builtin_order,omitempty"`
}

// SaveLayout applies folder ordering and field placements in one transaction.
func (s *Service) SaveLayout(ctx context.Context, accountID int64, layout Layout) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx,
		`UPDATE custom_field_folders SET position = 0
		 WHERE account_id = $1 AND system_key = $2`,
		accountID, contactSystemKey); err != nil {
		return err
	}

	var nonSystem []FolderPosition
	for _, fld := range layout.Folders {
		var isSystem bool
		if err := tx.QueryRow(ctx,
			`SELECT is_system FROM custom_field_folders WHERE id = $1 AND account_id = $2`,
			fld.ID, accountID).Scan(&isSystem); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return httpx.NotFound("folder not found")
			}
			return err
		}
		if !isSystem {
			nonSystem = append(nonSystem, fld)
		}
	}
	sort.Slice(nonSystem, func(i, j int) bool {
		if nonSystem[i].Position == nonSystem[j].Position {
			return nonSystem[i].ID < nonSystem[j].ID
		}
		return nonSystem[i].Position < nonSystem[j].Position
	})
	for i, fld := range nonSystem {
		pos := i + 1
		ct, err := tx.Exec(ctx,
			`UPDATE custom_field_folders SET position = $3 WHERE id = $1 AND account_id = $2`,
			fld.ID, accountID, pos)
		if err != nil {
			return err
		}
		if ct.RowsAffected() == 0 {
			return httpx.NotFound("folder not found")
		}
	}

	if layout.ContactBuiltinOrder != nil {
		if err := validateContactBuiltinOrder(layout.ContactBuiltinOrder); err != nil {
			return err
		}
		order := normalizeContactBuiltinOrder(layout.ContactBuiltinOrder)
		var stored []string
		if isDefaultContactBuiltinOrder(order) {
			stored = nil
		} else {
			stored = order
		}
		ct, err := tx.Exec(ctx,
			`UPDATE custom_field_folders SET contact_builtin_order = $1
			 WHERE account_id = $2 AND system_key = $3`,
			stored, accountID, contactSystemKey)
		if err != nil {
			return err
		}
		if ct.RowsAffected() == 0 {
			return httpx.NotFound("contact folder not found")
		}
	}

	for _, f := range layout.Fields {
		if f.FolderID != nil {
			var ok bool
			if err := tx.QueryRow(ctx,
				`SELECT EXISTS(SELECT 1 FROM custom_field_folders WHERE id = $1 AND account_id = $2)`,
				*f.FolderID, accountID).Scan(&ok); err != nil {
				return err
			}
			if !ok {
				return httpx.Validation("folder does not belong to account")
			}
		}
		ct, err := tx.Exec(ctx,
			`UPDATE custom_fields SET folder_id = $3, position = $4 WHERE id = $1 AND account_id = $2`,
			f.ID, accountID, f.FolderID, f.Position)
		if err != nil {
			return err
		}
		if ct.RowsAffected() == 0 {
			return httpx.NotFound("custom field not found")
		}
	}

	return tx.Commit(ctx)
}
