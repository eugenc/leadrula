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

func TestEnsureContactFolder_createsAtPositionZero(t *testing.T) {
	pool := connectFoldersTestDB(t)
	ctx := context.Background()
	svc := NewService(pool)
	accountID := firstAccountID(t, pool)

	folders, err := svc.ListFolders(ctx, accountID)
	if err != nil {
		t.Fatalf("ListFolders: %v", err)
	}

	var contact *CustomFieldFolder
	for i := range folders {
		if folders[i].IsSystem && folders[i].SystemKey != nil && *folders[i].SystemKey == contactSystemKey {
			contact = &folders[i]
			break
		}
	}
	if contact == nil {
		t.Fatal("expected contact system folder")
	}
	if contact.Name != "Contact" {
		t.Fatalf("expected name Contact, got %q", contact.Name)
	}
	if contact.Position != 0 {
		t.Fatalf("expected position 0, got %d", contact.Position)
	}

	// Idempotent: second call does not duplicate.
	folders2, err := svc.ListFolders(ctx, accountID)
	if err != nil {
		t.Fatalf("ListFolders again: %v", err)
	}
	count := 0
	for _, f := range folders2 {
		if f.IsSystem && f.SystemKey != nil && *f.SystemKey == contactSystemKey {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("expected exactly one contact folder, got %d", count)
	}
}

func TestDeleteFolder_rejectsSystemFolder(t *testing.T) {
	pool := connectFoldersTestDB(t)
	ctx := context.Background()
	svc := NewService(pool)
	accountID := firstAccountID(t, pool)

	folders, err := svc.ListFolders(ctx, accountID)
	if err != nil {
		t.Fatalf("ListFolders: %v", err)
	}
	var contactID int64
	for _, f := range folders {
		if f.IsSystem {
			contactID = f.ID
			break
		}
	}
	if contactID == 0 {
		t.Fatal("expected contact system folder")
	}

	if err := svc.DeleteFolder(ctx, accountID, contactID); err == nil {
		t.Fatal("expected error deleting system folder")
	}
}

func TestUpdateFolder_rejectsRenameSystemFolder(t *testing.T) {
	pool := connectFoldersTestDB(t)
	ctx := context.Background()
	svc := NewService(pool)
	accountID := firstAccountID(t, pool)

	folders, err := svc.ListFolders(ctx, accountID)
	if err != nil {
		t.Fatalf("ListFolders: %v", err)
	}
	var contactID int64
	for _, f := range folders {
		if f.IsSystem {
			contactID = f.ID
			break
		}
	}
	if contactID == 0 {
		t.Fatal("expected contact system folder")
	}

	newName := "Renamed"
	if _, err := svc.UpdateFolder(ctx, accountID, contactID, &newName, nil); err == nil {
		t.Fatal("expected error renaming system folder")
	}
}

func TestUpdateFolder_rejectsMoveSystemFolder(t *testing.T) {
	pool := connectFoldersTestDB(t)
	ctx := context.Background()
	svc := NewService(pool)
	accountID := firstAccountID(t, pool)

	folders, err := svc.ListFolders(ctx, accountID)
	if err != nil {
		t.Fatalf("ListFolders: %v", err)
	}
	var contactID int64
	for _, f := range folders {
		if f.IsSystem {
			contactID = f.ID
			break
		}
	}
	if contactID == 0 {
		t.Fatal("expected contact system folder")
	}

	pos := 5
	if _, err := svc.UpdateFolder(ctx, accountID, contactID, nil, &pos); err == nil {
		t.Fatal("expected error moving system folder")
	}
}

func TestSaveLayout_persistsContactBuiltinOrder(t *testing.T) {
	pool := connectFoldersTestDB(t)
	ctx := context.Background()
	svc := NewService(pool)
	accountID := firstAccountID(t, pool)

	custom := []string{"email", "phone", "first_name", "last_name", "address", "tags"}
	if err := svc.SaveLayout(ctx, accountID, Layout{ContactBuiltinOrder: custom}); err != nil {
		t.Fatalf("SaveLayout: %v", err)
	}

	folders, err := svc.ListFolders(ctx, accountID)
	if err != nil {
		t.Fatalf("ListFolders: %v", err)
	}
	var contact *CustomFieldFolder
	for i := range folders {
		if folders[i].IsSystem {
			contact = &folders[i]
			break
		}
	}
	if contact == nil {
		t.Fatal("expected contact folder")
	}
	if len(contact.ContactBuiltinOrder) != 6 ||
		contact.ContactBuiltinOrder[0] != "first_name" ||
		contact.ContactBuiltinOrder[1] != "last_name" ||
		contact.ContactBuiltinOrder[2] != "email" {
		t.Fatalf("expected normalized order with locked names first, got %v", contact.ContactBuiltinOrder)
	}

	if err := svc.SaveLayout(ctx, accountID, Layout{ContactBuiltinOrder: defaultContactBuiltinOrder}); err != nil {
		t.Fatalf("SaveLayout default: %v", err)
	}
	folders, _ = svc.ListFolders(ctx, accountID)
	for i := range folders {
		if folders[i].IsSystem {
			contact = &folders[i]
			break
		}
	}
	if contact.ContactBuiltinOrder != nil {
		t.Fatalf("expected NULL order after reset, got %v", contact.ContactBuiltinOrder)
	}
}

func TestSaveLayout_rejectsInvalidContactBuiltinOrder(t *testing.T) {
	pool := connectFoldersTestDB(t)
	ctx := context.Background()
	svc := NewService(pool)
	accountID := firstAccountID(t, pool)

	if err := svc.SaveLayout(ctx, accountID, Layout{ContactBuiltinOrder: []string{"bad_key"}}); err == nil {
		t.Fatal("expected validation error")
	}
}

func TestSaveLayout_pinsContactFolderAtZero(t *testing.T) {
	pool := connectFoldersTestDB(t)
	ctx := context.Background()
	svc := NewService(pool)
	accountID := firstAccountID(t, pool)

	folders, err := svc.ListFolders(ctx, accountID)
	if err != nil {
		t.Fatalf("ListFolders: %v", err)
	}
	var contactID int64
	var otherID int64
	for _, f := range folders {
		if f.IsSystem {
			contactID = f.ID
		} else if otherID == 0 {
			otherID = f.ID
		}
	}
	if contactID == 0 {
		t.Fatal("expected contact folder")
	}
	if otherID == 0 {
		other, err := svc.CreateFolder(ctx, accountID, fmt.Sprintf("pin-test-%d", time.Now().UnixNano()))
		if err != nil {
			t.Fatalf("CreateFolder: %v", err)
		}
		otherID = other.ID
		t.Cleanup(func() { _ = svc.DeleteFolder(context.Background(), accountID, otherID) })
	}

	// Client tries to put regular folder at 0 and contact elsewhere.
	layout := Layout{
		Folders: []FolderPosition{
			{ID: otherID, Position: 0},
			{ID: contactID, Position: 5},
		},
		ContactBuiltinOrder: defaultContactBuiltinOrder,
	}
	if err := svc.SaveLayout(ctx, accountID, layout); err != nil {
		t.Fatalf("SaveLayout: %v", err)
	}

	folders, err = svc.ListFolders(ctx, accountID)
	if err != nil {
		t.Fatalf("ListFolders after save: %v", err)
	}
	for _, f := range folders {
		if f.IsSystem && f.Position != 0 {
			t.Fatalf("contact folder position = %d, want 0", f.Position)
		}
		if f.ID == otherID && f.Position == 0 {
			t.Fatal("regular folder should not be at position 0")
		}
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
