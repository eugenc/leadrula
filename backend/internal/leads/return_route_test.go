package leads

import (
	"context"
	"fmt"
	"testing"

	"github.com/echayko/leadrula/backend/internal/accounts"
	"github.com/echayko/leadrula/backend/internal/auth"
	"github.com/echayko/leadrula/backend/internal/billing"
	"github.com/echayko/leadrula/backend/internal/notifications"
	"github.com/echayko/leadrula/backend/internal/routing"
)

func TestChangeStage_buyerRouteToReturnStartTriggersReturn(t *testing.T) {
	pool := connectLeadsTestDB(t)
	ctx := context.Background()
	svc := newStageTestService(t, pool)

	f, err := loadDisqMoveFixture(ctx, pool)
	if err != nil {
		t.Skip("no buyer contract lead with disqualification stage in database")
	}

	var originStageID, buyerPipelineID int64
	err = pool.QueryRow(ctx,
		`SELECT ps.id, ps.pipeline_id FROM pipeline_stages ps
		 WHERE ps.pipeline_id = (SELECT pipeline_id FROM leads WHERE id = $1)
		   AND ps.id <> $2 AND ps.stage_type NOT IN ('disqualification', 'action')
		 ORDER BY ps.position, ps.id LIMIT 1`,
		f.leadID, f.disqStageID).Scan(&originStageID, &buyerPipelineID)
	if err != nil {
		t.Skip("buyer pipeline has no plain stage for route origin")
	}

	var startStageID int64
	err = pool.QueryRow(ctx,
		`SELECT ps.id FROM pipeline_stages ps
		 WHERE ps.pipeline_id = $1 AND ps.id NOT IN ($2, $3)
		   AND ps.stage_type NOT IN ('disqualification', 'action')
		 ORDER BY ps.position, ps.id LIMIT 1`,
		buyerPipelineID, originStageID, f.disqStageID).Scan(&startStageID)
	if err != nil {
		t.Skip("buyer pipeline needs a third stage for route trigger test")
	}

	var ruleID int64
	err = pool.QueryRow(ctx,
		`INSERT INTO contract_return_rules(contract_id, buyer_stage_id, return_stage_id)
		 VALUES ($1,$2,$3)
		 ON CONFLICT (contract_id, buyer_stage_id) WHERE participation_id IS NULL
		 DO UPDATE SET return_stage_id = EXCLUDED.return_stage_id
		 RETURNING id`,
		f.contractID, f.disqStageID, f.returnStageID).Scan(&ruleID)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, `DELETE FROM contract_return_rules WHERE id = $1`, ruleID)
	})

	branchesJSON := `[{"name":"Route 1","position":0,"condition_logic":"and","conditions":[],"destination":"pipeline","delivery":"leads_pipeline","target_pipeline_id":` +
		fmt.Sprintf("%d", buyerPipelineID) + `,"target_stage_id":` + fmt.Sprintf("%d", f.disqStageID) + `}]`
	var routeID int64
	err = pool.QueryRow(ctx,
		`INSERT INTO routes(buyer_id, name, origin, origin_pipeline_id, origin_stage_id, destination, delivery, target_pipeline_id, target_stage_id, branches)
		 VALUES ($1, 'test return route', 'pipeline', $2, $3, 'pipeline', 'leads_pipeline', $2, $4, $5::jsonb)
		 RETURNING id`,
		f.ownerAccountID, buyerPipelineID, originStageID, f.disqStageID, branchesJSON).Scan(&routeID)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, `DELETE FROM routes WHERE id = $1`, routeID)
	})

	var origOwner, origStage int64
	var origContractID *int64
	var origStatus string
	var origPipelineID *int64
	if err := pool.QueryRow(ctx,
		`SELECT owner_account_id, stage_id, contract_id, status::text, pipeline_id FROM leads WHERE id = $1`,
		f.leadID).Scan(&origOwner, &origStage, &origContractID, &origStatus, &origPipelineID); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx,
			`UPDATE leads SET owner_account_id=$2, stage_id=$3, contract_id=$4, status=$5::lead_status, pipeline_id=$6 WHERE id=$1`,
			f.leadID, origOwner, origStage, origContractID, origStatus, origPipelineID)
	})

	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(ctx,
		`UPDATE leads SET stage_id=$2, pipeline_id=$3, status='distributed'::lead_status WHERE id=$1`,
		f.leadID, startStageID, buyerPipelineID); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}

	p := &auth.Principal{AccountID: f.ownerAccountID, AccountType: "buyer", Role: "admin", UserID: 1}
	updated, _, err := svc.ChangeStage(ctx, p, f.leadID, originStageID, nil, nil)
	if err != nil {
		t.Fatalf("ChangeStage: %v", err)
	}
	if updated.OwnerAccountID != f.publisherID {
		t.Fatalf("owner_account_id = %d, want publisher %d", updated.OwnerAccountID, f.publisherID)
	}
	if updated.Status != "returned" {
		t.Fatalf("status = %q, want returned", updated.Status)
	}
	if updated.StageID == nil || *updated.StageID != f.returnStageID {
		t.Fatalf("stage_id = %v, want return stage %d", updated.StageID, f.returnStageID)
	}
}

func TestApplyPipelineRoute_preservesContractID(t *testing.T) {
	pool := connectLeadsTestDB(t)
	ctx := context.Background()
	repo := NewRepository(pool)

	f, err := loadDisqMoveFixture(ctx, pool)
	if err != nil {
		t.Skip("no buyer contract lead with disqualification stage in database")
	}

	var originStageID, buyerPipelineID int64
	err = pool.QueryRow(ctx,
		`SELECT ps.id, ps.pipeline_id FROM pipeline_stages ps
		 WHERE ps.pipeline_id = (SELECT pipeline_id FROM leads WHERE id = $1)
		   AND ps.id <> $2
		 ORDER BY ps.position, ps.id LIMIT 1`,
		f.leadID, f.disqStageID).Scan(&originStageID, &buyerPipelineID)
	if err != nil {
		t.Skip("buyer pipeline has no other stage")
	}

	var origOwner, origStage int64
	var origContractID *int64
	var origStatus string
	var origPipelineID *int64
	if err := pool.QueryRow(ctx,
		`SELECT owner_account_id, stage_id, contract_id, status::text, pipeline_id FROM leads WHERE id = $1`,
		f.leadID).Scan(&origOwner, &origStage, &origContractID, &origStatus, &origPipelineID); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx,
			`UPDATE leads SET owner_account_id=$2, stage_id=$3, contract_id=$4, status=$5::lead_status, pipeline_id=$6 WHERE id=$1`,
			f.leadID, origOwner, origStage, origContractID, origStatus, origPipelineID)
	})

	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(ctx,
		`UPDATE leads SET stage_id=$2, pipeline_id=$3, contract_id=$4, status='distributed'::lead_status WHERE id=$1`,
		f.leadID, originStageID, buyerPipelineID, f.contractID); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}

	targetPipelineID := buyerPipelineID
	targetStageID := originStageID
	route := &routing.Route{
		Delivery:         "leads_pipeline",
		TargetPipelineID: &targetPipelineID,
		TargetStageID:    &targetStageID,
	}

	tx, err = pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)

	deps := RouteApplyDeps{Repo: repo}
	if _, err := applyPipelineRoute(ctx, tx, deps, route, f.ownerAccountID, f.leadID); err != nil {
		t.Fatalf("applyPipelineRoute: %v", err)
	}

	var contractID *int64
	var stageID int64
	if err := tx.QueryRow(ctx,
		`SELECT contract_id, stage_id FROM leads WHERE id = $1`, f.leadID).
		Scan(&contractID, &stageID); err != nil {
		t.Fatal(err)
	}
	if contractID == nil || *contractID != f.contractID {
		t.Fatalf("contract_id = %v, want %d", contractID, f.contractID)
	}
	if stageID != originStageID {
		t.Fatalf("stage_id = %d, want %d", stageID, originStageID)
	}
}

func TestApplyPipelineRoute_clearsStalePublisherTracking(t *testing.T) {
	pool := connectLeadsTestDB(t)
	ctx := context.Background()
	repo := NewRepository(pool)

	f, err := loadDisqMoveFixture(ctx, pool)
	if err != nil {
		t.Skip("no buyer contract lead with disqualification stage in database")
	}

	var originStageID, buyerPipelineID int64
	err = pool.QueryRow(ctx,
		`SELECT ps.id, ps.pipeline_id FROM pipeline_stages ps
		 WHERE ps.pipeline_id = (SELECT pipeline_id FROM leads WHERE id = $1)
		   AND ps.id <> $2
		 ORDER BY ps.position, ps.id LIMIT 1`,
		f.leadID, f.disqStageID).Scan(&originStageID, &buyerPipelineID)
	if err != nil {
		t.Skip("buyer pipeline has no other stage")
	}

	var pubStageID int64
	err = pool.QueryRow(ctx,
		`SELECT id FROM pipeline_stages WHERE pipeline_id = $1 ORDER BY position, id LIMIT 1`,
		f.sourcePipeline).Scan(&pubStageID)
	if err != nil {
		t.Skip("publisher pipeline has no stages")
	}

	var origOwner, origStage int64
	var origContractID *int64
	var origStatus string
	var origPipelineID *int64
	var origPubPipelineID, origPubStageID *int64
	if err := pool.QueryRow(ctx,
		`SELECT owner_account_id, stage_id, contract_id, status::text, pipeline_id,
		        publisher_pipeline_id, publisher_stage_id
		 FROM leads WHERE id = $1`,
		f.leadID).Scan(&origOwner, &origStage, &origContractID, &origStatus, &origPipelineID, &origPubPipelineID, &origPubStageID); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx,
			`UPDATE leads SET owner_account_id=$2, stage_id=$3, contract_id=$4, status=$5::lead_status,
			        pipeline_id=$6, publisher_pipeline_id=$7, publisher_stage_id=$8 WHERE id=$1`,
			f.leadID, origOwner, origStage, origContractID, origStatus, origPipelineID, origPubPipelineID, origPubStageID)
	})

	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(ctx,
		`UPDATE leads SET owner_account_id=$2, stage_id=$3, pipeline_id=$4, contract_id=$5,
		        status='distributed'::lead_status, publisher_pipeline_id=$6, publisher_stage_id=$7
		 WHERE id=$1`,
		f.leadID, f.ownerAccountID, originStageID, buyerPipelineID, f.contractID,
		f.sourcePipeline, pubStageID); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}

	targetPipelineID := buyerPipelineID
	targetStageID := f.disqStageID
	route := &routing.Route{
		Delivery:         "leads_pipeline",
		TargetPipelineID: &targetPipelineID,
		TargetStageID:    &targetStageID,
	}

	tx, err = pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)

	deps := RouteApplyDeps{Repo: repo}
	if _, err := applyPipelineRoute(ctx, tx, deps, route, f.ownerAccountID, f.leadID); err != nil {
		t.Fatalf("applyPipelineRoute: %v", err)
	}

	var pubPipelineID, pubStageIDOut *int64
	var stageID int64
	if err := tx.QueryRow(ctx,
		`SELECT stage_id, publisher_pipeline_id, publisher_stage_id FROM leads WHERE id = $1`, f.leadID).
		Scan(&stageID, &pubPipelineID, &pubStageIDOut); err != nil {
		t.Fatal(err)
	}
	if stageID != f.disqStageID {
		t.Fatalf("stage_id = %d, want %d", stageID, f.disqStageID)
	}
	if pubPipelineID != nil || pubStageIDOut != nil {
		t.Fatalf("publisher tracking = %v,%v want cleared", pubPipelineID, pubStageIDOut)
	}
}

func TestApplyPipelineRoute_resolvesContractFromDebit(t *testing.T) {
	pool := connectLeadsTestDB(t)
	ctx := context.Background()
	repo := NewRepository(pool)

	f, err := loadDisqMoveFixture(ctx, pool)
	if err != nil {
		t.Skip("no buyer contract lead with disqualification stage in database")
	}

	var originStageID, buyerPipelineID int64
	err = pool.QueryRow(ctx,
		`SELECT ps.id, ps.pipeline_id FROM pipeline_stages ps
		 WHERE ps.pipeline_id = (SELECT pipeline_id FROM leads WHERE id = $1)
		   AND ps.id <> $2
		 ORDER BY ps.position, ps.id LIMIT 1`,
		f.leadID, f.disqStageID).Scan(&originStageID, &buyerPipelineID)
	if err != nil {
		t.Skip("buyer pipeline has no other stage")
	}

	var origOwner, origStage int64
	var origContractID *int64
	var origStatus string
	var origPipelineID *int64
	if err := pool.QueryRow(ctx,
		`SELECT owner_account_id, stage_id, contract_id, status::text, pipeline_id FROM leads WHERE id = $1`,
		f.leadID).Scan(&origOwner, &origStage, &origContractID, &origStatus, &origPipelineID); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx,
			`UPDATE leads SET owner_account_id=$2, stage_id=$3, contract_id=$4, status=$5::lead_status, pipeline_id=$6 WHERE id=$1`,
			f.leadID, origOwner, origStage, origContractID, origStatus, origPipelineID)
		_, _ = pool.Exec(ctx,
			`DELETE FROM transactions WHERE lead_id = $1 AND contract_id = $2 AND description = 'test debit fallback route'`,
			f.leadID, f.contractID)
	})

	setupTx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := billing.EnsureBalance(ctx, setupTx, f.ownerAccountID); err != nil {
		t.Fatal(err)
	}
	if err := billing.Debit(ctx, setupTx, f.ownerAccountID, 10, f.leadID, f.contractID, "test debit fallback route"); err != nil {
		t.Fatal(err)
	}
	if _, err := setupTx.Exec(ctx,
		`UPDATE leads SET stage_id=$2, pipeline_id=$3, contract_id=NULL, status='review'::lead_status WHERE id=$1`,
		f.leadID, originStageID, buyerPipelineID); err != nil {
		t.Fatal(err)
	}
	if err := setupTx.Commit(ctx); err != nil {
		t.Fatal(err)
	}

	targetPipelineID := buyerPipelineID
	targetStageID := originStageID
	route := &routing.Route{
		Delivery:         "leads_pipeline",
		TargetPipelineID: &targetPipelineID,
		TargetStageID:    &targetStageID,
	}

	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)

	deps := RouteApplyDeps{Repo: repo}
	if _, err := applyPipelineRoute(ctx, tx, deps, route, f.ownerAccountID, f.leadID); err != nil {
		t.Fatalf("applyPipelineRoute: %v", err)
	}

	var contractID *int64
	var stageID int64
	var status string
	if err := tx.QueryRow(ctx,
		`SELECT contract_id, stage_id, status::text FROM leads WHERE id = $1`, f.leadID).
		Scan(&contractID, &stageID, &status); err != nil {
		t.Fatal(err)
	}
	if contractID == nil || *contractID != f.contractID {
		t.Fatalf("contract_id = %v, want %d", contractID, f.contractID)
	}
	if stageID != originStageID {
		t.Fatalf("stage_id = %d, want %d", stageID, originStageID)
	}
	if status != "distributed" {
		t.Fatalf("status = %q, want distributed", status)
	}
}

func TestTryReturnLead_resolvesContractFromDebit(t *testing.T) {
	pool := connectLeadsTestDB(t)
	ctx := context.Background()
	repo := NewRepository(pool)
	acc := accounts.NewRepository(pool)
	notif := notifications.NewService(pool, acc, nil, "http://localhost")

	f, err := loadDisqMoveFixture(ctx, pool)
	if err != nil {
		t.Skip("no buyer contract lead with disqualification stage in database")
	}

	var buyerPipelineID int64
	err = pool.QueryRow(ctx,
		`SELECT pipeline_id FROM leads WHERE id = $1`, f.leadID).Scan(&buyerPipelineID)
	if err != nil {
		t.Fatal(err)
	}

	var ruleID int64
	err = pool.QueryRow(ctx,
		`INSERT INTO contract_return_rules(contract_id, buyer_stage_id, return_stage_id)
		 VALUES ($1,$2,$3)
		 ON CONFLICT (contract_id, buyer_stage_id) WHERE participation_id IS NULL
		 DO UPDATE SET return_stage_id = EXCLUDED.return_stage_id
		 RETURNING id`,
		f.contractID, f.disqStageID, f.returnStageID).Scan(&ruleID)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, `DELETE FROM contract_return_rules WHERE id = $1`, ruleID)
	})

	var origOwner, origStage int64
	var origContractID *int64
	var origStatus string
	var origPipelineID *int64
	if err := pool.QueryRow(ctx,
		`SELECT owner_account_id, stage_id, contract_id, status::text, pipeline_id FROM leads WHERE id = $1`,
		f.leadID).Scan(&origOwner, &origStage, &origContractID, &origStatus, &origPipelineID); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx,
			`UPDATE leads SET owner_account_id=$2, stage_id=$3, contract_id=$4, status=$5::lead_status, pipeline_id=$6 WHERE id=$1`,
			f.leadID, origOwner, origStage, origContractID, origStatus, origPipelineID)
		_, _ = pool.Exec(ctx,
			`DELETE FROM transactions WHERE lead_id = $1 AND contract_id = $2 AND description IN ('test debit fallback return', 'lead returned')`,
			f.leadID, f.contractID)
	})

	setupTx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := billing.EnsureBalance(ctx, setupTx, f.ownerAccountID); err != nil {
		t.Fatal(err)
	}
	if err := billing.Debit(ctx, setupTx, f.ownerAccountID, 10, f.leadID, f.contractID, "test debit fallback return"); err != nil {
		t.Fatal(err)
	}
	if _, err := setupTx.Exec(ctx,
		`UPDATE leads SET stage_id=$2, pipeline_id=$3, contract_id=NULL, status='review'::lead_status WHERE id=$1`,
		f.leadID, f.disqStageID, buyerPipelineID); err != nil {
		t.Fatal(err)
	}
	if err := setupTx.Commit(ctx); err != nil {
		t.Fatal(err)
	}

	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)

	out, err := TryReturnLead(ctx, tx, ReturnDeps{Repo: repo, Notif: notif}, f.leadID)
	if err != nil {
		t.Fatalf("TryReturnLead: %v", err)
	}
	if !out.Returned {
		t.Fatal("expected lead to be returned")
	}
	if out.Lead.OwnerAccountID != f.publisherID {
		t.Fatalf("owner_account_id = %d, want publisher %d", out.Lead.OwnerAccountID, f.publisherID)
	}
	if out.Lead.Status != "returned" {
		t.Fatalf("status = %q, want returned", out.Lead.Status)
	}
	if out.Lead.StageID == nil || *out.Lead.StageID != f.returnStageID {
		t.Fatalf("stage_id = %v, want return stage %d", out.Lead.StageID, f.returnStageID)
	}
}
