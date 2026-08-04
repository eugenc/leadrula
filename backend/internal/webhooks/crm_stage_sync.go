package webhooks

import (
	"context"

	"github.com/echayko/leadrula/backend/internal/integrations/providers"
	"github.com/echayko/leadrula/backend/internal/pipelines"
)

func tryApplyCRMInboundStageSync(ctx context.Context, s *Service, webhook Webhook, leadID int64, flat map[string]any, providerSlug string) {
	if s == nil || s.leadSvc == nil || leadID == 0 || providerSlug == "" {
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

	cfg := providers.ParseInboundStageSync(providers.GHLConfigFromJSON(configJSON))
	if !providers.CRMInboundStageSyncReady(cfg) {
		return
	}

	crmPipelineID, crmStageID := providers.CRMInboundPipelineStage(slug, flat)
	if crmPipelineID == "" || crmStageID == "" {
		return
	}
	if crmPipelineID != cfg.CRMPipelineID {
		return
	}

	targetStageID, ok := providers.ResolveCRMLeadrulaStage(cfg.PipelineStageMap, cfg.LeadrulaPipelineID, crmPipelineID, crmStageID)
	if !ok {
		return
	}

	lead, err := s.leads.GetByID(ctx, s.leads.Pool(), leadID)
	if err != nil {
		return
	}
	if lead.StageID != nil && *lead.StageID == targetStageID {
		return
	}

	stage, err := s.leads.GetStage(ctx, s.leads.Pool(), targetStageID)
	if err != nil {
		return
	}
	if stage.StageType != pipelines.StageTypeStandard {
		return
	}

	_, _ = s.leadSvc.ChangeStageByWebhook(ctx, webhook.AccountID, leadID, targetStageID, nil, nil, webhook.Name, connID)
}

func tryApplyGHLInboundStageSync(ctx context.Context, s *Service, webhook Webhook, leadID int64, flat map[string]any) {
	tryApplyCRMInboundStageSync(ctx, s, webhook, leadID, flat, "ghl")
}
