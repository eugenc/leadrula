package customfields

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/echayko/leadrula/backend/internal/config"
	"github.com/echayko/leadrula/backend/internal/database"
	"github.com/jackc/pgx/v5/pgxpool"
)

func connectFoldersTestDB(t *testing.T) *pgxpool.Pool {
	t.Helper()
	cfg := config.Load()
	pool, err := database.Connect(context.Background(), cfg.DatabaseURL)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(func() { pool.Close() })
	return pool
}

func firstAccountID(t *testing.T, pool *pgxpool.Pool) int64 {
	t.Helper()
	var id int64
	if err := pool.QueryRow(context.Background(), `SELECT id FROM accounts ORDER BY id LIMIT 1`).Scan(&id); err != nil {
		t.Skip("no account in database")
	}
	return id
}

func TestSaveLayout_assignsAndDeleteFolderUnassigns(t *testing.T) {
	pool := connectFoldersTestDB(t)
	ctx := context.Background()
	svc := NewService(pool)
	accountID := firstAccountID(t, pool)

	folder, err := svc.CreateFolder(ctx, accountID, fmt.Sprintf("folder-test-%d", time.Now().UnixNano()))
	if err != nil {
		t.Fatalf("CreateFolder: %v", err)
	}
	folderDeleted := false
	t.Cleanup(func() {
		if !folderDeleted {
			_, _ = pool.Exec(context.Background(), `DELETE FROM custom_field_folders WHERE id = $1`, folder.ID)
		}
	})

	key := fmt.Sprintf("layout_test_%d", time.Now().UnixNano())
	field, err := svc.CreateField(ctx, accountID, "Layout Test", key, "text", nil, nil)
	if err != nil {
		t.Fatalf("CreateField: %v", err)
	}
	t.Cleanup(func() { _ = svc.DeleteField(context.Background(), accountID, field.ID) })

	layout := Layout{
		Folders: []FolderPosition{{ID: folder.ID, Position: 0}},
		Fields:  []FieldPlacement{{ID: field.ID, FolderID: &folder.ID, Position: 0}},
	}
	if err := svc.SaveLayout(ctx, accountID, layout); err != nil {
		t.Fatalf("SaveLayout: %v", err)
	}

	assigned := folderIDForField(t, pool, field.ID)
	if assigned == nil || *assigned != folder.ID {
		t.Fatalf("expected field assigned to folder %d, got %v", folder.ID, assigned)
	}

	if err := svc.DeleteFolder(ctx, accountID, folder.ID); err != nil {
		t.Fatalf("DeleteFolder: %v", err)
	}
	folderDeleted = true

	if got := folderIDForField(t, pool, field.ID); got != nil {
		t.Fatalf("expected folder_id NULL after delete, got %v", *got)
	}
}

func TestSaveLayout_rejectsForeignFolder(t *testing.T) {
	pool := connectFoldersTestDB(t)
	ctx := context.Background()
	svc := NewService(pool)
	accountID := firstAccountID(t, pool)

	key := fmt.Sprintf("layout_foreign_%d", time.Now().UnixNano())
	field, err := svc.CreateField(ctx, accountID, "Foreign Test", key, "text", nil, nil)
	if err != nil {
		t.Fatalf("CreateField: %v", err)
	}
	t.Cleanup(func() { _ = svc.DeleteField(context.Background(), accountID, field.ID) })

	bogusFolderID := int64(-1)
	layout := Layout{Fields: []FieldPlacement{{ID: field.ID, FolderID: &bogusFolderID, Position: 0}}}
	if err := svc.SaveLayout(ctx, accountID, layout); err == nil {
		t.Fatal("expected error for folder not belonging to account")
	}
}

func folderIDForField(t *testing.T, pool *pgxpool.Pool, fieldID int64) *int64 {
	t.Helper()
	var folderID *int64
	if err := pool.QueryRow(context.Background(),
		`SELECT folder_id FROM custom_fields WHERE id = $1`, fieldID).Scan(&folderID); err != nil {
		t.Fatalf("query folder_id: %v", err)
	}
	return folderID
}
