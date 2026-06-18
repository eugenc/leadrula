package webhooks

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/echayko/leadrula/backend/internal/database"
	"github.com/echayko/leadrula/backend/internal/leads"
)

func TestIngest_createAction_addsNoteFromPayload(t *testing.T) {
	ctx := context.Background()
	pool, err := database.Connect(ctx, "postgres://crm:crm@localhost:5432/crm?sslmode=disable")
	if err != nil {
		t.Skip(err)
	}
	defer pool.Close()

	repo := leads.NewRepository(pool)
	svc := NewService(pool, repo, nil, testEncKey(t), nil)

	var accountID int64
	if err := pool.QueryRow(ctx, `SELECT id FROM accounts ORDER BY id LIMIT 1`).Scan(&accountID); err != nil {
		t.Skip(err)
	}

	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	webhookName := "Note ingest test " + suffix
	falseVal := false
	wh, _, err := svc.Create(ctx, accountID, CreateWebhookInput{
		Name:            webhookName,
		Slug:            "note-ingest-test-" + suffix,
		OutboundEnabled: &falseVal,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = svc.Delete(ctx, accountID, wh.ID) }()

	dupUpdate := "update"
	noteKey := "comments"
	event, err := svc.CreateEvent(ctx, wh.ID, CreateEventParams{
		Action:        "create",
		DuplicateMode: &dupUpdate,
		NoteSourceKey: &noteKey,
	})
	if err != nil {
		t.Fatalf("CreateEvent: %v", err)
	}

	extField := "external_id"
	if _, err := svc.AddFieldMap(ctx, event.ID, "uuid", "builtin", &extField, nil); err != nil {
		t.Fatalf("AddFieldMap: %v", err)
	}

	extID := "note-test-" + suffix
	auth := &WebhookAuth{WebhookID: wh.ID, AccountID: accountID}

	first, err := svc.Ingest(ctx, auth, wh.Slug, map[string]any{
		"uuid":     extID,
		"comments": "hello from webhook",
	})
	if err != nil {
		t.Fatalf("first ingest: %v", err)
	}
	if first.Status != "processed" {
		t.Fatalf("first status = %q, want processed", first.Status)
	}

	var leadID int64
	if err := pool.QueryRow(ctx,
		`SELECT id FROM leads WHERE owner_account_id=$1 AND external_id=$2 AND deleted_at IS NULL`,
		accountID, extID,
	).Scan(&leadID); err != nil {
		t.Fatalf("lookup lead: %v", err)
	}

	var noteCount int
	if err := pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM lead_notes WHERE lead_id=$1`, leadID,
	).Scan(&noteCount); err != nil {
		t.Fatal(err)
	}
	if noteCount != 1 {
		t.Fatalf("note count = %d, want 1", noteCount)
	}

	var authorName, body string
	if err := pool.QueryRow(ctx,
		`SELECT COALESCE(author_name, ''), body FROM lead_notes WHERE lead_id=$1 ORDER BY created_at DESC LIMIT 1`,
		leadID,
	).Scan(&authorName, &body); err != nil {
		t.Fatalf("query note: %v", err)
	}
	if authorName != webhookName {
		t.Fatalf("author_name = %q, want %q", authorName, webhookName)
	}
	if body != "hello from webhook" {
		t.Fatalf("body = %q, want hello from webhook", body)
	}

	second, err := svc.Ingest(ctx, auth, wh.Slug, map[string]any{
		"uuid": extID,
	})
	if err != nil {
		t.Fatalf("second ingest: %v", err)
	}
	if second.Status != "processed" {
		t.Fatalf("second status = %q, want processed", second.Status)
	}

	var totalNotes int
	if err := pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM lead_notes WHERE lead_id=$1`, leadID,
	).Scan(&totalNotes); err != nil {
		t.Fatal(err)
	}
	if totalNotes != 1 {
		t.Fatalf("note count after empty payload = %d, want 1", totalNotes)
	}
}
