package integrations

import (
	"context"
	"encoding/json"

	"github.com/echayko/leadrula/backend/internal/database"
	"github.com/echayko/leadrula/backend/internal/integrations/providers"
	"github.com/echayko/leadrula/backend/internal/leads"
)

const (
	ghlOutboundDeliverFull         = "full"
	ghlOutboundDeliverSkip         = "skip"
	ghlOutboundDeliverContactOnly  = "contact_only"
	ghlAlreadyAtTargetStageReason  = "GHL already at target stage"
)

type ghlOutboundDeliverPlan struct {
	Action string
}

var ghlDeliveryContactFields = []string{
	"first_name", "last_name", "phone", "email", "address", "city", "state", "zip", "country", "source", "action_at",
}

// planGHLOutboundDeliver decides whether to skip, contact-only, or fully deliver to GHL.
// GHL API read errors return full delivery (fail-safe).
func (s *Service) planGHLOutboundDeliver(ctx context.Context, leadID int64, token string, connConfig map[string]any, enqueuedPayload, refreshedPayload []byte) ghlOutboundDeliverPlan {
	plan := ghlOutboundDeliverPlan{Action: ghlOutboundDeliverFull}
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

	opp, err := providers.FindGHLOpportunityByContact(ctx, token, cfg.LocationID, *lead.ExternalID, cfg.InboundSyncGHLPipelineID)
	if err != nil || opp.ID == "" || opp.PipelineStageID == "" {
		return plan
	}
	if opp.PipelineStageID != mappedGHLStageID {
		return plan
	}

	if GHLDeliveryContactPayloadChanged(enqueuedPayload, refreshedPayload) {
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

// GHLDeliveryContactPayloadChanged reports whether contact/custom delivery fields differ.
func GHLDeliveryContactPayloadChanged(enqueued, refreshed []byte) bool {
	a := ghlDeliveryContactSlice(enqueued)
	b := ghlDeliveryContactSlice(refreshed)
	ja, _ := json.Marshal(a)
	jb, _ := json.Marshal(b)
	return string(ja) != string(jb)
}

func ghlDeliveryContactSlice(payload []byte) map[string]any {
	out := map[string]any{}
	if len(payload) == 0 {
		return out
	}
	var m map[string]any
	if json.Unmarshal(payload, &m) != nil {
		return out
	}
	for _, k := range ghlDeliveryContactFields {
		if v, ok := m[k]; ok {
			out[k] = v
		}
	}
	if cf, ok := m["custom_fields"]; ok {
		out["custom_fields"] = cf
	}
	return out
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

// reconcileGHLStageBeforeDeliver pulls Leadrula forward when GHL already moved ahead.
// Returns refreshed delivery payload when the lead stage was updated.
func (s *Service) reconcileGHLStageBeforeDeliver(ctx context.Context, connID, leadID int64, token string, connConfig map[string]any, payload []byte) ([]byte, bool, error) {
	if s == nil || s.leadSvc == nil || s.pool == nil || leadID == 0 {
		return payload, false, nil
	}
	cfg, err := providers.ParseGHLConfig(providers.MergeGHLConfigDefaults(connConfig))
	if err != nil || !cfg.InboundStageSyncEnabled || !cfg.CreateOpportunity {
		return payload, false, nil
	}
	if cfg.InboundSyncLeadrulaPipelineID <= 0 || cfg.InboundSyncGHLPipelineID == "" {
		return payload, false, nil
	}

	repo := leads.NewRepository(s.pool)
	lead, err := repo.GetByID(ctx, s.pool, leadID)
	if err != nil {
		return payload, false, err
	}
	if lead.ExternalID == nil || *lead.ExternalID == "" {
		return payload, false, nil
	}
	if lead.PipelineID == nil || lead.StageID == nil || *lead.PipelineID != cfg.InboundSyncLeadrulaPipelineID {
		return payload, false, nil
	}

	lrPipelineID := *lead.PipelineID
	lrStageID := *lead.StageID
	lrGHLPipelineID, lrGHLStageID, err := providers.ResolveGHLStage(cfg.PipelineStageMap, lrPipelineID, lrStageID)
	if err != nil {
		return payload, false, nil
	}
	if lrGHLPipelineID != cfg.InboundSyncGHLPipelineID {
		return payload, false, nil
	}

	opp, err := providers.FindGHLOpportunityByContact(ctx, token, cfg.LocationID, *lead.ExternalID, cfg.InboundSyncGHLPipelineID)
	if err != nil || opp.ID == "" || opp.PipelineStageID == "" {
		return payload, false, err
	}
	if opp.PipelineStageID == lrGHLStageID {
		return payload, false, nil
	}

	targetLRStageID, ok := providers.ResolveLeadrulaStage(cfg.PipelineStageMap, cfg.InboundSyncLeadrulaPipelineID, cfg.InboundSyncGHLPipelineID, opp.PipelineStageID)
	if !ok || targetLRStageID <= 0 || targetLRStageID == lrStageID {
		return payload, false, nil
	}

	ghlPos, err := stagePosition(ctx, s.pool, targetLRStageID)
	if err != nil {
		return payload, false, err
	}
	lrPos, err := stagePosition(ctx, s.pool, lrStageID)
	if err != nil {
		return payload, false, err
	}
	if !ghlStageIsAheadOfLR(ghlPos, lrPos) {
		return payload, false, nil
	}
	var accountID int64
	if err := s.pool.QueryRow(ctx,
		`SELECT account_id FROM integration_connections WHERE id=$1`, connID).Scan(&accountID); err != nil {
		return payload, false, err
	}
	if _, err := s.leadSvc.ChangeStageByWebhook(ctx, accountID, leadID, targetLRStageID, nil, nil, "ghl stage reconcile", connID); err != nil {
		return payload, false, err
	}
	refreshed, err := leads.RefreshDeliveryPayload(ctx, s.pool, repo, leadID, payload)
	if err != nil {
		return payload, true, err
	}
	return refreshed, true, nil
}

func ghlStageIsAheadOfLR(ghlMappedStagePos, lrStagePos int) bool {
	return ghlMappedStagePos > lrStagePos
}

func stagePosition(ctx context.Context, q database.Querier, stageID int64) (int, error) {
	var pos int
	err := q.QueryRow(ctx, `SELECT position FROM pipeline_stages WHERE id=$1`, stageID).Scan(&pos)
	return pos, err
}

func ghlDeliveryToken(credentials []byte, providerSlug string) (string, error) {
	if providerSlug != "ghl" || len(credentials) == 0 {
		return "", nil
	}
	return providers.ParseGHLCredentials(credentials)
}
