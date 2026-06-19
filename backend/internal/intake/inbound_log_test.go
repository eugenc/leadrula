package intake

import (
	"context"
	"testing"

	"github.com/echayko/leadrula/backend/internal/config"
	"github.com/echayko/leadrula/backend/internal/database"
)

func TestListInboundLog_sourceIncludesAutoRouted(t *testing.T) {
	cfg := config.Load()
	ctx := context.Background()
	pool, err := database.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(func() { pool.Close() })

	var ownerID int64
	var execID int64
	var leadID int64
	var triggerLabel string
	err = pool.QueryRow(ctx,
		`SELECT e.owner_account_id, e.id, e.lead_id, COALESCE(e.trigger_label, '')
		 FROM route_executions e
		 WHERE e.trigger_type = 'source_ingest'
		   AND NOT EXISTS (SELECT 1 FROM lead_intake_queue q WHERE q.lead_id = e.lead_id)
		 ORDER BY e.created_at DESC
		 LIMIT 1`).Scan(&ownerID, &execID, &leadID, &triggerLabel)
	if err != nil {
		t.Skip("no auto-routed source_ingest execution without queue row in database")
	}

	svc := &Service{pool: pool}

	all, err := svc.ListInboundLog(ctx, ownerID, ListInboundLogParams{
		AccountType: "publisher",
		Type:        "source",
		Status:      "all",
		Page:        1,
		Limit:       50,
	})
	if err != nil {
		t.Fatalf("ListInboundLog source all: %v", err)
	}
	found := false
	for _, it := range all.Items {
		if it.Kind == "source" && it.ID == execID {
			found = true
			if it.Origin != triggerLabel && triggerLabel != "" {
				t.Fatalf("origin = %q, want %q", it.Origin, triggerLabel)
			}
			if it.Status != "routed" {
				t.Fatalf("status = %q, want routed", it.Status)
			}
			break
		}
	}
	if !found {
		t.Fatalf("auto-routed source_ingest execution %d not in source log (total=%d)", execID, all.Total)
	}

	pending, err := svc.ListInboundLog(ctx, ownerID, ListInboundLogParams{
		AccountType: "publisher",
		Type:        "source",
		Status:      "pending_review",
		Page:        1,
		Limit:       50,
	})
	if err != nil {
		t.Fatalf("ListInboundLog source pending: %v", err)
	}
	for _, it := range pending.Items {
		if it.ID == execID {
			t.Fatalf("auto-routed execution %d should not appear for pending_review filter", execID)
		}
	}

	routed, err := svc.ListInboundLog(ctx, ownerID, ListInboundLogParams{
		AccountType: "publisher",
		Type:        "source",
		Status:      "routed",
		Page:        1,
		Limit:       50,
	})
	if err != nil {
		t.Fatalf("ListInboundLog source routed: %v", err)
	}
	foundRouted := false
	for _, it := range routed.Items {
		if it.ID == execID {
			foundRouted = true
			break
		}
	}
	if !foundRouted {
		t.Fatalf("auto-routed execution %d not in routed source log (total=%d)", execID, routed.Total)
	}
}
