package leads

import (
	"context"
	"testing"
	"time"

	"github.com/echayko/leadrula/backend/internal/auth"
)

func TestTransferKindFromTrigger(t *testing.T) {
	tests := []struct {
		trigger string
		want    string
	}{
		{"return", "returned"},
		{"redistribute", "redistributed"},
		{"stage", "sold"},
		{"preassigned", "sold"},
		{"legacy_buyer", "sold"},
	}
	for _, tc := range tests {
		if got := transferKindFromTrigger(tc.trigger); got != tc.want {
			t.Fatalf("trigger %q: got %q want %q", tc.trigger, got, tc.want)
		}
	}
}

func TestFilterLeadHistory_buyerHidesPublisherNames(t *testing.T) {
	pubName := "LeadGen Co"
	buyerName := "Acme Solar"
	pubType := "publisher"
	sold := "sold"
	returned := "returned"

	entries := []LeadHistoryEntry{
		{
			ID:          1,
			Kind:        "stage_change",
			CreatedAt:   time.Now(),
			AccountName: &pubName,
			AccountType: &pubType,
		},
		{
			ID:              2,
			Kind:            "account_transfer",
			CreatedAt:       time.Now(),
			TransferKind:    &sold,
			FromAccountName: &pubName,
			ToAccountName:   &buyerName,
		},
		{
			ID:              3,
			Kind:            "account_transfer",
			CreatedAt:       time.Now(),
			TransferKind:    &returned,
			FromAccountName: &buyerName,
			ToAccountName:   &pubName,
		},
	}

	filtered := FilterLeadHistory(&auth.Principal{AccountType: "buyer", AccountID: 99}, entries)

	if filtered[0].AccountName == nil || *filtered[0].AccountName != buyerPublisherLabel {
		t.Fatalf("stage publisher name = %v, want %q", filtered[0].AccountName, buyerPublisherLabel)
	}
	if filtered[1].FromAccountName == nil || *filtered[1].FromAccountName != buyerPublisherLabel {
		t.Fatalf("sold from = %v, want %q", filtered[1].FromAccountName, buyerPublisherLabel)
	}
	if filtered[1].ToAccountName == nil || *filtered[1].ToAccountName != buyerName {
		t.Fatalf("sold to = %v, want %q", filtered[1].ToAccountName, buyerName)
	}
	if filtered[2].FromAccountName == nil || *filtered[2].FromAccountName != buyerName {
		t.Fatalf("return from = %v, want %q", filtered[2].FromAccountName, buyerName)
	}
	if filtered[2].ToAccountName == nil || *filtered[2].ToAccountName != buyerPublisherLabel {
		t.Fatalf("return to = %v, want %q", filtered[2].ToAccountName, buyerPublisherLabel)
	}
}

func TestFilterLeadHistory_publisherSeesFullNames(t *testing.T) {
	pubName := "LeadGen Co"
	pubType := "publisher"
	entries := []LeadHistoryEntry{{
		ID:          1,
		Kind:        "stage_change",
		CreatedAt:   time.Now(),
		AccountName: &pubName,
		AccountType: &pubType,
	}}

	filtered := FilterLeadHistory(&auth.Principal{AccountType: "publisher"}, entries)
	if filtered[0].AccountName == nil || *filtered[0].AccountName != pubName {
		t.Fatalf("publisher view name = %v, want %q", filtered[0].AccountName, pubName)
	}
}

func TestLeadHistory_infersAccountWhenOwnerNull(t *testing.T) {
	pool := connectLeadsTestDB(t)
	ctx := t.Context()
	repo := NewRepository(pool)

	var hasOwnerCol bool
	if err := pool.QueryRow(ctx,
		`SELECT EXISTS (
		   SELECT 1 FROM information_schema.columns
		   WHERE table_name = 'lead_stage_history' AND column_name = 'owner_account_id'
		 )`).Scan(&hasOwnerCol); err != nil || !hasOwnerCol {
		t.Skip("owner_account_id column not migrated")
	}

	var historyID, leadID int64
	err := pool.QueryRow(ctx,
		`SELECT h.id, h.lead_id FROM lead_stage_history h
		 JOIN leads l ON l.id = h.lead_id
		 WHERE l.deleted_at IS NULL
		 LIMIT 1`).Scan(&historyID, &leadID)
	if err != nil {
		t.Skip("no stage history rows")
	}

	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)

	var savedOwner *int64
	if err := tx.QueryRow(ctx,
		`SELECT owner_account_id FROM lead_stage_history WHERE id = $1`, historyID).Scan(&savedOwner); err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(ctx,
		`UPDATE lead_stage_history SET owner_account_id = NULL WHERE id = $1`, historyID); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(),
			`UPDATE lead_stage_history SET owner_account_id = $2 WHERE id = $1`, historyID, savedOwner)
	})

	entries, err := repo.LeadHistory(ctx, leadID)
	if err != nil {
		t.Fatal(err)
	}
	var found *LeadHistoryEntry
	for i := range entries {
		if entries[i].Kind == "stage_change" && entries[i].ID == historyID {
			found = &entries[i]
			break
		}
	}
	if found == nil {
		t.Fatal("history entry not found")
	}
	if found.AccountName == nil || *found.AccountName == "" {
		t.Fatalf("expected inferred account_name, got %v", found.AccountName)
	}
	if found.AccountType == nil || *found.AccountType == "" {
		t.Fatalf("expected inferred account_type, got %v", found.AccountType)
	}
}

func TestLeadHistory_mergesStageAndTransfer(t *testing.T) {
	pool := connectLeadsTestDB(t)
	ctx := t.Context()
	repo := NewRepository(pool)

	var hasOwnerCol bool
	if err := pool.QueryRow(ctx,
		`SELECT EXISTS (
		   SELECT 1 FROM information_schema.columns
		   WHERE table_name = 'lead_stage_history' AND column_name = 'owner_account_id'
		 )`).Scan(&hasOwnerCol); err != nil || !hasOwnerCol {
		t.Skip("owner_account_id column not migrated")
	}

	var leadID int64
	err := pool.QueryRow(ctx,
		`SELECT l.id FROM leads l
		 WHERE l.deleted_at IS NULL
		   AND EXISTS (SELECT 1 FROM lead_stage_history h WHERE h.lead_id = l.id)
		 LIMIT 1`).Scan(&leadID)
	if err != nil {
		t.Skip("no lead with stage history")
	}

	entries, err := repo.LeadHistory(ctx, leadID)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) == 0 {
		t.Fatal("expected history entries")
	}
	for i := 1; i < len(entries); i++ {
		if entries[i-1].CreatedAt.Before(entries[i].CreatedAt) {
			t.Fatalf("history not sorted desc at index %d", i)
		}
	}
}
