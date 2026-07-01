package leads

import (
	"context"
	"errors"
	"strconv"
	"testing"

	"github.com/echayko/leadrula/backend/internal/auth"
	"github.com/echayko/leadrula/backend/internal/config"
	"github.com/echayko/leadrula/backend/internal/database"
	"github.com/echayko/leadrula/backend/internal/permissions"
	"github.com/echayko/leadrula/backend/pkg/httpx"
)

func connectOversightTestDB(t *testing.T) *Repository {
	t.Helper()
	cfg := config.Load()
	ctx := context.Background()
	pool, err := database.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(func() { pool.Close() })
	return NewRepository(pool)
}

func TestCollaborationLeadAllowed_buyerOriginBlocked(t *testing.T) {
	repo := connectOversightTestDB(t)
	ctx := context.Background()

	var buyerID int64
	err := repo.pool.QueryRow(ctx,
		`SELECT l.owner_account_id FROM leads l
		 WHERE l.owner_account_id = l.publisher_id AND l.deleted_at IS NULL
		 LIMIT 1`).Scan(&buyerID)
	if err != nil {
		t.Skip("no buyer-origin lead fixture")
	}

	pubID := buyerID + 999999
	p := &auth.Principal{
		AccountID:               buyerID,
		AccountType:             "buyer",
		SwitchedFromPublisherID: pubID,
	}
	l := &Lead{OwnerAccountID: buyerID, PublisherID: buyerID}
	if repo.CollaborationLeadAllowed(ctx, p, l) {
		t.Fatal("expected buyer-origin lead to be blocked under publisher oversight")
	}
}

func TestCollaborationLeadAllowed_publisherDistributed(t *testing.T) {
	repo := connectOversightTestDB(t)
	ctx := context.Background()

	var leadID, buyerID, pubID int64
	err := repo.pool.QueryRow(ctx,
		`SELECT l.id, l.owner_account_id, l.publisher_id
		 FROM leads l
		 JOIN contracts c ON c.id = l.contract_id
		 WHERE l.deleted_at IS NULL AND l.owner_account_id <> l.publisher_id
		   AND c.status = 'active' AND c.deleted_at IS NULL AND c.buyer_id IS NOT NULL
		   AND l.contract_id IS NOT NULL
		 LIMIT 1`).Scan(&leadID, &buyerID, &pubID)
	if err != nil {
		t.Skip("no publisher-distributed direct-contract lead fixture")
	}

	p := &auth.Principal{
		AccountID:               buyerID,
		AccountType:             "buyer",
		SwitchedFromPublisherID: pubID,
	}
	l, err := repo.GetByID(ctx, repo.pool, leadID)
	if err != nil {
		t.Fatal(err)
	}
	if !repo.CollaborationLeadAllowed(ctx, p, l) {
		t.Fatal("expected publisher-distributed lead to be allowed")
	}
}

func TestCollaborationLeadAllowed_participationLead(t *testing.T) {
	repo := connectOversightTestDB(t)
	ctx := context.Background()

	var leadID, buyerID, pubID int64
	err := repo.pool.QueryRow(ctx,
		`SELECT l.id, l.owner_account_id, l.publisher_id
		 FROM leads l
		 JOIN contracts c ON c.id = l.contract_id
		 JOIN contract_participations cp ON cp.contract_id = c.id AND cp.buyer_id = l.owner_account_id
		 WHERE l.deleted_at IS NULL AND l.owner_account_id <> l.publisher_id
		   AND c.status = 'active' AND c.buyer_id IS NULL AND c.deleted_at IS NULL
		   AND cp.status = 'active' AND l.contract_id IS NOT NULL
		 LIMIT 1`).Scan(&leadID, &buyerID, &pubID)
	if err != nil {
		t.Skip("no publisher-distributed participation lead fixture")
	}

	p := &auth.Principal{
		AccountID:               buyerID,
		AccountType:             "buyer",
		SwitchedFromPublisherID: pubID,
	}
	l, err := repo.GetByID(ctx, repo.pool, leadID)
	if err != nil {
		t.Fatal(err)
	}
	if !repo.CollaborationLeadAllowed(ctx, p, l) {
		t.Fatal("expected participation lead to be allowed under publisher oversight")
	}
}

func TestGetByRef_publisherCollaborationLead(t *testing.T) {
	repo := connectOversightTestDB(t)
	ctx := context.Background()

	var leadID, pubID int64
	err := repo.pool.QueryRow(ctx,
		`SELECT l.id, l.publisher_id
		 FROM leads l
		 JOIN buyer_collaborations bc
		   ON bc.publisher_id = l.publisher_id AND bc.buyer_id = l.owner_account_id AND bc.status = 'active'
		 WHERE l.deleted_at IS NULL AND l.owner_account_id <> l.publisher_id
		 LIMIT 1`).Scan(&leadID, &pubID)
	if err != nil {
		t.Skip("no buyer-owned lead with active collaboration fixture")
	}

	p := &auth.Principal{AccountID: pubID, AccountType: "publisher", Role: "admin"}
	if _, err := repo.GetByRef(ctx, p, strconv.FormatInt(leadID, 10)); err != nil {
		t.Fatalf("expected publisher to view collaborating buyer lead, got %v", err)
	}
}

func TestGetByRef_publisherNoCollaborationBlocked(t *testing.T) {
	repo := connectOversightTestDB(t)
	ctx := context.Background()

	var leadID, pubID int64
	err := repo.pool.QueryRow(ctx,
		`SELECT l.id, l.publisher_id
		 FROM leads l
		 WHERE l.deleted_at IS NULL AND l.owner_account_id <> l.publisher_id
		   AND l.status NOT IN ('distributed', 'closed')
		   AND NOT EXISTS (
		     SELECT 1 FROM buyer_collaborations bc
		     WHERE bc.publisher_id = l.publisher_id AND bc.buyer_id = l.owner_account_id AND bc.status = 'active'
		   )
		 LIMIT 1`).Scan(&leadID, &pubID)
	if err != nil {
		t.Skip("no buyer-owned lead without collaboration fixture")
	}

	p := &auth.Principal{AccountID: pubID, AccountType: "publisher", Role: "admin"}
	if _, err := repo.GetByRef(ctx, p, strconv.FormatInt(leadID, 10)); err == nil {
		t.Fatal("expected publisher to be blocked from non-collaborating buyer lead")
	}
}

func TestList_oversightScope_switchedFromPublisher(t *testing.T) {
	repo := connectOversightTestDB(t)
	ctx := context.Background()

	var buyerID, pubID int64
	err := repo.pool.QueryRow(ctx,
		`SELECT l.owner_account_id, l.publisher_id
		 FROM leads l
		 WHERE l.deleted_at IS NULL AND l.owner_account_id <> l.publisher_id
		 LIMIT 1`).Scan(&buyerID, &pubID)
	if err != nil {
		t.Skip("no publisher-distributed lead fixture")
	}

	p := &auth.Principal{
		AccountID:               buyerID,
		AccountType:             "buyer",
		Role:                    "admin",
		SwitchedFromPublisherID: pubID,
	}
	res, err := repo.List(ctx, p, ListOptions{All: true})
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range res.Items {
		if item.PublisherID != pubID {
			t.Fatalf("lead %d has publisher_id=%d, want %d", item.ID, item.PublisherID, pubID)
		}
	}
}

func TestPublisherUser_listAndGet_respectAssignedScope(t *testing.T) {
	repo := connectOversightTestDB(t)
	ctx := context.Background()

	var leadID, accountID, userID int64
	err := repo.pool.QueryRow(ctx,
		`SELECT l.id, l.owner_account_id, u.id
		 FROM leads l
		 JOIN users u ON u.id = l.assigned_user_id
		 JOIN accounts a ON a.id = u.account_id
		 WHERE l.deleted_at IS NULL
		   AND a.type = 'publisher'
		   AND u.role = 'user'
		   AND l.owner_account_id = u.account_id
		 LIMIT 1`).Scan(&leadID, &accountID, &userID)
	if err != nil {
		t.Skip("no publisher-owned lead assigned to publisher user fixture")
	}

	p := &auth.Principal{
		UserID:      userID,
		AccountID:   accountID,
		AccountType: "publisher",
		Role:        "user",
		Perms:       permissions.Resolve("user", "publisher", nil),
	}

	if _, err := repo.GetByRef(ctx, p, strconv.FormatInt(leadID, 10)); err != nil {
		t.Fatalf("expected assigned lead to be viewable, got %v", err)
	}

	res, err := repo.List(ctx, p, ListOptions{All: true})
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, item := range res.Items {
		if item.ID == leadID {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("expected assigned lead in list results")
	}

	var otherLeadID int64
	err = repo.pool.QueryRow(ctx,
		`SELECT l.id
		 FROM leads l
		 WHERE l.deleted_at IS NULL
		   AND l.owner_account_id = $1
		   AND (l.assigned_user_id IS NULL OR l.assigned_user_id <> $2)
		 LIMIT 1`, accountID, userID).Scan(&otherLeadID)
	if err != nil {
		t.Skip("no unassigned/other-assigned publisher lead for negative case")
	}

	_, err = repo.GetByRef(ctx, p, strconv.FormatInt(otherLeadID, 10))
	var appErr *httpx.AppError
	if !errors.As(err, &appErr) || appErr.Code != httpx.CodeForbidden {
		t.Fatalf("expected forbidden for unassigned lead, got %v", err)
	}
}

func TestPublisherAdmin_listAndGet_allScope(t *testing.T) {
	repo := connectOversightTestDB(t)
	ctx := context.Background()

	var leadID, accountID, userID int64
	err := repo.pool.QueryRow(ctx,
		`SELECT l.id, l.owner_account_id, u.id
		 FROM leads l
		 JOIN users u ON u.account_id = l.owner_account_id
		 JOIN accounts a ON a.id = u.account_id
		 WHERE l.deleted_at IS NULL
		   AND a.type = 'publisher'
		   AND u.role = 'admin'
		   AND l.owner_account_id = u.account_id
		 LIMIT 1`).Scan(&leadID, &accountID, &userID)
	if err != nil {
		t.Skip("no publisher-owned lead for admin fixture")
	}

	p := &auth.Principal{
		UserID:      userID,
		AccountID:   accountID,
		AccountType: "publisher",
		Role:        "admin",
		Perms:       permissions.Resolve("admin", "publisher", nil),
	}

	if _, err := repo.GetByRef(ctx, p, strconv.FormatInt(leadID, 10)); err != nil {
		t.Fatalf("expected admin to view lead, got %v", err)
	}

	res, err := repo.List(ctx, p, ListOptions{All: true})
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, item := range res.Items {
		if item.ID == leadID {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("expected lead in admin list results")
	}
}
