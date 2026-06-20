package customfields

import (
	"context"
	"errors"
	"time"

	"github.com/echayko/leadrula/backend/pkg/httpx"
	"github.com/jackc/pgx/v5"
)

type CustomFieldFolder struct {
	ID        int64     `json:"id"`
	PublicID  string    `json:"public_id"`
	AccountID int64     `json:"-"`
	Name      string    `json:"name"`
	Position  int       `json:"position"`
	CreatedAt time.Time `json:"created_at"`
}

const folderCols = `id, public_id, account_id, name, position, created_at`

func scanFolder(row pgx.Row, f *CustomFieldFolder) error {
	return row.Scan(&f.ID, &f.PublicID, &f.AccountID, &f.Name, &f.Position, &f.CreatedAt)
}

func (s *Service) ListFolders(ctx context.Context, accountID int64) ([]CustomFieldFolder, error) {
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

func (s *Service) UpdateFolder(ctx context.Context, accountID, id int64, name *string, position *int) (*CustomFieldFolder, error) {
	f := &CustomFieldFolder{}
	err := scanFolder(s.pool.QueryRow(ctx,
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
	Folders []FolderPosition `json:"folders"`
	Fields  []FieldPlacement `json:"fields"`
}

// SaveLayout applies folder ordering and field placements in one transaction.
// Every id must belong to the account or the whole update is rejected.
func (s *Service) SaveLayout(ctx context.Context, accountID int64, layout Layout) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	for _, fld := range layout.Folders {
		ct, err := tx.Exec(ctx,
			`UPDATE custom_field_folders SET position = $3 WHERE id = $1 AND account_id = $2`,
			fld.ID, accountID, fld.Position)
		if err != nil {
			return err
		}
		if ct.RowsAffected() == 0 {
			return httpx.NotFound("folder not found")
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
