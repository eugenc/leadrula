package webhooks

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/echayko/leadrula/backend/internal/database"
)

func TestUpdateEvent_createToUpdate_clearsDuplicateMode(t *testing.T) {
	ctx := context.Background()
	pool, err := database.Connect(ctx, "postgres://crm:crm@localhost:5432/crm?sslmode=disable")
	if err != nil {
		t.Skip(err)
	}
	defer pool.Close()

	svc := NewService(pool, nil, nil, testEncKey(t), nil)

	var accountID int64
	if err := pool.QueryRow(ctx, `SELECT id FROM accounts ORDER BY id LIMIT 1`).Scan(&accountID); err != nil {
		t.Skip(err)
	}

	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	falseVal := false
	wh, _, err := svc.Create(ctx, accountID, CreateWebhookInput{
		Name:            "Action switch test " + suffix,
		Slug:            "action-switch-test-" + suffix,
		OutboundEnabled: &falseVal,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = svc.Delete(ctx, accountID, wh.ID) }()

	dupReject := "reject"
	created, err := svc.CreateEvent(ctx, wh.ID, CreateEventParams{
		Action:        "create",
		DuplicateMode: &dupReject,
	})
	if err != nil {
		t.Fatalf("CreateEvent: %v", err)
	}
	if created.DuplicateMode == nil || *created.DuplicateMode != "reject" {
		t.Fatalf("expected duplicate_mode reject, got %+v", created.DuplicateMode)
	}

	action := "update"
	lookupBy := "external_id"
	updated, err := svc.UpdateEvent(ctx, wh.ID, created.ID, UpdateEventParams{
		Action:   &action,
		LookupBy: &lookupBy,
	})
	if err != nil {
		t.Fatalf("UpdateEvent create→update: %v", err)
	}
	if updated.Action != "update" {
		t.Fatalf("action = %q", updated.Action)
	}
	if updated.DuplicateMode != nil {
		t.Fatalf("duplicate_mode should be cleared, got %+v", updated.DuplicateMode)
	}
	if updated.LookupBy == nil || *updated.LookupBy != "external_id" {
		t.Fatalf("lookup_by = %+v", updated.LookupBy)
	}
}

func TestUpdateEvent_updateToCreate_clearsLookupBy(t *testing.T) {
	ctx := context.Background()
	pool, err := database.Connect(ctx, "postgres://crm:crm@localhost:5432/crm?sslmode=disable")
	if err != nil {
		t.Skip(err)
	}
	defer pool.Close()

	svc := NewService(pool, nil, nil, testEncKey(t), nil)

	var accountID int64
	if err := pool.QueryRow(ctx, `SELECT id FROM accounts ORDER BY id LIMIT 1`).Scan(&accountID); err != nil {
		t.Skip(err)
	}

	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	falseVal := false
	wh, _, err := svc.Create(ctx, accountID, CreateWebhookInput{
		Name:            "Action switch reverse test " + suffix,
		Slug:            "action-switch-reverse-" + suffix,
		OutboundEnabled: &falseVal,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = svc.Delete(ctx, accountID, wh.ID) }()

	lookupBy := "external_id"
	created, err := svc.CreateEvent(ctx, wh.ID, CreateEventParams{
		Action:   "update",
		LookupBy: &lookupBy,
	})
	if err != nil {
		t.Fatalf("CreateEvent: %v", err)
	}

	action := "create"
	dupReject := "reject"
	updated, err := svc.UpdateEvent(ctx, wh.ID, created.ID, UpdateEventParams{
		Action:        &action,
		DuplicateMode: &dupReject,
	})
	if err != nil {
		t.Fatalf("UpdateEvent update→create: %v", err)
	}
	if updated.Action != "create" {
		t.Fatalf("action = %q", updated.Action)
	}
	if updated.LookupBy != nil {
		t.Fatalf("lookup_by should be cleared, got %+v", updated.LookupBy)
	}
	if updated.DuplicateMode == nil || *updated.DuplicateMode != "reject" {
		t.Fatalf("duplicate_mode = %+v", updated.DuplicateMode)
	}
}
