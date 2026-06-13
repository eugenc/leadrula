package contracts

import (
	"context"
	"testing"
)

func TestListCompensations_excludesParticipationCopies(t *testing.T) {
	pool := connectContractsTestDB(t)
	ctx := context.Background()
	svc := NewService(pool)

	var publisherID, contractID int64
	var templateCount int
	err := pool.QueryRow(ctx,
		`SELECT c.publisher_id, c.id,
		        (SELECT COUNT(*)::int FROM contract_compensations
		         WHERE contract_id = c.id AND participation_id IS NULL)
		 FROM contracts c
		 WHERE c.status = 'active' AND c.buyer_id IS NULL AND c.deleted_at IS NULL
		   AND EXISTS (
		     SELECT 1 FROM contract_compensations cc
		     WHERE cc.contract_id = c.id AND cc.participation_id IS NOT NULL
		   )
		 LIMIT 1`).Scan(&publisherID, &contractID, &templateCount)
	if err != nil {
		t.Skip("no active open offer with participation compensation copies")
	}

	comps, err := svc.ListCompensations(ctx, publisherID, contractID)
	if err != nil {
		t.Fatal(err)
	}
	if len(comps) != templateCount {
		t.Fatalf("expected %d template compensations, got %d", templateCount, len(comps))
	}

	var totalCount int
	if err := pool.QueryRow(ctx,
		`SELECT COUNT(*)::int FROM contract_compensations WHERE contract_id = $1`,
		contractID).Scan(&totalCount); err != nil {
		t.Fatal(err)
	}
	if totalCount <= templateCount {
		t.Fatalf("expected participation copies in DB, total=%d templates=%d", totalCount, templateCount)
	}
}

func TestListCompensations_afterInviteBuyer(t *testing.T) {
	pool := connectContractsTestDB(t)
	ctx := context.Background()
	svc := NewService(pool)

	var publisherID, contractID int64
	var templateCount int
	err := pool.QueryRow(ctx,
		`SELECT c.publisher_id, c.id,
		        (SELECT COUNT(*)::int FROM contract_compensations
		         WHERE contract_id = c.id AND participation_id IS NULL)
		 FROM contracts c
		 WHERE c.status = 'active' AND c.buyer_id IS NULL AND c.deleted_at IS NULL
		   AND (SELECT COUNT(*) FROM contract_compensations
		        WHERE contract_id = c.id AND participation_id IS NULL) >= 2
		 LIMIT 1`).Scan(&publisherID, &contractID, &templateCount)
	if err != nil {
		t.Skip("no active open offer with at least two template compensations")
	}

	var buyerID int64
	err = pool.QueryRow(ctx,
		`SELECT p.buyer_id FROM partnerships p
		 WHERE p.publisher_id = $1 AND p.status = 'active'
		   AND NOT EXISTS (
		     SELECT 1 FROM contract_participations cp
		     WHERE cp.contract_id = $2 AND cp.buyer_id = p.buyer_id
		   )
		 LIMIT 1`, publisherID, contractID).Scan(&buyerID)
	if err != nil {
		t.Skip("no partnered buyer available to invite on this contract")
	}

	part, err := svc.AddParticipation(ctx, publisherID, contractID, AddParticipationParams{BuyerID: buyerID})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(),
			`DELETE FROM contract_participations WHERE id = $1`, part.ID)
	})

	comps, err := svc.ListCompensations(ctx, publisherID, contractID)
	if err != nil {
		t.Fatal(err)
	}
	if len(comps) != templateCount {
		t.Fatalf("expected %d template compensations after invite, got %d", templateCount, len(comps))
	}

	var totalCount int
	if err := pool.QueryRow(ctx,
		`SELECT COUNT(*)::int FROM contract_compensations WHERE contract_id = $1`,
		contractID).Scan(&totalCount); err != nil {
		t.Fatal(err)
	}
	expectedTotal := templateCount * 2
	if totalCount != expectedTotal {
		t.Fatalf("expected %d total compensation rows in DB after invite, got %d", expectedTotal, totalCount)
	}
}
