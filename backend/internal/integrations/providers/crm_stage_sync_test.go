package providers

import "testing"

func TestResolveCRMLeadrulaStage_crmKeys(t *testing.T) {
	entries := []CRMPipelineStageMapEntry{
		{
			LeadrulaPipelineID: 1,
			LeadrulaStageID:    10,
			CRMPipelineID:      "pipe-a",
			CRMStageID:         "stage-b",
		},
	}
	id, ok := ResolveCRMLeadrulaStage(entries, 1, "pipe-a", "stage-b")
	if !ok || id != 10 {
		t.Fatalf("got id=%d ok=%v", id, ok)
	}
}

func TestResolveCRMLeadrulaStage_ghlLegacyFallback(t *testing.T) {
	entries := []CRMPipelineStageMapEntry{
		{
			LeadrulaPipelineID: 2,
			LeadrulaStageID:    20,
			GHLPipelineID:      "ghl-pipe",
			GHLPipelineStageID: "ghl-stage",
		},
	}
	id, ok := ResolveCRMLeadrulaStage(entries, 2, "ghl-pipe", "ghl-stage")
	if !ok || id != 20 {
		t.Fatalf("got id=%d ok=%v", id, ok)
	}
}

func TestCRMInboundStageSyncReady(t *testing.T) {
	cfg := InboundStageSyncConfig{
		Enabled:            true,
		LeadrulaPipelineID: 1,
		CRMPipelineID:      "p1",
		PipelineStageMap: []CRMPipelineStageMapEntry{
			{LeadrulaPipelineID: 1, LeadrulaStageID: 5, CRMPipelineID: "p1", CRMStageID: "s1"},
		},
	}
	if !CRMInboundStageSyncReady(cfg) {
		t.Fatal("expected ready")
	}
	cfg.PipelineStageMap = nil
	if CRMInboundStageSyncReady(cfg) {
		t.Fatal("expected not ready without map")
	}
}

func TestCRMInboundPipelineStage_pipedrive(t *testing.T) {
	pipe, stage := CRMInboundPipelineStage("pipedrive", map[string]any{
		"current.pipeline_id": "9",
		"current.stage_id":    "42",
		"current.person_id":   "1001",
	})
	if pipe != "9" || stage != "42" {
		t.Fatalf("pipeline=%q stage=%q", pipe, stage)
	}
	if CRMInboundContactID("pipedrive", map[string]any{"current.person_id": "1001"}) != "1001" {
		t.Fatal("expected person_id")
	}
}

func TestCRMInboundPipelineStage_hubspot(t *testing.T) {
	pipe, stage := CRMInboundPipelineStage("hubspot", map[string]any{
		"propertyName":  "dealstage",
		"propertyValue": "appointmentscheduled",
		"pipeline":      "default",
	})
	if pipe != "default" || stage != "appointmentscheduled" {
		t.Fatalf("pipeline=%q stage=%q", pipe, stage)
	}
}

func TestCRMInboundPipelineStage_ghlUnchanged(t *testing.T) {
	pipe, stage := CRMInboundPipelineStage("ghl", map[string]any{
		"pipelineId":      "p-1",
		"pipelineStageId": "s-2",
	})
	if pipe != "p-1" || stage != "s-2" {
		t.Fatalf("pipeline=%q stage=%q", pipe, stage)
	}
}

func TestParseInboundStageSync_ghlPipelineFallback(t *testing.T) {
	cfg := ParseInboundStageSync(map[string]any{
		"inbound_stage_sync_enabled":        true,
		"inbound_sync_leadrula_pipeline_id":  float64(3),
		"inbound_sync_ghl_pipeline_id":      "ghl-p",
		"pipeline_stage_map": []any{
			map[string]any{
				"leadrula_pipeline_id":  float64(3),
				"leadrula_stage_id":     float64(7),
				"ghl_pipeline_id":       "ghl-p",
				"ghl_pipeline_stage_id": "ghl-s",
			},
		},
	})
	if cfg.CRMPipelineID != "ghl-p" {
		t.Fatalf("CRMPipelineID=%q", cfg.CRMPipelineID)
	}
	if !CRMInboundStageSyncReady(cfg) {
		t.Fatal("expected ready with ghl legacy keys")
	}
}

func TestDiagnoseCRMInboundStageSync_missingStageFields(t *testing.T) {
	cfg := InboundStageSyncConfig{
		Enabled:            true,
		LeadrulaPipelineID: 1,
		CRMPipelineID:      "p1",
		PipelineStageMap: []CRMPipelineStageMapEntry{
			{LeadrulaPipelineID: 1, LeadrulaStageID: 5, CRMPipelineID: "p1", CRMStageID: "s1"},
		},
	}
	diag := DiagnoseCRMInboundStageSync("ghl", map[string]any{"contact_id": "c1"}, cfg, nil)
	if diag.CanSync || diag.SkipReason != "payload missing pipelineId or pipelineStageId" {
		t.Fatalf("diag = %+v", diag)
	}
}

func TestDiagnoseCRMInboundStageSync_ready(t *testing.T) {
	cfg := InboundStageSyncConfig{
		Enabled:            true,
		LeadrulaPipelineID: 1,
		CRMPipelineID:      "p1",
		PipelineStageMap: []CRMPipelineStageMapEntry{
			{LeadrulaPipelineID: 1, LeadrulaStageID: 5, CRMPipelineID: "p1", CRMStageID: "s1"},
		},
	}
	diag := DiagnoseCRMInboundStageSync("ghl", map[string]any{
		"pipelineId":      "p1",
		"pipelineStageId": "s1",
	}, cfg, nil)
	if !diag.CanSync || diag.TargetStageID != 5 {
		t.Fatalf("diag = %+v", diag)
	}
}
