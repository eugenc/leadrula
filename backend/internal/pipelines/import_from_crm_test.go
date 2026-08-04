package pipelines

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/echayko/leadrula/backend/internal/integrations/providers"
)

func TestBuildMapEntries_crmKeys(t *testing.T) {
	pipeIn := ImportCRMPipelineInput{
		ExternalID: "crm-pipe-1",
		Name:       "Sales",
		Stages: []ImportCRMStageInput{
			{ExternalID: "crm-stage-1", Name: "New"},
			{ExternalID: "crm-stage-2", Name: "Won", IsWon: true},
		},
	}
	stages := []Stage{
		{ID: 10, Name: "New"},
		{ID: 11, Name: "Won"},
	}
	entries := buildMapEntries(5, pipeIn, stages)
	if len(entries) != 2 {
		t.Fatalf("got %d entries", len(entries))
	}
	if entries[0].CRMPipelineID != "crm-pipe-1" || entries[0].CRMStageID != "crm-stage-1" {
		t.Fatalf("entry0: crm=%q/%q", entries[0].CRMPipelineID, entries[0].CRMStageID)
	}
	if entries[0].LeadrulaPipelineID != 5 || entries[0].LeadrulaStageID != 10 {
		t.Fatalf("entry0 leadrula ids: %d/%d", entries[0].LeadrulaPipelineID, entries[0].LeadrulaStageID)
	}
	if entries[1].GHLPipelineStageID != "crm-stage-2" {
		t.Fatalf("expected ghl legacy stage id fallback, got %q", entries[1].GHLPipelineStageID)
	}
}

func TestSyncPipelineStages_addRenameReorder(t *testing.T) {
	pool := connectPipelinesTestDB(t)
	ctx := context.Background()
	svc := newPipelinesTestService(t, pool)

	var accountID int64
	if err := pool.QueryRow(ctx, `SELECT id FROM accounts ORDER BY id LIMIT 1`).Scan(&accountID); err != nil {
		t.Skip("need an account")
	}
	p := testAdminPrincipal(accountID)

	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	pipeName := "CRM Sync Test " + suffix
	pl, stages, err := svc.importPipelineWithStages(ctx, p, pipeName, []ImportCRMStageInput{
		{ExternalID: "s1", Name: "New", Position: 0},
		{ExternalID: "s2", Name: "Qualified", Position: 1},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(stages) != 2 {
		t.Fatalf("expected 2 stages, got %d", len(stages))
	}

	stageMap := []providers.GHLPipelineStageMapEntry{
		{
			LeadrulaPipelineID: pl.ID,
			LeadrulaStageID:    stages[0].ID,
			CRMPipelineID:      "pipe-ext",
			CRMStageID:         "s1",
		},
		{
			LeadrulaPipelineID: pl.ID,
			LeadrulaStageID:    stages[1].ID,
			CRMPipelineID:      "pipe-ext",
			CRMStageID:         "s2",
		},
	}

	pipeIn := ImportCRMPipelineInput{
		ExternalID: "pipe-ext",
		Name:       pipeName,
		Stages: []ImportCRMStageInput{
			{ExternalID: "s1", Name: "New", Position: 0},
			{ExternalID: "s2", Name: "Qualified Renamed", Position: 1},
			{ExternalID: "s3", Name: "Demo", Position: 2},
		},
	}

	summary, finalStages, err := svc.syncPipelineStages(ctx, p, pl, pipeIn, stageMap)
	if err != nil {
		t.Fatal(err)
	}
	if summary.StagesAdded != 1 {
		t.Fatalf("StagesAdded=%d want 1", summary.StagesAdded)
	}
	if summary.StagesRenamed != 1 {
		t.Fatalf("StagesRenamed=%d want 1", summary.StagesRenamed)
	}
	if len(finalStages) != 3 {
		t.Fatalf("got %d final stages, want 3 (no delete)", len(finalStages))
	}
	if finalStages[1].Name != "Qualified Renamed" {
		t.Fatalf("stage[1] name=%q", finalStages[1].Name)
	}
	if finalStages[2].Name != "Demo" {
		t.Fatalf("stage[2] name=%q", finalStages[2].Name)
	}

	// Re-sync by name fallback when map is empty — should match existing, not duplicate.
	summary2, finalStages2, err := svc.syncPipelineStages(ctx, p, pl, pipeIn, nil)
	if err != nil {
		t.Fatal(err)
	}
	if summary2.StagesAdded != 0 {
		t.Fatalf("second sync added stages: %d", summary2.StagesAdded)
	}
	if len(finalStages2) != 3 {
		t.Fatalf("second sync stage count=%d", len(finalStages2))
	}
}

func TestImportFromCRM_syncExistingByName(t *testing.T) {
	pool := connectPipelinesTestDB(t)
	ctx := context.Background()
	svc := newPipelinesTestService(t, pool)

	var accountID int64
	if err := pool.QueryRow(ctx, `SELECT id FROM accounts ORDER BY id LIMIT 1`).Scan(&accountID); err != nil {
		t.Skip("need an account")
	}
	p := testAdminPrincipal(accountID)

	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	name := "CRM Import Sync " + suffix
	pl, _, err := svc.importPipelineWithStages(ctx, p, name, []ImportCRMStageInput{
		{ExternalID: "a", Name: "Stage A", Position: 0},
	})
	if err != nil {
		t.Fatal(err)
	}

	result, err := svc.ImportFromCRM(ctx, p, ImportFromCRMInput{
		ConnectionID:  1,
		ProviderSlug:  "pipedrive",
		Pipelines: []ImportCRMPipelineInput{
			{
				ExternalID: "pd-pipe",
				Name:       name,
				Stages: []ImportCRMStageInput{
					{ExternalID: "a", Name: "Stage A", Position: 0},
					{ExternalID: "b", Name: "Stage B", Position: 1},
				},
			},
		},
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Synced) != 1 {
		t.Fatalf("Synced=%d want 1", len(result.Synced))
	}
	if len(result.Created) != 0 {
		t.Fatalf("Created=%d want 0", len(result.Created))
	}
	if result.Synced[0].ID != pl.ID {
		t.Fatalf("synced pipeline id=%d want %d", result.Synced[0].ID, pl.ID)
	}
	if result.Synced[0].StagesAdded != 1 {
		t.Fatalf("StagesAdded=%d want 1", result.Synced[0].StagesAdded)
	}
}

func TestResolveImportPipelineName_noConflict(t *testing.T) {
	existing := map[string]bool{"other": true}
	name, renamed := resolveImportPipelineName("Sales", "HubSpot", existing)
	if renamed || name != "Sales" {
		t.Fatalf("got name=%q renamed=%v", name, renamed)
	}
}

func TestResolveImportPipelineName_providerSuffix(t *testing.T) {
	existing := map[string]bool{"sales": true}
	name, renamed := resolveImportPipelineName("Sales", "HubSpot", existing)
	if !renamed || name != "Sales (HubSpot)" {
		t.Fatalf("got name=%q renamed=%v", name, renamed)
	}
}

func TestResolveImportPipelineName_numericSuffix(t *testing.T) {
	existing := map[string]bool{
		"sales":           true,
		"sales (hubspot)": true,
	}
	name, renamed := resolveImportPipelineName("Sales", "HubSpot", existing)
	if !renamed || name != "Sales (HubSpot) (2)" {
		t.Fatalf("got name=%q renamed=%v", name, renamed)
	}
}
