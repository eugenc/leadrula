package integrations

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/echayko/leadrula/backend/internal/integrations/providers"
	"github.com/echayko/leadrula/backend/pkg/httpx"
)

// ConnectionStageMap returns pipeline_stage_map entries from a connection config.
func (s *Service) ConnectionStageMap(ctx context.Context, accountID, connectionID int64) ([]providers.GHLPipelineStageMapEntry, error) {
	conn, err := s.GetConnection(ctx, accountID, connectionID)
	if err != nil {
		return nil, err
	}
	return providers.PipelineStageMapFromConfig(configMap(conn.Config)), nil
}

// MergeCRMPipelineStageMap appends pipeline/stage map entries to any supported CRM connection.
// When inboundLRPipelineID and inboundCRMPipelineID are set, inbound stage sync is enabled for that pipeline pair.
func (s *Service) MergeCRMPipelineStageMap(ctx context.Context, accountID, connectionID int64, newEntries []providers.GHLPipelineStageMapEntry, inboundLRPipelineID int64, inboundCRMPipelineID string) error {
	conn, err := s.GetConnection(ctx, accountID, connectionID)
	if err != nil {
		return err
	}
	if !providers.CRMPipelineImportSupported(conn.ProviderSlug) {
		return httpx.Validation(conn.ProviderSlug + " does not support pipeline stage mapping")
	}
	return s.mergePipelineStageMap(ctx, accountID, connectionID, configMap(conn.Config), newEntries, inboundLRPipelineID, inboundCRMPipelineID)
}

// MergeGHLPipelineStageMap appends new GHL pipeline/stage map entries to a connection config.
func (s *Service) MergeGHLPipelineStageMap(ctx context.Context, accountID, connectionID int64, newEntries []providers.GHLPipelineStageMapEntry) error {
	return s.MergeCRMPipelineStageMap(ctx, accountID, connectionID, newEntries, 0, "")
}

func (s *Service) mergePipelineStageMap(ctx context.Context, accountID, connectionID int64, cfg map[string]any, newEntries []providers.GHLPipelineStageMapEntry, inboundLRPipelineID int64, inboundCRMPipelineID string) error {
	merged := providers.MergePipelineStageMapEntries(
		providers.PipelineStageMapFromConfig(cfg),
		newEntries,
	)
	entries := make([]map[string]any, 0, len(merged))
	for _, e := range merged {
		row := map[string]any{
			"leadrula_pipeline_id": e.LeadrulaPipelineID,
			"leadrula_stage_id":     e.LeadrulaStageID,
			"crm_pipeline_id":      firstNonEmpty(e.CRMPipelineID, e.GHLPipelineID),
			"crm_stage_id":         firstNonEmpty(e.CRMStageID, e.GHLPipelineStageID),
		}
		if e.GHLPipelineID != "" {
			row["ghl_pipeline_id"] = e.GHLPipelineID
		}
		if e.GHLPipelineStageID != "" {
			row["ghl_pipeline_stage_id"] = e.GHLPipelineStageID
		}
		if name := firstNonEmpty(e.CRMStageName, e.GHLStageName); name != "" {
			row["crm_stage_name"] = name
			row["ghl_stage_name"] = name
		}
		entries = append(entries, row)
	}
	cfg["pipeline_stage_map"] = entries
	if inboundLRPipelineID > 0 && strings.TrimSpace(inboundCRMPipelineID) != "" {
		cfg["inbound_stage_sync_enabled"] = true
		cfg["inbound_sync_leadrula_pipeline_id"] = inboundLRPipelineID
		cfg["inbound_sync_crm_pipeline_id"] = inboundCRMPipelineID
		cfg["inbound_sync_ghl_pipeline_id"] = inboundCRMPipelineID
	}
	configJSON, _ := json.Marshal(cfg)
	_, err := s.pool.Exec(ctx,
		`UPDATE integration_connections SET config=$2, updated_at=now() WHERE id=$1 AND account_id=$3`,
		connectionID, configJSON, accountID)
	return err
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

func (s *Service) FetchCRMPipelines(ctx context.Context, accountID, connectionID int64) ([]providers.CRMPipeline, string, error) {
	conn, err := s.GetConnection(ctx, accountID, connectionID)
	if err != nil {
		return nil, "", err
	}
	if !providers.CRMPipelineImportSupported(conn.ProviderSlug) {
		return nil, "", httpx.Validation(conn.ProviderSlug + " does not support CRM pipeline import")
	}
	credentials, err := s.connectionCredentialsRaw(ctx, accountID, connectionID)
	if err != nil {
		return nil, "", err
	}
	pipelines, err := providers.FetchCRMPipelines(ctx, conn.ProviderSlug, credentials, configMap(conn.Config))
	if err != nil {
		return nil, "", err
	}
	return pipelines, conn.ProviderSlug, nil
}

func (s *Service) connectionCredentialsRaw(ctx context.Context, accountID, connectionID int64) ([]byte, error) {
	var encCredentials []byte
	err := s.pool.QueryRow(ctx,
		`SELECT credentials FROM integration_connections WHERE id=$1 AND account_id=$2`,
		connectionID, accountID).Scan(&encCredentials)
	if err != nil {
		return nil, err
	}
	if len(encCredentials) == 0 {
		return nil, nil
	}
	return decrypt(s.encKey, encCredentials)
}
