package webhooks

import (
	"context"
	"fmt"
	"strings"

	"github.com/echayko/leadrula/backend/internal/integrations/providers"
	"github.com/echayko/leadrula/backend/internal/leads"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ghlInboundOpportunityLookup is overridable in tests.
var ghlInboundOpportunityLookup = providers.FindGHLOpportunityByContact

func shouldTryGHLInboundStageAPIFallback(diag providers.InboundStageSyncDiagnosis, nameBased bool) bool {
	if diag.CanSync {
		return false
	}
	switch {
	case nameBased && diag.SkipReason == "lead already at target stage":
		return true
	case diag.SkipReason == "payload missing pipelineId or pipelineStageId":
		return true
	case strings.HasPrefix(diag.SkipReason, "no stage map entry for GHL stage name"):
		return true
	default:
		return false
	}
}

func (s *Service) ghlConnectionToken(ctx context.Context, connID int64) (string, error) {
	if s == nil || s.pool == nil {
		return "", fmt.Errorf("service unavailable")
	}
	var enc []byte
	if err := s.pool.QueryRow(ctx,
		`SELECT credentials FROM integration_connections WHERE id=$1`, connID).Scan(&enc); err != nil {
		return "", err
	}
	if len(enc) == 0 {
		return "", fmt.Errorf("ghl credentials missing")
	}
	creds := enc
	if enc[0] != '{' {
		var err error
		creds, err = aesDecrypt(s.encKey, enc)
		if err != nil {
			return "", err
		}
	}
	return providers.ParseGHLCredentials(creds)
}

func tryGHLInboundStageSyncFromAPI(
	ctx context.Context,
	s *Service,
	syncCfg providers.InboundStageSyncConfig,
	configJSON []byte,
	connID int64,
	lead *leads.Lead,
) (providers.InboundStageSyncDiagnosis, string) {
	if s == nil || lead == nil || lead.StageID == nil {
		return providers.InboundStageSyncDiagnosis{SkipReason: "lead has no pipeline stage"}, ""
	}
	if lead.ExternalID == nil || strings.TrimSpace(*lead.ExternalID) == "" {
		return providers.InboundStageSyncDiagnosis{SkipReason: "lead missing external_id for GHL API stage lookup"}, ""
	}

	ghlCfg, err := providers.ParseGHLConfig(providers.MergeGHLConfigDefaults(providers.GHLConfigFromJSON(configJSON)))
	if err != nil || strings.TrimSpace(ghlCfg.LocationID) == "" {
		return providers.InboundStageSyncDiagnosis{SkipReason: "GHL connection config invalid"}, ""
	}

	token, err := s.ghlConnectionToken(ctx, connID)
	if err != nil || token == "" {
		return providers.InboundStageSyncDiagnosis{}, fmt.Sprintf("GHL API token unavailable: %v", err)
	}

	opp, err := ghlInboundOpportunityLookup(ctx, token, ghlCfg.LocationID, strings.TrimSpace(*lead.ExternalID), syncCfg.CRMPipelineID)
	if err != nil {
		return providers.InboundStageSyncDiagnosis{}, fmt.Sprintf("GHL API opportunity lookup failed: %v", err)
	}
	if strings.TrimSpace(opp.PipelineStageID) == "" {
		return providers.InboundStageSyncDiagnosis{SkipReason: "GHL API returned no opportunity stage"}, ""
	}

	targetLRStageID, ok := providers.ResolveLeadrulaStage(
		ghlCfg.PipelineStageMap, syncCfg.LeadrulaPipelineID, syncCfg.CRMPipelineID, opp.PipelineStageID,
	)
	if !ok {
		return providers.InboundStageSyncDiagnosis{
			SkipReason:    fmt.Sprintf("no Leadrula stage mapped for GHL API stage %s", opp.PipelineStageID),
			CRMPipelineID: syncCfg.CRMPipelineID,
			CRMStageID:    opp.PipelineStageID,
		}, ""
	}

	currentStageID := *lead.StageID
	if currentStageID == targetLRStageID {
		return providers.InboundStageSyncDiagnosis{
			SkipReason:    "GHL API confirms lead already at target stage",
			TargetStageID: targetLRStageID,
			CRMPipelineID: syncCfg.CRMPipelineID,
			CRMStageID:    opp.PipelineStageID,
		}, ""
	}

	ghlPos, err := pipelineStagePosition(ctx, s.pool, targetLRStageID)
	if err != nil {
		return providers.InboundStageSyncDiagnosis{}, fmt.Sprintf("GHL API stage position lookup failed: %v", err)
	}
	lrPos, err := pipelineStagePosition(ctx, s.pool, currentStageID)
	if err != nil {
		return providers.InboundStageSyncDiagnosis{}, fmt.Sprintf("LR stage position lookup failed: %v", err)
	}
	if !ghlStageIsAheadOfLR(ghlPos, lrPos) {
		return providers.InboundStageSyncDiagnosis{
			SkipReason:    "GHL API stage is not ahead of current LR stage",
			TargetStageID: targetLRStageID,
			CRMPipelineID: syncCfg.CRMPipelineID,
			CRMStageID:    opp.PipelineStageID,
		}, ""
	}

	return providers.InboundStageSyncDiagnosis{
		CanSync:       true,
		TargetStageID: targetLRStageID,
		CRMPipelineID: syncCfg.CRMPipelineID,
		CRMStageID:    opp.PipelineStageID,
	}, ""
}

func ghlStageIsAheadOfLR(ghlMappedStagePos, lrStagePos int) bool {
	return ghlMappedStagePos > lrStagePos
}

func pipelineStagePosition(ctx context.Context, pool *pgxpool.Pool, stageID int64) (int, error) {
	var pos int
	err := pool.QueryRow(ctx, `SELECT position FROM pipeline_stages WHERE id=$1`, stageID).Scan(&pos)
	return pos, err
}
