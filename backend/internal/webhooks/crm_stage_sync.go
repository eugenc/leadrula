package webhooks

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/echayko/leadrula/backend/internal/integrations/providers"
	"github.com/echayko/leadrula/backend/internal/leads"
)

func tryApplyCRMInboundStageSync(ctx context.Context, s *Service, webhook Webhook, leadID int64, flat map[string]any, providerSlug string) {
	if s == nil || s.leadSvc == nil || s.leads == nil || leadID == 0 || providerSlug == "" {
		return
	}
	if webhook.IntegrationConnectionID == nil || *webhook.IntegrationConnectionID <= 0 {
		return
	}
	connID := *webhook.IntegrationConnectionID

	var configJSON []byte
	var slug string
	if err := s.pool.QueryRow(ctx,
		`SELECT c.config, p.slug FROM integration_connections c
		 JOIN integration_providers p ON p.id = c.provider_id
		 WHERE c.id=$1 AND c.account_id=$2`,
		connID, webhook.AccountID).Scan(&configJSON, &slug); err != nil {
		return
	}
	if providerSlug != "" {
		slug = providerSlug
	}

	syncCfg := providers.ParseInboundStageSync(providers.GHLConfigFromJSON(configJSON))
	if !providers.CRMInboundStageSyncReady(syncCfg) {
		return
	}

	lead, err := s.leads.GetByID(ctx, s.leads.Pool(), leadID)
	if err != nil {
		return
	}

	actor := leads.ActorWebhook(webhook.Name)
	if lead.PipelineID != nil && *lead.PipelineID != syncCfg.LeadrulaPipelineID {
		_ = s.leads.LogCRMSyncSkipped(ctx, s.leads.Pool(), leadID, lead.OwnerAccountID, actor, "lead not in inbound sync pipeline")
		return
	}

	if slug == "ghl" {
		flat = providers.NormalizeGHLInboundFlat(flat)
	}

	diag := providers.DiagnoseCRMInboundStageSync(slug, flat, syncCfg, lead.StageID, lead.PipelineID)
	nameBased := false
	if !diag.CanSync && slug == "ghl" {
		byName, tried := tryGHLInboundStageSyncByName(ctx, s, flat, syncCfg, lead.StageID)
		if tried {
			nameBased = true
			diag = byName
		}
	}
	if !diag.CanSync && slug == "ghl" && shouldTryGHLInboundStageAPIFallback(diag, nameBased) {
		apiDiag, apiErrReason := tryGHLInboundStageSyncFromAPI(ctx, s, syncCfg, configJSON, connID, lead)
		if apiDiag.CanSync {
			diag = apiDiag
			nameBased = false
		} else if apiErrReason != "" {
			_ = s.leads.LogCRMSyncSkipped(ctx, s.leads.Pool(), leadID, lead.OwnerAccountID, actor, apiErrReason)
			return
		} else if apiDiag.SkipReason != "" {
			_ = s.leads.LogCRMSyncSkipped(ctx, s.leads.Pool(), leadID, lead.OwnerAccountID, actor, apiDiag.SkipReason)
			return
		}
	}
	if !diag.CanSync {
		reason := diag.SkipReason
		if nameBased && reason == "lead already at target stage" {
			stageName := providers.GHLInboundPipelineStageName(flat)
			reason = fmt.Sprintf(`GHL pipleline_stage %q matches current LR stage — if GHL shows a different stage, the default webhook payload may be stale`, stageName)
		}
		if reason != "" && (diag.SkipReason != "lead already at target stage" || nameBased) {
			_ = s.leads.LogCRMSyncSkipped(ctx, s.leads.Pool(), leadID, lead.OwnerAccountID, actor, reason)
		}
		return
	}

	if _, err := s.leads.GetStage(ctx, s.leads.Pool(), diag.TargetStageID); err != nil {
		_ = s.leads.LogCRMSyncSkipped(ctx, s.leads.Pool(), leadID, lead.OwnerAccountID, actor, "target stage not found")
		return
	}

	actionAt, disqReasonID := crmInboundStagePromptFields(ctx, s, webhook, flat, lead)
	if _, err := s.leadSvc.ChangeStageByWebhook(ctx, webhook.AccountID, leadID, diag.TargetStageID, actionAt, disqReasonID, webhook.Name, true, connID); err != nil {
		_ = s.leads.LogCRMSyncSkipped(ctx, s.leads.Pool(), leadID, lead.OwnerAccountID, actor, "stage move failed: "+err.Error())
	}
}

func crmInboundStagePromptFields(ctx context.Context, s *Service, webhook Webhook, flat map[string]any, lead *leads.Lead) (actionAt *time.Time, disqReasonID *int64) {
	if s == nil {
		return nil, nil
	}
	var eventID int64
	if err := s.pool.QueryRow(ctx,
		`SELECT id FROM webhook_events WHERE webhook_id=$1 AND action='create' ORDER BY id LIMIT 1`,
		webhook.ID).Scan(&eventID); err == nil {
		if maps, err := s.ListFieldMap(ctx, eventID); err == nil {
			builtins, _, _ := applyFieldMaps(flat, maps)
			if v, ok := builtins["action_at"]; ok && v != "" {
				if parsed, err := leads.ParseActionAt(v); err == nil {
					actionAt = parsed
				}
			}
			if v, ok := builtins["disqualification_reason_id"]; ok && v != "" {
				if id, err := strconv.ParseInt(v, 10, 64); err == nil {
					disqReasonID = &id
				}
			}
		}
	}
	if actionAt == nil && lead != nil && lead.ActionAt != nil {
		actionAt = lead.ActionAt
	}
	return actionAt, disqReasonID
}

func tryApplyGHLInboundStageSync(ctx context.Context, s *Service, webhook Webhook, leadID int64, flat map[string]any) {
	tryApplyCRMInboundStageSync(ctx, s, webhook, leadID, flat, "ghl")
}

func tryGHLInboundStageSyncByName(ctx context.Context, s *Service, flat map[string]any, syncCfg providers.InboundStageSyncConfig, currentStageID *int64) (providers.InboundStageSyncDiagnosis, bool) {
	crmPipelineID, crmStageID := providers.GHLInboundPipelineStage(flat)
	if crmPipelineID == "" || crmStageID != "" || crmPipelineID != syncCfg.CRMPipelineID {
		return providers.InboundStageSyncDiagnosis{}, false
	}
	stageName := providers.GHLInboundPipelineStageName(flat)
	if stageName == "" {
		return providers.InboundStageSyncDiagnosis{}, false
	}

	targetStageID, ok := providers.ResolveCRMLeadrulaStageByGHLStageName(
		syncCfg.PipelineStageMap, syncCfg.LeadrulaPipelineID, crmPipelineID, stageName,
	)
	if !ok {
		targetStageID, ok = resolveGHLInboundStageByLeadrulaName(ctx, s, syncCfg, crmPipelineID, stageName)
	}
	if !ok {
		return providers.InboundStageSyncDiagnosis{
			SkipReason:    fmt.Sprintf("no stage map entry for GHL stage name %q", stageName),
			CRMPipelineID: crmPipelineID,
		}, true
	}
	if currentStageID != nil && *currentStageID == targetStageID {
		return providers.InboundStageSyncDiagnosis{
			SkipReason:    "lead already at target stage",
			TargetStageID: targetStageID,
			CRMPipelineID: crmPipelineID,
		}, true
	}
	return providers.InboundStageSyncDiagnosis{
		CanSync:       true,
		TargetStageID: targetStageID,
		CRMPipelineID: crmPipelineID,
	}, true
}

func resolveGHLInboundStageByLeadrulaName(ctx context.Context, s *Service, syncCfg providers.InboundStageSyncConfig, crmPipelineID, stageName string) (int64, bool) {
	if s == nil || s.pool == nil {
		return 0, false
	}
	var targetStageID int64
	err := s.pool.QueryRow(ctx,
		`SELECT ps.id FROM pipeline_stages ps
		 WHERE ps.pipeline_id = $1 AND lower(trim(ps.name)) = lower(trim($2))
		 LIMIT 1`,
		syncCfg.LeadrulaPipelineID, stageName).Scan(&targetStageID)
	if err != nil {
		return 0, false
	}
	if !providers.HasCRMStageMapEntry(syncCfg.PipelineStageMap, syncCfg.LeadrulaPipelineID, crmPipelineID, targetStageID) {
		return 0, false
	}
	return targetStageID, true
}
