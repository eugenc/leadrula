package webhooks

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/echayko/leadrula/backend/internal/database"
	"github.com/echayko/leadrula/backend/internal/integrations/providers"
	"github.com/echayko/leadrula/backend/internal/leads"
	"github.com/echayko/leadrula/backend/internal/pipelines"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestTryApplyGHLInboundStageSync_direct(t *testing.T) {
	ctx := context.Background()
	pool, err := database.Connect(ctx, "postgres://crm:crm@localhost:5432/crm?sslmode=disable")
	if err != nil {
		t.Skip(err)
	}
	defer pool.Close()

	repo := leads.NewRepository(pool)
	pipesSvc := pipelines.NewService(pool, nil)
	leadSvc := leads.NewService(repo, nil, nil, pipesSvc, nil)
	svc := NewService(pool, repo, leadSvc, testEncKey(t), nil)

	var accountID int64
	if err := pool.QueryRow(ctx, `SELECT id FROM accounts ORDER BY id LIMIT 1`).Scan(&accountID); err != nil {
		t.Skip(err)
	}
	pipelineID, stage1ID, stage2ID := testAccountPipelineStages(ctx, t, pool, accountID)

	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	connID, connPublicID := testGHLConnection(ctx, t, pool, accountID, "GHL DirectSync "+suffix)
	ghlPipelineID := "ghl-pipe-direct-" + suffix
	ghlStage2ID := "ghl-stage-2-direct-" + suffix
	cfg := map[string]any{
		"location_id":                       "loc-test",
		"create_contact":                    true,
		"inbound_stage_sync_enabled":        true,
		"inbound_sync_leadrula_pipeline_id": pipelineID,
		"inbound_sync_ghl_pipeline_id":      ghlPipelineID,
		"pipeline_stage_map": []any{
			map[string]any{
				"leadrula_pipeline_id":  pipelineID,
				"leadrula_stage_id":     stage2ID,
				"ghl_pipeline_id":       ghlPipelineID,
				"ghl_pipeline_stage_id": ghlStage2ID,
			},
		},
	}
	cfgJSON, _ := json.Marshal(cfg)
	if _, err := pool.Exec(ctx, `UPDATE integration_connections SET config=$2 WHERE id=$1`, connID, cfgJSON); err != nil {
		t.Fatal(err)
	}
	cfgParsed, err := providers.ParseGHLConfig(providers.GHLConfigFromJSON(cfgJSON))
	if err != nil {
		t.Fatalf("ParseGHLConfig: %v", err)
	}
	if !providers.InboundStageSyncReady(cfgParsed) {
		t.Fatal("expected sync ready")
	}

	ids, err := svc.ProvisionGHLWebhooks(ctx, accountID, connID, connPublicID, "GHL DirectSync "+suffix)
	if err != nil {
		t.Fatal(err)
	}
	defer svc.DeleteGHLWebhooks(ctx, accountID, *ids)

	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	leadID, _, err := repo.InsertLead(ctx, tx, accountID, accountID, "", nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.PlaceInPipeline(ctx, tx, leadID, accountID, pipelineID, stage1ID, nil); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}

	if _, err := leadSvc.ChangeStageByWebhook(ctx, accountID, leadID, stage2ID, nil, nil, "test", connID); err != nil {
		t.Fatalf("ChangeStageByWebhook: %v", err)
	}
	lead, err := repo.GetByID(ctx, pool, leadID)
	if err != nil {
		t.Fatal(err)
	}
	if lead.StageID == nil || *lead.StageID != stage2ID {
		t.Fatalf("after direct ChangeStageByWebhook stage_id = %v, want %d", lead.StageID, stage2ID)
	}

	// reset to stage1 for sync test
	if err := repo.UpdateStage(ctx, pool, leadID, pipelineID, stage1ID); err != nil {
		t.Fatal(err)
	}

	var connIDOnWebhook *int64
	if err := pool.QueryRow(ctx, `SELECT integration_connection_id FROM webhooks WHERE id=$1`, ids.Inbound).Scan(&connIDOnWebhook); err != nil {
		t.Fatal(err)
	}
	if connIDOnWebhook == nil || *connIDOnWebhook != connID {
		t.Fatalf("webhook connection link = %v, want %d", connIDOnWebhook, connID)
	}

	webhook := Webhook{
		ID:                      ids.Inbound,
		AccountID:               accountID,
		IntegrationConnectionID: &connID,
	}
	tryApplyGHLInboundStageSync(ctx, svc, webhook, leadID, map[string]any{
		"pipelineId":      ghlPipelineID,
		"pipelineStageId": ghlStage2ID,
	})

	leadAfter, err := repo.GetByID(ctx, pool, leadID)
	if err != nil {
		t.Fatal(err)
	}
	if leadAfter.StageID == nil || *leadAfter.StageID != stage2ID {
		t.Fatalf("stage_id = %v, want %d", leadAfter.StageID, stage2ID)
	}
}

func TestTryApplyGHLInboundStageSync_stageNameFallback(t *testing.T) {
	ctx := context.Background()
	pool, err := database.Connect(ctx, "postgres://crm:crm@localhost:5432/crm?sslmode=disable")
	if err != nil {
		t.Skip(err)
	}
	defer pool.Close()

	repo := leads.NewRepository(pool)
	pipesSvc := pipelines.NewService(pool, nil)
	leadSvc := leads.NewService(repo, nil, nil, pipesSvc, nil)
	svc := NewService(pool, repo, leadSvc, testEncKey(t), nil)

	var accountID int64
	if err := pool.QueryRow(ctx, `SELECT id FROM accounts ORDER BY id LIMIT 1`).Scan(&accountID); err != nil {
		t.Skip(err)
	}
	pipelineID, stage1ID, stage2ID := testAccountPipelineStages(ctx, t, pool, accountID)

	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	connID, connPublicID := testGHLConnection(ctx, t, pool, accountID, "GHL NameSync "+suffix)
	ghlPipelineID := "ghl-pipe-name-" + suffix
	ghlStage2ID := "ghl-stage-name-" + suffix
	ghlStage2Name := "Installed Complete"
	cfg := map[string]any{
		"location_id":                       "loc-test",
		"create_contact":                    true,
		"inbound_stage_sync_enabled":        true,
		"inbound_sync_leadrula_pipeline_id": pipelineID,
		"inbound_sync_ghl_pipeline_id":      ghlPipelineID,
		"pipeline_stage_map": []any{
			map[string]any{
				"leadrula_pipeline_id":  pipelineID,
				"leadrula_stage_id":     stage2ID,
				"ghl_pipeline_id":       ghlPipelineID,
				"ghl_pipeline_stage_id": ghlStage2ID,
				"ghl_stage_name":        ghlStage2Name,
			},
		},
	}
	cfgJSON, _ := json.Marshal(cfg)
	if _, err := pool.Exec(ctx, `UPDATE integration_connections SET config=$2 WHERE id=$1`, connID, cfgJSON); err != nil {
		t.Fatal(err)
	}

	ids, err := svc.ProvisionGHLWebhooks(ctx, accountID, connID, connPublicID, "GHL NameSync "+suffix)
	if err != nil {
		t.Fatal(err)
	}
	defer svc.DeleteGHLWebhooks(ctx, accountID, *ids)

	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	leadID, _, err := repo.InsertLead(ctx, tx, accountID, accountID, "", nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.PlaceInPipeline(ctx, tx, leadID, accountID, pipelineID, stage1ID, nil); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}

	webhook := Webhook{
		ID:                      ids.Inbound,
		AccountID:               accountID,
		IntegrationConnectionID: &connID,
	}
	tryApplyGHLInboundStageSync(ctx, svc, webhook, leadID, map[string]any{
		"contact_id":      "contact-name-fallback",
		"pipeline_id":     ghlPipelineID,
		"pipleline_stage": ghlStage2Name,
	})

	leadAfter, err := repo.GetByID(ctx, pool, leadID)
	if err != nil {
		t.Fatal(err)
	}
	if leadAfter.StageID == nil || *leadAfter.StageID != stage2ID {
		t.Fatalf("stage_id = %v, want %d (GHL name %q)", leadAfter.StageID, stage2ID, ghlStage2Name)
	}
}

func TestTryApplyGHLInboundStageSync_leadrulaNameFallback(t *testing.T) {
	ctx := context.Background()
	pool, err := database.Connect(ctx, "postgres://crm:crm@localhost:5432/crm?sslmode=disable")
	if err != nil {
		t.Skip(err)
	}
	defer pool.Close()

	repo := leads.NewRepository(pool)
	pipesSvc := pipelines.NewService(pool, nil)
	leadSvc := leads.NewService(repo, nil, nil, pipesSvc, nil)
	svc := NewService(pool, repo, leadSvc, testEncKey(t), nil)

	var accountID int64
	if err := pool.QueryRow(ctx, `SELECT id FROM accounts ORDER BY id LIMIT 1`).Scan(&accountID); err != nil {
		t.Skip(err)
	}
	pipelineID, stage1ID, stage2ID := testAccountPipelineStages(ctx, t, pool, accountID)

	var stage2Name string
	if err := pool.QueryRow(ctx, `SELECT name FROM pipeline_stages WHERE id=$1`, stage2ID).Scan(&stage2Name); err != nil {
		t.Fatal(err)
	}

	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	connID, connPublicID := testGHLConnection(ctx, t, pool, accountID, "GHL LRNameSync "+suffix)
	ghlPipelineID := "ghl-pipe-lrname-" + suffix
	ghlStage2ID := "ghl-stage-lrname-" + suffix
	cfg := map[string]any{
		"location_id":                       "loc-test",
		"create_contact":                    true,
		"inbound_stage_sync_enabled":        true,
		"inbound_sync_leadrula_pipeline_id": pipelineID,
		"inbound_sync_ghl_pipeline_id":      ghlPipelineID,
		"pipeline_stage_map": []any{
			map[string]any{
				"leadrula_pipeline_id":  pipelineID,
				"leadrula_stage_id":     stage2ID,
				"ghl_pipeline_id":       ghlPipelineID,
				"ghl_pipeline_stage_id": ghlStage2ID,
			},
		},
	}
	cfgJSON, _ := json.Marshal(cfg)
	if _, err := pool.Exec(ctx, `UPDATE integration_connections SET config=$2 WHERE id=$1`, connID, cfgJSON); err != nil {
		t.Fatal(err)
	}

	ids, err := svc.ProvisionGHLWebhooks(ctx, accountID, connID, connPublicID, "GHL LRNameSync "+suffix)
	if err != nil {
		t.Fatal(err)
	}
	defer svc.DeleteGHLWebhooks(ctx, accountID, *ids)

	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	leadID, _, err := repo.InsertLead(ctx, tx, accountID, accountID, "", nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.PlaceInPipeline(ctx, tx, leadID, accountID, pipelineID, stage1ID, nil); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}

	webhook := Webhook{
		ID:                      ids.Inbound,
		AccountID:               accountID,
		IntegrationConnectionID: &connID,
	}
	tryApplyGHLInboundStageSync(ctx, svc, webhook, leadID, map[string]any{
		"contact_id":      "contact-lrname-fallback",
		"pipeline_id":     ghlPipelineID,
		"pipleline_stage": stage2Name,
	})

	leadAfter, err := repo.GetByID(ctx, pool, leadID)
	if err != nil {
		t.Fatal(err)
	}
	if leadAfter.StageID == nil || *leadAfter.StageID != stage2ID {
		t.Fatalf("stage_id = %v, want %d (Leadrula name %q, no ghl_stage_name in map)", leadAfter.StageID, stage2ID, stage2Name)
	}
}

func testOtherPipelineStage(ctx context.Context, t *testing.T, pool *pgxpool.Pool, accountID, excludePipelineID int64) (pipelineID, stageID int64) {
	t.Helper()
	err := pool.QueryRow(ctx,
		`SELECT p.id, s.id
		 FROM pipelines p
		 JOIN pipeline_stages s ON s.pipeline_id = p.id
		 WHERE p.account_id = $1 AND p.id <> $2 AND s.stage_type = 'standard'
		 ORDER BY p.id, s.position
		 LIMIT 1`,
		accountID, excludePipelineID,
	).Scan(&pipelineID, &stageID)
	if err != nil {
		t.Skipf("need second pipeline with standard stage: %v", err)
	}
	return pipelineID, stageID
}

func TestTryApplyGHLInboundStageSync_leadOutsideSyncPipeline(t *testing.T) {
	ctx := context.Background()
	pool, err := database.Connect(ctx, "postgres://crm:crm@localhost:5432/crm?sslmode=disable")
	if err != nil {
		t.Skip(err)
	}
	defer pool.Close()

	repo := leads.NewRepository(pool)
	pipesSvc := pipelines.NewService(pool, nil)
	leadSvc := leads.NewService(repo, nil, nil, pipesSvc, nil)
	svc := NewService(pool, repo, leadSvc, testEncKey(t), nil)

	var accountID int64
	if err := pool.QueryRow(ctx, `SELECT id FROM accounts ORDER BY id LIMIT 1`).Scan(&accountID); err != nil {
		t.Skip(err)
	}
	syncPipelineID, stage1ID, stage2ID := testAccountPipelineStages(ctx, t, pool, accountID)
	otherPipelineID, otherStageID := testOtherPipelineStage(ctx, t, pool, accountID, syncPipelineID)

	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	connID, connPublicID := testGHLConnection(ctx, t, pool, accountID, "GHL CrossPipeSkip "+suffix)
	ghlPipelineID := "ghl-pipe-cross-" + suffix
	ghlStage2ID := "ghl-stage-cross-" + suffix
	cfg := map[string]any{
		"location_id":                       "loc-test",
		"create_contact":                    true,
		"inbound_stage_sync_enabled":        true,
		"inbound_sync_leadrula_pipeline_id": syncPipelineID,
		"inbound_sync_ghl_pipeline_id":      ghlPipelineID,
		"pipeline_stage_map": []any{
			map[string]any{
				"leadrula_pipeline_id":  syncPipelineID,
				"leadrula_stage_id":     stage2ID,
				"ghl_pipeline_id":       ghlPipelineID,
				"ghl_pipeline_stage_id": ghlStage2ID,
			},
		},
	}
	cfgJSON, _ := json.Marshal(cfg)
	if _, err := pool.Exec(ctx, `UPDATE integration_connections SET config=$2 WHERE id=$1`, connID, cfgJSON); err != nil {
		t.Fatal(err)
	}

	ids, err := svc.ProvisionGHLWebhooks(ctx, accountID, connID, connPublicID, "GHL CrossPipeSkip "+suffix)
	if err != nil {
		t.Fatal(err)
	}
	defer svc.DeleteGHLWebhooks(ctx, accountID, *ids)

	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	leadID, _, err := repo.InsertLead(ctx, tx, accountID, accountID, "", nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.PlaceInPipeline(ctx, tx, leadID, accountID, otherPipelineID, otherStageID, nil); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}

	webhook := Webhook{
		ID:                      ids.Inbound,
		AccountID:               accountID,
		IntegrationConnectionID: &connID,
		Name:                    "GHL CrossPipeSkip",
	}
	tryApplyGHLInboundStageSync(ctx, svc, webhook, leadID, map[string]any{
		"pipelineId":      ghlPipelineID,
		"pipelineStageId": ghlStage2ID,
	})

	leadAfter, err := repo.GetByID(ctx, pool, leadID)
	if err != nil {
		t.Fatal(err)
	}
	if leadAfter.PipelineID == nil || *leadAfter.PipelineID != otherPipelineID {
		t.Fatalf("pipeline_id = %v, want %d", leadAfter.PipelineID, otherPipelineID)
	}
	if leadAfter.StageID == nil || *leadAfter.StageID != otherStageID {
		t.Fatalf("stage_id = %v, want %d (should not revert to sync pipeline stage %d)", leadAfter.StageID, otherStageID, stage2ID)
	}
	_ = stage1ID
}

func TestTryApplyGHLInboundStageSync_pendingOutboundSkipped(t *testing.T) {
	ctx := context.Background()
	pool, err := database.Connect(ctx, "postgres://crm:crm@localhost:5432/crm?sslmode=disable")
	if err != nil {
		t.Skip(err)
	}
	defer pool.Close()

	repo := leads.NewRepository(pool)
	pipesSvc := pipelines.NewService(pool, nil)
	leadSvc := leads.NewService(repo, nil, nil, pipesSvc, nil)
	svc := NewService(pool, repo, leadSvc, testEncKey(t), nil)

	var accountID int64
	if err := pool.QueryRow(ctx, `SELECT id FROM accounts ORDER BY id LIMIT 1`).Scan(&accountID); err != nil {
		t.Skip(err)
	}
	pipelineID, stage1ID, stage2ID := testAccountPipelineStages(ctx, t, pool, accountID)

	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	connID, connPublicID := testGHLConnection(ctx, t, pool, accountID, "GHL PendingSkip "+suffix)
	ghlPipelineID := "ghl-pipe-pending-" + suffix
	ghlStage2ID := "ghl-stage-pending-" + suffix
	cfg := map[string]any{
		"location_id":                       "loc-test",
		"create_contact":                    true,
		"inbound_stage_sync_enabled":        true,
		"inbound_sync_leadrula_pipeline_id": pipelineID,
		"inbound_sync_ghl_pipeline_id":      ghlPipelineID,
		"pipeline_stage_map": []any{
			map[string]any{
				"leadrula_pipeline_id":  pipelineID,
				"leadrula_stage_id":     stage2ID,
				"ghl_pipeline_id":       ghlPipelineID,
				"ghl_pipeline_stage_id": ghlStage2ID,
			},
		},
	}
	cfgJSON, _ := json.Marshal(cfg)
	if _, err := pool.Exec(ctx, `UPDATE integration_connections SET config=$2 WHERE id=$1`, connID, cfgJSON); err != nil {
		t.Fatal(err)
	}

	ids, err := svc.ProvisionGHLWebhooks(ctx, accountID, connID, connPublicID, "GHL PendingSkip "+suffix)
	if err != nil {
		t.Fatal(err)
	}
	defer svc.DeleteGHLWebhooks(ctx, accountID, *ids)

	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	leadID, _, err := repo.InsertLead(ctx, tx, accountID, accountID, "", nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.PlaceInPipeline(ctx, tx, leadID, accountID, pipelineID, stage1ID, nil); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}

	var queueID int64
	if err := pool.QueryRow(ctx,
		`INSERT INTO integration_delivery_queue (lead_id, connection_id, payload, status)
		 VALUES ($1, $2, '{}', 'pending'::delivery_status)
		 RETURNING id`, leadID, connID).Scan(&queueID); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, `DELETE FROM integration_delivery_queue WHERE id = $1`, queueID)
	})

	webhook := Webhook{
		ID:                      ids.Inbound,
		AccountID:               accountID,
		IntegrationConnectionID: &connID,
		Name:                    "GHL PendingSkip",
	}
	tryApplyGHLInboundStageSync(ctx, svc, webhook, leadID, map[string]any{
		"pipelineId":      ghlPipelineID,
		"pipelineStageId": ghlStage2ID,
	})

	leadAfter, err := repo.GetByID(ctx, pool, leadID)
	if err != nil {
		t.Fatal(err)
	}
	if leadAfter.StageID == nil || *leadAfter.StageID != stage1ID {
		t.Fatalf("stage_id = %v, want %d (inbound should skip while outbound pending)", leadAfter.StageID, stage1ID)
	}
}
