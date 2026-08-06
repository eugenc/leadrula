package integrations

import (
	"context"
	"encoding/json"

	"github.com/echayko/leadrula/backend/internal/integrations/providers"
	"github.com/echayko/leadrula/backend/internal/leads"
)

const (
	ghlOutboundDeliverFull        = "full"
	ghlOutboundDeliverSkip        = "skip"
	ghlOutboundDeliverContactOnly = "contact_only"
	ghlAlreadyAtTargetStageReason = "GHL already at target stage"
)

// ghlOutboundOpportunityLookup is overridable in tests.
var ghlOutboundOpportunityLookup = providers.FindGHLOpportunityByContact

type ghlOutboundDeliverPlan struct {
	Action string
}

// planGHLOutboundDeliver decides whether to skip, contact-only, or fully deliver to GHL.
// GHL API read errors return full delivery (fail-safe).
func (s *Service) planGHLOutboundDeliver(ctx context.Context, leadID int64, token string, connConfig map[string]any, enqueuedPayload, refreshedPayload []byte) ghlOutboundDeliverPlan {
	plan := ghlOutboundDeliverPlan{Action: ghlOutboundDeliverFull}
	if payloadHasSkipOpportunityStage(enqueuedPayload) {
		plan.Action = ghlOutboundDeliverContactOnly
		return plan
	}
	if s == nil || s.pool == nil || leadID == 0 || token == "" {
		return plan
	}
	cfg, err := providers.ParseGHLConfig(providers.MergeGHLConfigDefaults(connConfig))
	if err != nil || !cfg.InboundStageSyncEnabled || !cfg.CreateOpportunity {
		return plan
	}
	if cfg.InboundSyncLeadrulaPipelineID <= 0 || cfg.InboundSyncGHLPipelineID == "" {
		return plan
	}

	repo := leads.NewRepository(s.pool)
	lead, err := repo.GetByID(ctx, s.pool, leadID)
	if err != nil {
		return plan
	}
	if lead.ExternalID == nil || *lead.ExternalID == "" {
		return plan
	}
	if lead.PipelineID == nil || lead.StageID == nil || *lead.PipelineID != cfg.InboundSyncLeadrulaPipelineID {
		return plan
	}

	lrPipelineID := *lead.PipelineID
	lrStageID := *lead.StageID
	_, mappedGHLStageID, err := providers.ResolveGHLStage(cfg.PipelineStageMap, lrPipelineID, lrStageID)
	if err != nil {
		return plan
	}

	opp, err := ghlOutboundOpportunityLookup(ctx, token, cfg.LocationID, *lead.ExternalID, cfg.InboundSyncGHLPipelineID)
	if err != nil || opp.ID == "" || opp.PipelineStageID == "" {
		return plan
	}
	if opp.PipelineStageID != mappedGHLStageID {
		return plan
	}

	if providers.GHLContactPayloadChanged(cfg, enqueuedPayload, refreshedPayload) {
		if cfg.DeliveryMode == "webhook" {
			plan.Action = ghlOutboundDeliverSkip
			return plan
		}
		plan.Action = ghlOutboundDeliverContactOnly
		return plan
	}
	plan.Action = ghlOutboundDeliverSkip
	return plan
}

func setSkipOpportunityStage(payload []byte) []byte {
	var m map[string]any
	if json.Unmarshal(payload, &m) != nil {
		m = map[string]any{}
	}
	cfg, _ := m["_config"].(map[string]any)
	if cfg == nil {
		cfg = map[string]any{}
	}
	cfg["skip_opportunity_stage"] = true
	m["_config"] = cfg
	b, err := json.Marshal(m)
	if err != nil {
		return payload
	}
	return b
}

func ghlDeliveryToken(credentials []byte, providerSlug string) (string, error) {
	if providerSlug != "ghl" || len(credentials) == 0 {
		return "", nil
	}
	return providers.ParseGHLCredentials(credentials)
}
