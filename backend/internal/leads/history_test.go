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

func TestFilterLeadHistory_buyerScoped(t *testing.T) {
	pubName := "Florida Net"
	buyerName := "Sunbright Solar USA"
	otherBuyer := "Other Buyer"
	pubType := "publisher"
	buyerType := "buyer"
	sold := "sold"
	returned := "returned"
	redistributed := "redistributed"

	const pubID int64 = 1
	const buyerID int64 = 99
	const otherBuyerID int64 = 50

	base := time.Date(2026, 6, 18, 10, 0, 0, 0, time.UTC)

	entries := []LeadHistoryEntry{
		{
			ID:             1,
			Kind:           "stage_change",
			CreatedAt:      base.Add(-time.Hour),
			AccountName:    &pubName,
			AccountType:    &pubType,
			ownerAccountID: pubID,
		},
		{
			ID:             2,
			Kind:           "stage_change",
			CreatedAt:      base.Add(10 * time.Minute),
			AccountName:    &buyerName,
			AccountType:    &buyerType,
			ownerAccountID: buyerID,
		},
		{
			ID:              3,
			Kind:            "account_transfer",
			CreatedAt:       base,
			TransferKind:    &sold,
			FromAccountName: &pubName,
			ToAccountName:   &buyerName,
			fromAccountID:   pubID,
			toAccountID:     buyerID,
		},
		{
			ID:              4,
			Kind:            "account_transfer",
			CreatedAt:       base.Add(30 * time.Minute),
			TransferKind:    &returned,
			FromAccountName: &buyerName,
			ToAccountName:   &pubName,
			fromAccountID:   buyerID,
			toAccountID:     pubID,
		},
		{
			ID:              5,
			Kind:            "account_transfer",
			CreatedAt:       base.Add(-30 * time.Minute),
			TransferKind:    &sold,
			FromAccountName: &pubName,
			ToAccountName:   &otherBuyer,
			fromAccountID:   pubID,
			toAccountID:     otherBuyerID,
		},
		{
			ID:              6,
			Kind:            "account_transfer",
			CreatedAt:       base.Add(40 * time.Minute),
			TransferKind:    &redistributed,
			FromAccountName: &pubName,
			ToAccountName:   &otherBuyer,
			fromAccountID:   pubID,
			toAccountID:     otherBuyerID,
		},
		{
			ID:        7,
			Kind:      "lead_created",
			CreatedAt: base.Add(-2 * time.Hour),
		},
	}

	filtered := FilterLeadHistory(&auth.Principal{AccountType: "buyer", AccountID: buyerID}, entries)
	if len(filtered) != 3 {
		t.Fatalf("buyer filtered len = %d, want 3", len(filtered))
	}
	if filtered[0].ID != 2 {
		t.Fatalf("expected buyer stage row, got id %d", filtered[0].ID)
	}
	if filtered[1].ID != 3 {
		t.Fatalf("expected sold transfer, got id %d", filtered[1].ID)
	}
	if filtered[2].ID != 4 {
		t.Fatalf("expected returned transfer, got id %d", filtered[2].ID)
	}
}

func TestFilterLeadHistory_ownershipWindowExcludesPostReturn(t *testing.T) {
	sold := "sold"
	returned := "returned"
	base := time.Date(2026, 6, 18, 12, 0, 0, 0, time.UTC)
	pubName := "Pub"
	buyerName := "Buyer"

	entries := []LeadHistoryEntry{
		{
			ID:              1,
			Kind:            "account_transfer",
			CreatedAt:       base,
			TransferKind:    &sold,
			FromAccountName: &pubName,
			ToAccountName:   &buyerName,
			fromAccountID:   1,
			toAccountID:     99,
		},
		{
			ID:             2,
			Kind:           "stage_change",
			CreatedAt:      base.Add(10 * time.Minute),
			ownerAccountID: 99,
		},
		{
			ID:              3,
			Kind:            "account_transfer",
			CreatedAt:       base.Add(20 * time.Minute),
			TransferKind:    &returned,
			fromAccountID:   99,
			toAccountID:     1,
		},
		{
			ID:             4,
			Kind:           "stage_change",
			CreatedAt:      base.Add(30 * time.Minute),
			ownerAccountID: 1,
		},
	}

	filtered := FilterLeadHistory(&auth.Principal{AccountType: "buyer", AccountID: 99}, entries)
	if len(filtered) != 3 {
		t.Fatalf("filtered len = %d, want 3 (sold, buyer stage, returned)", len(filtered))
	}
	if filtered[len(filtered)-1].ID != 3 {
		t.Fatalf("expected returned transfer in window, got id %d", filtered[len(filtered)-1].ID)
	}
}

func TestFilterLeadHistory_noteVisibility(t *testing.T) {
	sold := "sold"
	returned := "returned"
	base := time.Date(2026, 6, 18, 14, 0, 0, 0, time.UTC)
	pubName := "Pub"
	buyerName := "Buyer"

	entries := []LeadHistoryEntry{
		{
			ID:              1,
			Kind:            "account_transfer",
			CreatedAt:       base,
			TransferKind:    &sold,
			FromAccountName: &pubName,
			ToAccountName:   &buyerName,
			fromAccountID:   1,
			toAccountID:     99,
		},
		{
			ID:        2,
			Kind:      "note_added",
			CreatedAt: base.Add(-time.Hour),
			Summary:   "Before sold",
		},
		{
			ID:        3,
			Kind:      "note_added",
			CreatedAt: base.Add(10 * time.Minute),
			Summary:   "During ownership",
		},
		{
			ID:              4,
			Kind:            "account_transfer",
			CreatedAt:       base.Add(20 * time.Minute),
			TransferKind:    &returned,
			fromAccountID:   99,
			toAccountID:     1,
		},
		{
			ID:        5,
			Kind:      "note_added",
			CreatedAt: base.Add(30 * time.Minute),
			Summary:   "After return",
		},
	}

	pubFiltered := FilterLeadHistory(&auth.Principal{AccountType: "publisher"}, entries)
	if len(pubFiltered) != 5 {
		t.Fatalf("publisher note filter len = %d, want 5", len(pubFiltered))
	}

	buyerFiltered := FilterLeadHistory(&auth.Principal{AccountType: "buyer", AccountID: 99}, entries)
	if len(buyerFiltered) != 3 {
		t.Fatalf("buyer note filter len = %d, want 3 (sold, during note, returned)", len(buyerFiltered))
	}
	for _, e := range buyerFiltered {
		if e.Kind == "note_added" && e.Summary != "During ownership" {
			t.Fatalf("buyer saw unexpected note: %q", e.Summary)
		}
	}
}

func TestLeadHistory_includesNotes(t *testing.T) {
	pool := connectLeadsTestDB(t)
	ctx := t.Context()
	repo := NewRepository(pool)

	var leadID, userID int64
	err := pool.QueryRow(ctx,
		`SELECT l.id, u.id FROM leads l
		 JOIN users u ON u.account_id = l.owner_account_id AND u.is_active
		 WHERE l.deleted_at IS NULL LIMIT 1`).Scan(&leadID, &userID)
	if err != nil {
		t.Skip("no lead/user pair")
	}

	body := "activity history note test"
	n, err := repo.AddNote(ctx, leadID, userID, body)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM lead_notes WHERE id = $1`, n.ID)
	})

	entries, err := repo.LeadHistory(ctx, leadID)
	if err != nil {
		t.Fatal(err)
	}
	var found bool
	for _, e := range entries {
		if e.Kind == "note_added" && e.ID == n.ID {
			found = true
			if e.Summary != body {
				t.Fatalf("summary = %q, want %q", e.Summary, body)
			}
			if e.ActorType != "user" {
				t.Fatalf("actor_type = %q, want user", e.ActorType)
			}
			break
		}
	}
	if !found {
		t.Fatal("note_added entry not found in LeadHistory")
	}
}

func TestFilterLeadHistory_oversightSeesFullTrail(t *testing.T) {
	pubName := "Florida Net"
	buyerName := "Sunbright Solar USA"
	pubType := "publisher"
	sold := "sold"

	const pubID int64 = 1
	const buyerID int64 = 99

	entries := []LeadHistoryEntry{
		{
			ID:             1,
			Kind:           "stage_change",
			CreatedAt:      time.Now(),
			AccountName:    &pubName,
			AccountType:    &pubType,
			ownerAccountID: pubID,
		},
		{
			ID:              2,
			Kind:            "account_transfer",
			CreatedAt:       time.Now(),
			TransferKind:    &sold,
			FromAccountName: &pubName,
			ToAccountName:   &buyerName,
			fromAccountID:   pubID,
			toAccountID:     buyerID,
		},
	}

	p := &auth.Principal{
		AccountType: "buyer",
		AccountID:   buyerID,
		Impersonator: &auth.Principal{
			AccountType: "publisher",
			AccountID:   pubID,
		},
	}
	filtered := FilterLeadHistory(p, entries)
	if len(filtered) != 2 {
		t.Fatalf("oversight filtered len = %d, want 2", len(filtered))
	}
	if filtered[0].AccountName == nil || *filtered[0].AccountName != pubName {
		t.Fatalf("oversight publisher name = %v, want %q", filtered[0].AccountName, pubName)
	}
}

func TestFilterLeadHistory_publisherSeesFullNames(t *testing.T) {
	pubName := "LeadGen Co"
	pubType := "publisher"
	entries := []LeadHistoryEntry{{
		ID:             1,
		Kind:           "stage_change",
		CreatedAt:      time.Now(),
		AccountName:    &pubName,
		AccountType:    &pubType,
		ownerAccountID: 1,
	}}

	filtered := FilterLeadHistory(&auth.Principal{AccountType: "publisher"}, entries)
	if len(filtered) != 1 {
		t.Fatalf("publisher filtered len = %d, want 1", len(filtered))
	}
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
	if found.ownerAccountID == 0 {
		t.Fatal("expected resolved owner_account_id")
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
