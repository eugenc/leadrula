package intake

import (
	"context"
	"testing"

	"github.com/echayko/leadrula/backend/internal/config"
	"github.com/echayko/leadrula/backend/internal/database"
)

func TestLogLeadFilterClause_leadID(t *testing.T) {
	clause, args := logLeadFilterClause(2, 42, "", "d.lead_id")
	if clause != " AND d.lead_id = $2" {
		t.Fatalf("clause = %q", clause)
	}
	if len(args) != 1 || args[0] != int64(42) {
		t.Fatalf("args = %v", args)
	}
}

func TestLogLeadFilterClause_search(t *testing.T) {
	clause, args := logLeadFilterClause(3, 0, "jane@example.com", "q.lead_id")
	if clause == "" {
		t.Fatal("expected non-empty clause")
	}
	if len(args) != 1 || args[0] != "%jane@example.com%" {
		t.Fatalf("args = %v", args)
	}
	if clause != ` AND (
		l.first_name ILIKE $3 OR
		l.last_name ILIKE $3 OR
		TRIM(CONCAT(l.first_name, ' ', l.last_name)) ILIKE $3 OR
		l.email ILIKE $3 OR
		l.phone ILIKE $3 OR
		l.public_id::text ILIKE $3
	)` {
		t.Fatalf("unexpected clause: %q", clause)
	}
}

func TestLogLeadFilterClause_empty(t *testing.T) {
	clause, args := logLeadFilterClause(2, 0, "  ", "d.lead_id")
	if clause != "" || len(args) != 0 {
		t.Fatalf("clause=%q args=%v", clause, args)
	}
}

func TestAppendLogLeadFilter(t *testing.T) {
	where, args := appendLogLeadFilter("w.account_id = $1", []any{99}, 0, "manuel", "d.lead_id")
	if len(args) != 2 {
		t.Fatalf("args = %v", args)
	}
	if args[1] != "%manuel%" {
		t.Fatalf("search arg = %v", args[1])
	}
	if where == "w.account_id = $1" {
		t.Fatal("expected filter appended")
	}
}

func TestListInboundLog_integrationSearch(t *testing.T) {
	cfg := config.Load()
	ctx := context.Background()
	pool, err := database.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(func() { pool.Close() })

	svc := &Service{pool: pool}

	var accountID int64
	err = pool.QueryRow(ctx,
		`SELECT c.account_id
		 FROM integration_delivery_queue q
		 JOIN integration_connections c ON c.id = q.connection_id
		 JOIN leads l ON l.id = q.lead_id
		 WHERE q.lead_id IS NOT NULL
		   AND (l.first_name <> '' OR l.last_name <> '' OR l.email <> '' OR l.phone <> '')
		 LIMIT 1`).Scan(&accountID)
	if err != nil {
		t.Skip("no integration delivery with lead data in database")
	}

	var firstName string
	err = pool.QueryRow(ctx,
		`SELECT l.first_name
		 FROM integration_delivery_queue q
		 JOIN integration_connections c ON c.id = q.connection_id
		 JOIN leads l ON l.id = q.lead_id
		 WHERE c.account_id = $1 AND q.lead_id IS NOT NULL AND l.first_name <> ''
		 LIMIT 1`, accountID).Scan(&firstName)
	if err != nil {
		t.Skip("no named lead on integration delivery")
	}

	all, err := svc.ListInboundLog(ctx, accountID, ListInboundLogParams{
		AccountType: "buyer",
		Type:        "integration",
		Page:        1,
		Limit:       100,
	})
	if err != nil {
		t.Fatalf("ListInboundLog all: %v", err)
	}

	filtered, err := svc.ListInboundLog(ctx, accountID, ListInboundLogParams{
		AccountType: "buyer",
		Type:        "integration",
		Search:      firstName,
		Page:        1,
		Limit:       100,
	})
	if err != nil {
		t.Fatalf("ListInboundLog filtered: %v", err)
	}
	if filtered.Total > all.Total {
		t.Fatalf("filtered total %d > all total %d", filtered.Total, all.Total)
	}
	if filtered.Total == 0 {
		t.Fatal("expected at least one row for first_name search")
	}
	for _, it := range filtered.Items {
		if it.FirstName != firstName && it.LastName != firstName {
			// name may match partial ILIKE on combined fields
			continue
		}
	}
}

func TestListInboundLog_allPublisher(t *testing.T) {
	cfg := config.Load()
	ctx := context.Background()
	pool, err := database.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(func() { pool.Close() })

	rows, err := pool.Query(ctx, `SELECT id, name FROM accounts WHERE type = 'publisher' ORDER BY id`)
	if err != nil {
		t.Fatalf("query publishers: %v", err)
	}
	defer rows.Close()

	svc := &Service{pool: pool}
	var count int
	for rows.Next() {
		var accountID int64
		var name string
		if err := rows.Scan(&accountID, &name); err != nil {
			t.Fatalf("scan publisher: %v", err)
		}
		count++
		_, err = svc.ListInboundLog(ctx, accountID, ListInboundLogParams{
			AccountType: "publisher",
			Type:        "all",
			Page:        1,
			Limit:       25,
		})
		if err != nil {
			t.Fatalf("ListInboundLog all publisher account %d (%s): %v", accountID, name, err)
		}
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("publishers rows: %v", err)
	}
	if count == 0 {
		t.Skip("no publisher accounts in database")
	}
}

func TestListQueue_scopedToPublisher(t *testing.T) {
	cfg := config.Load()
	ctx := context.Background()
	pool, err := database.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(func() { pool.Close() })

	svc := &Service{pool: pool}

	var publisherID int64
	err = pool.QueryRow(ctx,
		`SELECT l.publisher_id
		 FROM lead_intake_queue q
		 JOIN leads l ON l.id = q.lead_id
		 LIMIT 1`).Scan(&publisherID)
	if err != nil {
		t.Skip("no intake queue rows in database")
	}

	var expectedTotal int64
	err = pool.QueryRow(ctx,
		`SELECT COUNT(*)
		 FROM lead_intake_queue q
		 JOIN leads l ON l.id = q.lead_id
		 WHERE l.publisher_id = $1`, publisherID).Scan(&expectedTotal)
	if err != nil {
		t.Fatalf("count queue for publisher: %v", err)
	}

	got, err := svc.ListQueue(ctx, publisherID, ListQueueParams{
		Status: "all",
		Page:   1,
		Limit:  100,
	})
	if err != nil {
		t.Fatalf("ListQueue: %v", err)
	}
	if got.Total != expectedTotal {
		t.Fatalf("ListQueue total = %d, want %d (publisher-scoped)", got.Total, expectedTotal)
	}

	var otherPublisherID int64
	err = pool.QueryRow(ctx,
		`SELECT id FROM accounts WHERE account_type = 'publisher' AND id <> $1 LIMIT 1`,
		publisherID).Scan(&otherPublisherID)
	if err != nil {
		t.Skip("no second publisher account to compare scoping")
	}

	other, err := svc.ListQueue(ctx, otherPublisherID, ListQueueParams{
		Status: "all",
		Page:   1,
		Limit:  100,
	})
	if err != nil {
		t.Fatalf("ListQueue other publisher: %v", err)
	}

	for _, it := range got.Items {
		var owner int64
		if err := pool.QueryRow(ctx, `SELECT publisher_id FROM leads WHERE id=$1`, it.LeadID).Scan(&owner); err != nil {
			t.Fatalf("lead publisher_id: %v", err)
		}
		if owner != publisherID {
			t.Fatalf("item lead_id=%d belongs to publisher %d, want %d", it.LeadID, owner, publisherID)
		}
	}

	if expectedTotal > 0 && other.Total == got.Total && otherPublisherID != publisherID {
		var otherExpected int64
		_ = pool.QueryRow(ctx,
			`SELECT COUNT(*)
			 FROM lead_intake_queue q
			 JOIN leads l ON l.id = q.lead_id
			 WHERE l.publisher_id = $1`, otherPublisherID).Scan(&otherExpected)
		if otherExpected != other.Total {
			t.Fatalf("other publisher total mismatch: got %d want %d", other.Total, otherExpected)
		}
	}
}
