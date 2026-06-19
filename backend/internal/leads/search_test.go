package leads

import (
	"context"
	"testing"

	"github.com/echayko/leadrula/backend/internal/auth"
)

func TestList_withSearchQuery(t *testing.T) {
	repo := connectOversightTestDB(t)
	ctx := context.Background()

	var accountID int64
	var accountType string
	err := repo.pool.QueryRow(ctx,
		`SELECT owner_account_id, a.type::text
		 FROM leads l
		 JOIN accounts a ON a.id = l.owner_account_id
		 WHERE l.deleted_at IS NULL
		 LIMIT 1`).Scan(&accountID, &accountType)
	if err != nil {
		t.Skip("no lead fixture")
	}

	p := &auth.Principal{
		AccountID:   accountID,
		AccountType: accountType,
		Role:        "admin",
	}

	for _, term := range []string{"test", "review", "in review"} {
		t.Run(term, func(t *testing.T) {
			result, err := repo.List(ctx, p, ListOptions{
				ListFilters: ListFilters{Search: term},
				Page:        1,
				Limit:       10,
			})
			if err != nil {
				t.Fatalf("List with Search=%q: %v", term, err)
			}
			if result == nil {
				t.Fatal("expected non-nil result")
			}
		})
	}
}
