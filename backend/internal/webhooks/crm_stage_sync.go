package webhooks

import (
	"context"
	"fmt"

	"github.com/echayko/leadrula/backend/internal/integrations/providers"
	"github.com/echayko/leadrula/backend/internal/leads"
	"github.com/echayko/leadrula/backend/internal/pipelines"
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

	diag := providers.DiagnoseCRMInboundStageSync(slug, flat, syncCfg, lead.StageID)
	if !diag.CanSync && slug == "ghl" {
		if byName, ok := tryGHLInboundStageSyncByName(flat, syncCfg, lead.StageID); ok {
			diag = byName
		}
	}
	actor := leads.ActorWebhook(webhook.Name)
	if !diag.CanSync {
		if diag.SkipReason != "" && diag.SkipReason != "lead already at target stage" {
			_ = s.leads.LogCRMSyncSkipped(ctx, s.leads.Pool(), leadID, lead.OwnerAccountID, actor, diag.SkipReason)
		}
		return
	}

	stage, err := s.leads.GetStage(ctx, s.leads.Pool(), diag.TargetStageID)
	if err != nil {
		_ = s.leads.LogCRMSyncSkipped(ctx, s.leads.Pool(), leadID, lead.OwnerAccountID, actor, "target stage not found")
		return
	}
	if stage.StageType != pipelines.StageTypeStandard {
		_ = s.leads.LogCRMSyncSkipped(ctx, s.leads.Pool(), leadID, lead.OwnerAccountID, actor,
			fmt.Sprintf("target stage type %q is not a standard pipeline stage", stage.StageType))
		return
	}

	if _, err := s.leadSvc.ChangeStageByWebhook(ctx, webhook.AccountID, leadID, diag.TargetStageID, nil, nil, webhook.Name, connID); err != nil {
		_ = s.leads.LogCRMSyncSkipped(ctx, s.leads.Pool(), leadID, lead.OwnerAccountID, actor, "stage move failed: "+err.Error())
	}
}

func tryApplyGHLInboundStageSync(ctx context.Context, s *Service, webhook Webhook, leadID int64, flat map[string]any) {
	tryApplyCRMInboundStageSync(ctx, s, webhook, leadID, flat, "ghl")
}

func tryGHLInboundStageSyncByName(flat map[string]any, syncCfg providers.InboundStageSyncConfig, currentStageID *int64) (providers.InboundStageSyncDiagnosis, bool) {
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
		return providers.InboundStageSyncDiagnosis{
			SkipReason:    fmt.Sprintf("no stage map entry for GHL stage name %q", stageName),
			CRMPipelineID: crmPipelineID,
		}, false
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
