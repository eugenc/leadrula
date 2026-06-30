package leads

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/echayko/leadrula/backend/internal/accounts"
	"github.com/echayko/leadrula/backend/internal/auth"
	"github.com/echayko/leadrula/backend/internal/permissions"
	"github.com/echayko/leadrula/backend/internal/billing"
	"github.com/echayko/leadrula/backend/internal/config"
	"github.com/echayko/leadrula/backend/internal/contracts"
	"github.com/echayko/leadrula/backend/internal/database"
	"github.com/echayko/leadrula/backend/internal/notifications"
	"github.com/echayko/leadrula/backend/internal/pipelines"
	"github.com/echayko/leadrula/backend/pkg/httpx"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func connectLeadsTestDB(t *testing.T) *pgxpool.Pool {
	t.Helper()
	cfg := config.Load()
	ctx := context.Background()
	pool, err := database.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(func() { pool.Close() })
	return pool
}

func TestAssertCanEdit(t *testing.T) {
	assigned := int64(42)
	lead := &Lead{AssignedUserID: &assigned}

	if err := assertCanEdit(&auth.Principal{FullAccess: true}, lead); err != nil {
		t.Fatalf("all scope: %v", err)
	}
	if err := assertCanEdit(&auth.Principal{
		UserID: 42,
		Perms:  permissions.Effective{LeadScope: permissions.LeadScopeAssigned},
	}, lead); err != nil {
		t.Fatalf("assigned user: %v", err)
	}
	if err := assertCanEdit(&auth.Principal{
		UserID: 99,
		Perms:  permissions.Effective{LeadScope: permissions.LeadScopeAssigned},
	}, lead); err == nil {
		t.Fatal("expected forbidden for non-assigned user")
	}
	if err := assertCanEdit(&auth.Principal{
		Perms: permissions.Effective{LeadScope: permissions.LeadScopeFollowed},
	}, lead); err == nil {
		t.Fatal("expected forbidden for followed scope")
	}
	other := int64(99)
	followedOnly := &Lead{AssignedUserID: &other}
	if err := assertCanEdit(&auth.Principal{
		UserID: 42,
		Perms:  permissions.Effective{LeadScope: permissions.LeadScopeAssignedAndFollowed},
	}, followedOnly); err == nil {
		t.Fatal("expected forbidden for followed-only lead under union scope")
	}
	if err := assertCanEdit(&auth.Principal{
		UserID: 42,
		Perms:  permissions.Effective{LeadScope: permissions.LeadScopeAssignedAndFollowed},
	}, lead); err != nil {
		t.Fatalf("assigned lead under union scope: %v", err)
	}
}

func TestUpdateStage_setsPipelineID(t *testing.T) {
	pool := connectLeadsTestDB(t)
	ctx := context.Background()
	repo := NewRepository(pool)

	var leadID, stageID, pipelineID int64
	err := pool.QueryRow(ctx,
		`SELECT l.id, l.stage_id, ps.pipeline_id
		 FROM leads l
		 JOIN pipeline_stages ps ON ps.id = l.stage_id
		 WHERE l.deleted_at IS NULL AND l.stage_id IS NOT NULL
		 LIMIT 1`).Scan(&leadID, &stageID, &pipelineID)
	if err != nil {
		t.Skip("no lead with stage in database")
	}

	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)

	var otherStageID int64
	err = pool.QueryRow(ctx,
		`SELECT id FROM pipeline_stages WHERE pipeline_id = $1 AND id != $2 ORDER BY position, id LIMIT 1`,
		pipelineID, stageID).Scan(&otherStageID)
	if err != nil {
		t.Skip("pipeline has only one stage")
	}

	if err := repo.UpdateStage(ctx, tx, leadID, pipelineID, otherStageID); err != nil {
		t.Fatal(err)
	}

	var gotPipelineID, gotStageID int64
	err = tx.QueryRow(ctx,
		`SELECT pipeline_id, stage_id FROM leads WHERE id = $1`, leadID).Scan(&gotPipelineID, &gotStageID)
	if err != nil {
		t.Fatal(err)
	}
	if gotPipelineID != pipelineID || gotStageID != otherStageID {
		t.Fatalf("got pipeline=%d stage=%d want pipeline=%d stage=%d",
			gotPipelineID, gotStageID, pipelineID, otherStageID)
	}
}

func TestClearFromPipeline_nullsPipelineAndStage(t *testing.T) {
	pool := connectLeadsTestDB(t)
	ctx := context.Background()
	repo := NewRepository(pool)

	var leadID int64
	err := pool.QueryRow(ctx,
		`SELECT id FROM leads WHERE deleted_at IS NULL AND stage_id IS NOT NULL LIMIT 1`).Scan(&leadID)
	if err != nil {
		t.Skip("no lead with stage in database")
	}

	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)

	if err := repo.ClearFromPipeline(ctx, tx, leadID); err != nil {
		t.Fatal(err)
	}

	var pipelineID, stageID *int64
	err = tx.QueryRow(ctx,
		`SELECT pipeline_id, stage_id FROM leads WHERE id = $1`, leadID).Scan(&pipelineID, &stageID)
	if err != nil {
		t.Fatal(err)
	}
	if pipelineID != nil || stageID != nil {
		t.Fatalf("expected null pipeline and stage, got pipeline=%v stage=%v", pipelineID, stageID)
	}
}

func TestClearFromPipeline_serviceForbiddenForNonAssignee(t *testing.T) {
	pool := connectLeadsTestDB(t)
	ctx := context.Background()
	repo := NewRepository(pool)
	svc := &Service{repo: repo}

	var leadID, ownerAccountID, assigneeID int64
	err := pool.QueryRow(ctx,
		`SELECT l.id, l.owner_account_id, l.assigned_user_id
		 FROM leads l
		 WHERE l.deleted_at IS NULL AND l.assigned_user_id IS NOT NULL
		   AND l.stage_id IS NOT NULL
		 LIMIT 1`).Scan(&leadID, &ownerAccountID, &assigneeID)
	if err != nil {
		t.Skip("no assigned lead with stage in database")
	}

	p := &auth.Principal{
		AccountID: ownerAccountID,
		UserID:    assigneeID + 9999,
		Role:      "user",
	}
	_, err = svc.ClearFromPipeline(ctx, p, leadID)
	if err == nil {
		t.Fatal("expected forbidden for non-assigned user")
	}
	var appErr *httpx.AppError
	if !errors.As(err, &appErr) || appErr.Code != httpx.CodeForbidden {
		t.Fatalf("expected forbidden, got %v", err)
	}
}

func newStageTestService(t *testing.T, pool *pgxpool.Pool) *Service {
	t.Helper()
	acc := accounts.NewRepository(pool)
	return NewService(
		NewRepository(pool),
		notifications.NewService(pool, acc, nil, "http://localhost"),
		acc,
		pipelines.NewService(pool, nil),
		nil,
	)
}

type disqMoveFixture struct {
	leadID         int64
	ownerAccountID int64
	publisherID    int64
	contractID     int64
	fromStageID    int64
	disqStageID    int64
	reasonID       int64
	returnStageID  int64
	sourcePipeline int64
}

func loadDisqMoveFixture(ctx context.Context, pool *pgxpool.Pool) (*disqMoveFixture, error) {
	f := &disqMoveFixture{}
	err := pool.QueryRow(ctx,
		`SELECT l.id, l.owner_account_id, c.publisher_id, l.contract_id, l.stage_id,
		        disq.id, dr.id, c.return_stage_id, c.source_pipeline_id
		 FROM leads l
		 JOIN contracts c ON c.id = l.contract_id
		 JOIN pipeline_stages cur ON cur.id = l.stage_id
		 JOIN pipeline_stages disq ON disq.pipeline_id = l.pipeline_id AND disq.stage_type = 'disqualification'
		 JOIN disqualification_reasons dr ON dr.stage_id = disq.id AND dr.is_active
		 WHERE l.deleted_at IS NULL AND l.contract_id IS NOT NULL
		   AND cur.stage_type <> 'disqualification'
		   AND c.return_stage_id IS NOT NULL AND c.source_pipeline_id IS NOT NULL
		 LIMIT 1`).Scan(
		&f.leadID, &f.ownerAccountID, &f.publisherID, &f.contractID, &f.fromStageID,
		&f.disqStageID, &f.reasonID, &f.returnStageID, &f.sourcePipeline,
	)
	return f, err
}

func TestChangeStage_disqualificationTriggersReturn(t *testing.T) {
	pool := connectLeadsTestDB(t)
	ctx := context.Background()
	svc := newStageTestService(t, pool)

	f, err := loadDisqMoveFixture(ctx, pool)
	if err != nil {
		t.Skip("no buyer contract lead with disqualification stage in database")
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
	})

	p := &auth.Principal{AccountID: f.ownerAccountID, AccountType: "buyer", Role: "admin", UserID: 1}
	reasonID := f.reasonID
	updated, _, err := svc.ChangeStage(ctx, p, f.leadID, f.disqStageID, nil, &reasonID)
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

func TestChangeStage_skipsPublisherSyncWhenReturnRuleMatches(t *testing.T) {
	pool := connectLeadsTestDB(t)
	ctx := context.Background()
	svc := newStageTestService(t, pool)

	f, err := loadDisqMoveFixture(ctx, pool)
	if err != nil {
		t.Skip("no buyer contract lead with disqualification stage in database")
	}

	var wrongPubStage int64
	err = pool.QueryRow(ctx,
		`SELECT ps.id FROM pipeline_stages ps
		 WHERE ps.pipeline_id = (SELECT pipeline_id FROM leads WHERE id = $1)
		   AND ps.id <> $2
		 ORDER BY ps.position, ps.id LIMIT 1`,
		f.leadID, f.disqStageID).Scan(&wrongPubStage)
	if err != nil {
		t.Skip("buyer pipeline has no other stage for invalid stage map test")
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

	var mapID int64
	err = pool.QueryRow(ctx,
		`INSERT INTO contract_stage_maps(contract_id, participation_id, buyer_stage_id, publisher_stage_id)
		 VALUES ($1, NULL, $2, $3)
		 ON CONFLICT (contract_id, participation_id, buyer_stage_id)
		 DO UPDATE SET publisher_stage_id = EXCLUDED.publisher_stage_id
		 RETURNING id`,
		f.contractID, f.disqStageID, wrongPubStage).Scan(&mapID)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, `DELETE FROM contract_stage_maps WHERE id = $1`, mapID)
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

	p := &auth.Principal{AccountID: f.ownerAccountID, AccountType: "buyer", Role: "admin", UserID: 1}
	reasonID := f.reasonID
	updated, _, err := svc.ChangeStage(ctx, p, f.leadID, f.disqStageID, nil, &reasonID)
	if err != nil {
		t.Fatalf("ChangeStage with invalid stage map should skip sync and return: %v", err)
	}
	if updated.OwnerAccountID != f.publisherID {
		t.Fatalf("owner_account_id = %d, want publisher %d", updated.OwnerAccountID, f.publisherID)
	}
}

func TestChangeStage_invalidReturnDestination(t *testing.T) {
	pool := connectLeadsTestDB(t)
	ctx := context.Background()
	svc := newStageTestService(t, pool)

	f, err := loadDisqMoveFixture(ctx, pool)
	if err != nil {
		t.Skip("no buyer contract lead with disqualification stage in database")
	}

	var wrongReturnStage int64
	err = pool.QueryRow(ctx,
		`SELECT ps.id FROM pipeline_stages ps
		 WHERE ps.pipeline_id = (SELECT pipeline_id FROM leads WHERE id = $1)
		   AND ps.id <> $2
		 ORDER BY ps.position, ps.id LIMIT 1`,
		f.leadID, f.disqStageID).Scan(&wrongReturnStage)
	if err != nil {
		t.Skip("buyer pipeline has no other stage for invalid return test")
	}

	var ruleID int64
	err = pool.QueryRow(ctx,
		`INSERT INTO contract_return_rules(contract_id, buyer_stage_id, return_stage_id)
		 VALUES ($1,$2,$3)
		 ON CONFLICT (contract_id, buyer_stage_id) WHERE participation_id IS NULL
		 DO UPDATE SET return_stage_id = EXCLUDED.return_stage_id
		 RETURNING id`,
		f.contractID, f.disqStageID, wrongReturnStage).Scan(&ruleID)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, `DELETE FROM contract_return_rules WHERE id = $1`, ruleID)
	})

	p := &auth.Principal{AccountID: f.ownerAccountID, AccountType: "buyer", Role: "admin", UserID: 1}
	reasonID := f.reasonID
	_, _, err = svc.ChangeStage(ctx, p, f.leadID, f.disqStageID, nil, &reasonID)
	if err == nil {
		t.Fatal("expected error for misconfigured return destination")
	}
	var appErr *httpx.AppError
	if !errors.As(err, &appErr) || appErr.Code != httpx.CodeBusinessRule {
		t.Fatalf("expected business_rule, got %v", err)
	}
	if appErr.Message != "return destination is misconfigured for this stage" {
		t.Fatalf("message = %q", appErr.Message)
	}
}

func TestValidateReturnDestination(t *testing.T) {
	pool := connectLeadsTestDB(t)
	ctx := context.Background()

	var pipelineID, stageID int64
	err := pool.QueryRow(ctx,
		`SELECT pipeline_id, id FROM pipeline_stages ORDER BY pipeline_id, position, id LIMIT 1`).Scan(&pipelineID, &stageID)
	if err != nil {
		t.Skip("no pipeline stages in database")
	}

	if err := contracts.ValidateReturnDestination(ctx, pool, pipelineID, stageID); err != nil {
		t.Fatalf("valid stage: %v", err)
	}

	var otherStage int64
	err = pool.QueryRow(ctx,
		`SELECT id FROM pipeline_stages WHERE pipeline_id <> $1 LIMIT 1`, pipelineID).Scan(&otherStage)
	if errors.Is(err, pgx.ErrNoRows) {
		t.Skip("need stage from another pipeline")
	}
	if err != nil {
		t.Fatal(err)
	}

	err = contracts.ValidateReturnDestination(ctx, pool, pipelineID, otherStage)
	if err == nil {
		t.Fatal("expected error for stage in wrong pipeline")
	}
	var appErr *httpx.AppError
	if !errors.As(err, &appErr) || appErr.Code != httpx.CodeBusinessRule {
		t.Fatalf("expected business_rule, got %v", err)
	}
}

func TestChangeStage_returnRefundsBuyerAndReversesEarning(t *testing.T) {
	pool := connectLeadsTestDB(t)
	ctx := context.Background()
	svc := newStageTestService(t, pool)

	f, err := loadDisqMoveFixture(ctx, pool)
	if err != nil {
		t.Skip("no buyer contract lead with disqualification stage in database")
	}

	var compensationID int64
	err = pool.QueryRow(ctx,
		`SELECT id FROM contract_compensations WHERE contract_id = $1 LIMIT 1`,
		f.contractID).Scan(&compensationID)
	if err != nil {
		t.Skip("contract has no compensation row")
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

	const debitAmt = 25.50
	setupTx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := billing.EnsureBalance(ctx, setupTx, f.ownerAccountID); err != nil {
		t.Fatal(err)
	}
	if err := billing.Debit(ctx, setupTx, f.ownerAccountID, debitAmt, f.leadID, f.contractID, "test distribute"); err != nil {
		t.Fatal(err)
	}
	if err := contracts.RecordEarningDistribute(ctx, setupTx, compensationID, f.leadID, debitAmt, nil); err != nil {
		t.Fatal(err)
	}
	if err := setupTx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx,
			`DELETE FROM compensation_earnings WHERE lead_id = $1 AND compensation_id = $2`,
			f.leadID, compensationID)
		_, _ = pool.Exec(ctx,
			`DELETE FROM transactions WHERE lead_id = $1 AND contract_id = $2 AND description IN ('test distribute', 'lead returned')`,
			f.leadID, f.contractID)
	})

	var balBefore float64
	if err := pool.QueryRow(ctx,
		`SELECT balance::float8 FROM buyer_balances WHERE buyer_id = $1`, f.ownerAccountID).Scan(&balBefore); err != nil {
		t.Fatal(err)
	}

	_, _ = pool.Exec(ctx,
		`DELETE FROM transactions WHERE lead_id = $1 AND contract_id = $2 AND description = 'lead returned'`,
		f.leadID, f.contractID)

	var origOwner, origStage int64
	var origContractID *int64
	var origStatus string
	var origPipelineID *int64
	var origDisqReason *int64
	if err := pool.QueryRow(ctx,
		`SELECT owner_account_id, stage_id, contract_id, status::text, pipeline_id, disqualification_reason_id FROM leads WHERE id = $1`,
		f.leadID).Scan(&origOwner, &origStage, &origContractID, &origStatus, &origPipelineID, &origDisqReason); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx,
			`UPDATE leads SET owner_account_id=$2, stage_id=$3, contract_id=$4, status=$5::lead_status,
			        pipeline_id=$6, disqualification_reason_id=$7 WHERE id=$1`,
			f.leadID, origOwner, origStage, origContractID, origStatus, origPipelineID, origDisqReason)
	})

	p := &auth.Principal{AccountID: f.ownerAccountID, AccountType: "buyer", Role: "admin", UserID: 1}
	reasonID := f.reasonID
	updated, _, err := svc.ChangeStage(ctx, p, f.leadID, f.disqStageID, nil, &reasonID)
	if err != nil {
		t.Fatalf("ChangeStage: %v", err)
	}
	if updated.OwnerAccountID != f.publisherID {
		t.Fatalf("owner_account_id = %d, want publisher %d", updated.OwnerAccountID, f.publisherID)
	}
	if updated.DisqReasonID != nil {
		t.Fatalf("disqualification_reason_id = %v, want nil after return", updated.DisqReasonID)
	}

	var balAfter float64
	if err := pool.QueryRow(ctx,
		`SELECT balance::float8 FROM buyer_balances WHERE buyer_id = $1`, f.ownerAccountID).Scan(&balAfter); err != nil {
		t.Fatal(err)
	}
	if balAfter != balBefore+debitAmt {
		t.Fatalf("buyer balance = %v, want %v (refunded distribute debit)", balAfter, balBefore+debitAmt)
	}

	var returnAmt float64
	err = pool.QueryRow(ctx,
		`SELECT amount::float8 FROM compensation_earnings
		 WHERE lead_id = $1 AND compensation_id = $2 AND kind = 'return'`,
		f.leadID, compensationID).Scan(&returnAmt)
	if err != nil {
		t.Fatalf("return earning row: %v", err)
	}
	if returnAmt != -debitAmt {
		t.Fatalf("return earning = %v, want %v", returnAmt, -debitAmt)
	}
}

func TestChangeStage_actionToStandardClearsActionAt(t *testing.T) {
	pool := connectLeadsTestDB(t)
	ctx := context.Background()
	svc := newStageTestService(t, pool)

	var leadID, ownerAccountID, actionStageID, standardStageID int64
	err := pool.QueryRow(ctx,
		`SELECT l.id, l.owner_account_id, ps_action.id, ps_standard.id
		 FROM leads l
		 JOIN pipeline_stages ps_action ON ps_action.pipeline_id = l.pipeline_id AND ps_action.stage_type = 'action'
		 JOIN pipeline_stages ps_standard ON ps_standard.pipeline_id = l.pipeline_id AND ps_standard.stage_type = 'standard'
		 WHERE l.deleted_at IS NULL AND l.contract_id IS NULL
		 LIMIT 1`).Scan(&leadID, &ownerAccountID, &actionStageID, &standardStageID)
	if err != nil {
		t.Skip("no lead with action and standard stages in same pipeline")
	}

	actionAt := time.Now().UTC().Add(24 * time.Hour)
	_, err = pool.Exec(ctx, `UPDATE leads SET stage_id = $2, action_at = $3 WHERE id = $1`, leadID, actionStageID, actionAt)
	if err != nil {
		t.Fatal(err)
	}

	p := &auth.Principal{AccountID: ownerAccountID, Role: "admin", UserID: 1}
	updated, _, err := svc.ChangeStage(ctx, p, leadID, standardStageID, nil, nil)
	if err != nil {
		t.Fatalf("ChangeStage: %v", err)
	}
	if updated.ActionAt != nil {
		t.Fatalf("action_at = %v, want nil after move to standard stage", updated.ActionAt)
	}
}
