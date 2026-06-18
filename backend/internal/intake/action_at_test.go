package intake

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/echayko/leadrula/backend/internal/database"
	"github.com/echayko/leadrula/backend/internal/leads"
	"github.com/echayko/leadrula/backend/internal/routing"
)

func TestApplyPayloadMappings_actionAt(t *testing.T) {
	ctx := context.Background()
	pool, err := database.Connect(ctx, "postgres://crm:crm@localhost:5432/crm?sslmode=disable")
	if err != nil {
		t.Skip(err)
	}
	defer pool.Close()

	repo := leads.NewRepository(pool)
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)

	var accountID int64
	if err := tx.QueryRow(ctx, `SELECT id FROM accounts ORDER BY id LIMIT 1`).Scan(&accountID); err != nil {
		t.Skip(err)
	}

	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	raw := []byte(fmt.Sprintf(`{"first_name":"Action","last_name":"Map-%s"}`, suffix))
	leadID, _, err := repo.InsertLead(ctx, tx, accountID, accountID, "test-source", raw)
	if err != nil {
		t.Fatal(err)
	}

	actionAt := "2026-06-18T15:30:00Z"
	flat := map[string]any{
		"appt_time": actionAt,
	}
	maps := []routing.SourceFieldMapEntry{
		{SourceKey: "appt_time", TargetType: "builtin", BuiltinField: strPtr("action_at")},
	}

	if err := applyPayloadMappings(ctx, tx, repo, accountID, leadID, flat, maps); err != nil {
		t.Fatal(err)
	}

	lead, err := repo.GetByID(ctx, tx, leadID)
	if err != nil {
		t.Fatal(err)
	}
	if lead.ActionAt == nil {
		t.Fatal("expected action_at to be set")
	}
	if lead.ActionAt.UTC().Format(time.RFC3339) != actionAt {
		t.Fatalf("action_at = %v want %v", lead.ActionAt.UTC().Format(time.RFC3339), actionAt)
	}
}

func strPtr(s string) *string { return &s }
