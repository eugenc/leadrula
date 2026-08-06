package integrations

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/echayko/leadrula/backend/internal/database"
	"github.com/echayko/leadrula/backend/internal/integrations/providers"
	"github.com/echayko/leadrula/backend/internal/leads"
	"github.com/jackc/pgx/v5/pgxpool"
)

func planTestPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	ctx := context.Background()
	pool, err := database.Connect(ctx, "postgres://crm:crm@localhost:5432/crm?sslmode=disable")
	if err != nil {
		t.Skip(err)
	}
	t.Cleanup(func() { pool.Close() })
	return pool
}

func planTestLead(ctx context.Context, t *testing.T, pool *pgxpool.Pool) (accountID, pipelineID, sitStageID, installedStageID, leadID int64, contactID string) {
	t.Helper()
	repo := leads.NewRepository(pool)
	if err := pool.QueryRow(ctx, `SELECT id FROM accounts ORDER BY id LIMIT 1`).Scan(&accountID); err != nil {
		t.Skip(err)
	}
	if err := pool.QueryRow(ctx,
		`SELECT p.id, MIN(ps.id), MAX(ps.id)
		 FROM pipelines p
		 JOIN pipeline_stages ps ON ps.pipeline_id = p.id
		 WHERE p.account_id = $1
		 GROUP BY p.id
		 HAVING COUNT(ps.id) >= 2
		 ORDER BY p.id
		 LIMIT 1`, accountID).Scan(&pipelineID, &sitStageID, &installedStageID); err != nil {
		t.Skip(err)
	}

	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	contactID = "contact-plan-" + suffix
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var err2 error
	leadID, _, err2 = repo.InsertLead(ctx, tx, accountID, accountID, "", nil)
	if err2 != nil {
		t.Fatal(err2)
	}
	if err := repo.SetExternalID(ctx, tx, leadID, contactID); err != nil {
		t.Fatal(err)
	}
	if err := repo.PlaceInPipeline(ctx, tx, leadID, accountID, pipelineID, installedStageID, nil); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	return accountID, pipelineID, sitStageID, installedStageID, leadID, contactID
}

func planTestConnConfig(pipelineID, sitStageID, installedStageID int64, ghlPipelineID, ghlSitStageID, ghlInstalledStageID string) map[string]any {
	return map[string]any{
		"location_id":                       "loc-plan-test",
		"create_opportunity":                true,
		"inbound_stage_sync_enabled":        true,
		"inbound_sync_leadrula_pipeline_id": pipelineID,
		"inbound_sync_ghl_pipeline_id":      ghlPipelineID,
		"pipeline_stage_map": []any{
			map[string]any{
				"leadrula_pipeline_id":  pipelineID,
				"leadrula_stage_id":     sitStageID,
				"ghl_pipeline_id":       ghlPipelineID,
				"ghl_pipeline_stage_id": ghlSitStageID,
				"ghl_stage_name":        "Sit",
			},
			map[string]any{
				"leadrula_pipeline_id":  pipelineID,
				"leadrula_stage_id":     installedStageID,
				"ghl_pipeline_id":       ghlPipelineID,
				"ghl_pipeline_stage_id": ghlInstalledStageID,
				"ghl_stage_name":        "Installed",
			},
		},
	}
}

func TestPlanGHLOutboundDeliver_stageMismatchDeliverFull(t *testing.T) {
	ctx := context.Background()
	pool := planTestPool(t)
	_, pipelineID, sitStageID, installedStageID, leadID, contactID := planTestLead(ctx, t, pool)

	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	ghlPipelineID := "ghl-pipe-plan-" + suffix
	ghlSitStageID := "ghl-sit-" + suffix
	ghlInstalledStageID := "ghl-installed-" + suffix
	connConfig := planTestConnConfig(pipelineID, sitStageID, installedStageID, ghlPipelineID, ghlSitStageID, ghlInstalledStageID)

	prev := ghlOutboundOpportunityLookup
	ghlOutboundOpportunityLookup = func(_ context.Context, _, _, gotContactID, _ string) (providers.GHLOpportunityRef, error) {
		if gotContactID != contactID {
			t.Fatalf("contactID = %q, want %q", gotContactID, contactID)
		}
		return providers.GHLOpportunityRef{
			ID:              "opp-plan-" + suffix,
			PipelineID:      ghlPipelineID,
			PipelineStageID: ghlSitStageID,
		}, nil
	}
	t.Cleanup(func() { ghlOutboundOpportunityLookup = prev })

	payload, _ := json.Marshal(map[string]any{
		"first_name":  "Paul",
		"last_name":   "Toro",
		"phone":       "555",
		"pipeline_id": pipelineID,
		"stage_id":    installedStageID,
	})
	s := &Service{pool: pool}
	plan := s.planGHLOutboundDeliver(ctx, leadID, "token", connConfig, payload, payload)
	if plan.Action != ghlOutboundDeliverFull {
		t.Fatalf("action = %q, want %q when GHL is behind Leadrula", plan.Action, ghlOutboundDeliverFull)
	}
}

func TestPlanGHLOutboundDeliver_alreadySyncedSkips(t *testing.T) {
	ctx := context.Background()
	pool := planTestPool(t)
	_, pipelineID, sitStageID, installedStageID, leadID, contactID := planTestLead(ctx, t, pool)

	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	ghlPipelineID := "ghl-pipe-plan-" + suffix
	ghlSitStageID := "ghl-sit-" + suffix
	ghlInstalledStageID := "ghl-installed-" + suffix
	connConfig := planTestConnConfig(pipelineID, sitStageID, installedStageID, ghlPipelineID, ghlSitStageID, ghlInstalledStageID)

	prev := ghlOutboundOpportunityLookup
	ghlOutboundOpportunityLookup = func(_ context.Context, _, _, gotContactID, _ string) (providers.GHLOpportunityRef, error) {
		if gotContactID != contactID {
			t.Fatalf("contactID = %q, want %q", gotContactID, contactID)
		}
		return providers.GHLOpportunityRef{
			ID:              "opp-plan-" + suffix,
			PipelineID:      ghlPipelineID,
			PipelineStageID: ghlInstalledStageID,
		}, nil
	}
	t.Cleanup(func() { ghlOutboundOpportunityLookup = prev })

	payload, _ := json.Marshal(map[string]any{
		"first_name":  "Paul",
		"last_name":   "Toro",
		"phone":       "555",
		"pipeline_id": pipelineID,
		"stage_id":    installedStageID,
	})
	s := &Service{pool: pool}
	plan := s.planGHLOutboundDeliver(ctx, leadID, "token", connConfig, payload, payload)
	if plan.Action != ghlOutboundDeliverSkip {
		t.Fatalf("action = %q, want %q when GHL already matches mapped target", plan.Action, ghlOutboundDeliverSkip)
	}
}

func TestGHLDeliveryContactPayloadChanged(t *testing.T) {
	cfg, err := providers.ParseGHLConfig(providers.MergeGHLConfigDefaults(map[string]any{"location_id": "loc1"}))
	if err != nil {
		t.Fatal(err)
	}
	base := map[string]any{
		"first_name":  "Jane",
		"last_name":   "Doe",
		"phone":       "555",
		"pipeline_id": float64(1),
		"stage_id":    float64(2),
	}
	baseJSON, _ := json.Marshal(base)

	changedPhone := map[string]any{
		"first_name":  "Jane",
		"last_name":   "Doe",
		"phone":       "556",
		"pipeline_id": float64(1),
		"stage_id":    float64(3),
	}
	changedPhoneJSON, _ := json.Marshal(changedPhone)

	if providers.GHLContactPayloadChanged(cfg, baseJSON, baseJSON) {
		t.Fatal("expected identical payloads to be unchanged")
	}
	if !providers.GHLContactPayloadChanged(cfg, baseJSON, changedPhoneJSON) {
		t.Fatal("expected phone change to be detected")
	}
	stageOnly := map[string]any{
		"first_name":  "Jane",
		"last_name":   "Doe",
		"phone":       "555",
		"pipeline_id": float64(1),
		"stage_id":    float64(99),
	}
	stageOnlyJSON, _ := json.Marshal(stageOnly)
	if providers.GHLContactPayloadChanged(cfg, baseJSON, stageOnlyJSON) {
		t.Fatal("stage-only change should be ignored for contact diff")
	}
}

func TestGHLContactPayloadSlice_ignoresStage(t *testing.T) {
	cfg, err := providers.ParseGHLConfig(providers.MergeGHLConfigDefaults(map[string]any{"location_id": "loc1"}))
	if err != nil {
		t.Fatal(err)
	}
	raw := []byte(`{"first_name":"A","stage_id":1,"custom_fields":{"42":"x"}}`)
	slice := providers.GHLContactPayloadSlice(cfg, raw)
	if _, ok := slice["stage_id"]; ok {
		t.Fatal("stage_id should not be in contact slice")
	}
	if slice["first_name"] != "A" {
		t.Fatalf("first_name = %v", slice["first_name"])
	}
	if _, ok := slice["custom_fields"]; ok {
		t.Fatal("unmapped custom_fields should not be included")
	}
}

func TestSetSkipOpportunityStage(t *testing.T) {
	out := setSkipOpportunityStage([]byte(`{"first_name":"Jane"}`))
	var m map[string]any
	if err := json.Unmarshal(out, &m); err != nil {
		t.Fatal(err)
	}
	cfg, ok := m["_config"].(map[string]any)
	if !ok || cfg["skip_opportunity_stage"] != true {
		t.Fatalf("_config = %v", m["_config"])
	}
}
