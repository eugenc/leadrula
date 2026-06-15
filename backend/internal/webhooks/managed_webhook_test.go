package webhooks

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/echayko/leadrula/backend/internal/database"
	"github.com/echayko/leadrula/backend/pkg/httpx"
)

func TestManagedWebhook_allowsEventUpdate_blocksMetadataEdit(t *testing.T) {
	ctx := context.Background()
	pool, err := database.Connect(ctx, "postgres://crm:crm@localhost:5432/crm?sslmode=disable")
	if err != nil {
		t.Skip(err)
	}
	defer pool.Close()

	svc := NewService(pool, nil, nil, testEncKey(t), nil)

	var accountID, webhookID, eventID int64
	err = pool.QueryRow(ctx,
		`SELECT w.account_id, w.id, we.id
		 FROM webhooks w
		 JOIN webhook_events we ON we.webhook_id = w.id
		 WHERE w.integration_connection_id IS NOT NULL
		   AND w.inbound_enabled = true
		 ORDER BY w.id DESC
		 LIMIT 1`,
	).Scan(&accountID, &webhookID, &eventID)
	if err != nil {
		t.Skip("no managed inbound webhook with events in database")
	}

	if err := svc.AssertUserEditableWebhook(ctx, accountID, webhookID); err == nil {
		t.Fatal("expected managed webhook metadata edit to be blocked")
	} else {
		var appErr *httpx.AppError
		if !errors.As(err, &appErr) || appErr.Code != httpx.CodeValidation {
			t.Fatalf("expected validation error, got %v", err)
		}
	}

	conditions := json.RawMessage(`[{"field":"action","op":"eq","value":"Update"}]`)
	logic := "and"
	updated, err := svc.UpdateEvent(ctx, webhookID, eventID, UpdateEventParams{
		ConditionLogic: &logic,
		Conditions:     conditions,
	})
	if err != nil {
		t.Fatalf("UpdateEvent on managed webhook: %v", err)
	}
	if updated.ConditionLogic != "and" {
		t.Fatalf("condition_logic = %q", updated.ConditionLogic)
	}
}

func TestManagedWebhook_fieldMapOwnership(t *testing.T) {
	ctx := context.Background()
	pool, err := database.Connect(ctx, "postgres://crm:crm@localhost:5432/crm?sslmode=disable")
	if err != nil {
		t.Skip(err)
	}
	defer pool.Close()

	svc := NewService(pool, nil, nil, testEncKey(t), nil)

	var accountID, mapID int64
	err = pool.QueryRow(ctx,
		`SELECT w.account_id, fm.id
		 FROM webhook_event_field_map fm
		 JOIN webhook_events we ON we.id = fm.event_id
		 JOIN webhooks w ON w.id = we.webhook_id
		 WHERE w.integration_connection_id IS NOT NULL
		 LIMIT 1`,
	).Scan(&accountID, &mapID)
	if err != nil {
		t.Skip("no field map on managed webhook in database")
	}

	if err := svc.AssertUserEditableFieldMap(ctx, accountID, mapID); err != nil {
		t.Fatalf("AssertUserEditableFieldMap: %v", err)
	}
	if err := svc.AssertUserEditableFieldMap(ctx, accountID, mapID+999999); err == nil {
		t.Fatal("expected not found for unknown map id")
	}
}
