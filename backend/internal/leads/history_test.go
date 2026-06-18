package leads

import (
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
