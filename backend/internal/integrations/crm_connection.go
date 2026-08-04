package integrations

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/echayko/leadrula/backend/internal/integrations/providers"
	"github.com/echayko/leadrula/backend/internal/webhooks"
	"github.com/echayko/leadrula/backend/pkg/httpx"
)

type CRMConnectionDetail struct {
	Connection     Connection          `json:"connection"`
	InboundWebhook *InboundWebhookInfo `json:"inbound_webhook,omitempty"`
	WebhookIDs     webhooks.CRMWebhookIDs `json:"webhook_ids,omitempty"`
}

type CRMConnectionResponse struct {
	Connection
	InboundWebhook *InboundWebhookInfo `json:"inbound_webhook,omitempty"`
}

var crmConfigurableSlugs = map[string]bool{
	"pipedrive": true,
	"hubspot":   true,
	"zoho_crm":  true,
}

func CRMConnectionConfigurable(slug string) bool {
	return crmConfigurableSlugs[slug]
}

func (s *Service) UpdateCRMConnection(ctx context.Context, accountID, id int64, config map[string]any) (*Connection, error) {
	conn, err := s.GetConnection(ctx, accountID, id)
	if err != nil {
		return nil, err
	}
	if !CRMConnectionConfigurable(conn.ProviderSlug) {
		return nil, httpx.Validation("crm connection settings not supported for " + conn.ProviderSlug)
	}
	if config == nil {
		config = map[string]any{}
	}
	existing := configMap(conn.Config)
	for k, v := range existing {
		if _, ok := config[k]; !ok {
			config[k] = v
		}
	}
	if err := validateCRMConnectionConfig(conn.ProviderSlug, config); err != nil {
		return nil, httpx.Validation(err.Error())
	}
	configJSON, _ := json.Marshal(config)
	var updated Connection
	err = s.pool.QueryRow(ctx,
		`UPDATE integration_connections SET config=$2, updated_at=now() WHERE id=$1 AND account_id=$3
		 RETURNING id, public_id::text, account_id, name, config, status, created_at`,
		id, configJSON, accountID).Scan(
		&updated.ID, &updated.PublicID, &updated.AccountID, &updated.Name, &updated.Config, &updated.Status, &updated.CreatedAt)
	if err != nil {
		return nil, err
	}
	updated.ProviderSlug = conn.ProviderSlug
	updated.ProviderName = conn.ProviderName
	return &updated, nil
}

func validateCRMConnectionConfig(slug string, config map[string]any) error {
	if !boolFromConfig(config["inbound_stage_sync_enabled"]) {
		return nil
	}
	syncCfg := providers.ParseInboundStageSync(config)
	if syncCfg.LeadrulaPipelineID <= 0 {
		return fmt.Errorf("inbound_sync_leadrula_pipeline_id required when inbound stage sync is enabled")
	}
	if syncCfg.CRMPipelineID == "" {
		return fmt.Errorf("inbound_sync_crm_pipeline_id required when inbound stage sync is enabled")
	}
	if !providers.CRMInboundStageSyncReady(syncCfg) {
		return fmt.Errorf("configure stage mappings for the selected pipeline")
	}
	return nil
}

func boolFromConfig(v any) bool {
	switch x := v.(type) {
	case bool:
		return x
	case string:
		return x == "true" || x == "1"
	default:
		return false
	}
}

func (s *Service) CRMConnectionDetail(ctx context.Context, accountID, connectionID int64, apiBaseURL string) (*CRMConnectionDetail, error) {
	conn, err := s.GetConnection(ctx, accountID, connectionID)
	if err != nil {
		return nil, err
	}
	if !CRMConnectionConfigurable(conn.ProviderSlug) && conn.ProviderSlug != "ghl" {
		return nil, httpx.Validation("not a configurable crm connection")
	}
	detail := &CRMConnectionDetail{
		Connection: *conn,
		WebhookIDs: webhooks.ParseCRMWebhookIDs(conn.Config),
	}
	if conn.Status == "active" && detail.WebhookIDs.InboundSlug != "" {
		detail.InboundWebhook = BuildCRMInboundWebhookInfo(apiBaseURL, detail.WebhookIDs.Inbound, detail.WebhookIDs.InboundSlug, conn.ProviderSlug)
	}
	return detail, nil
}

func BuildCRMInboundWebhookInfo(apiBaseURL string, webhookID int64, slug, providerSlug string) *InboundWebhookInfo {
	info := BuildInboundWebhookInfo(apiBaseURL, webhookID, slug)
	if info == nil {
		return nil
	}
	switch providerSlug {
	case "pipedrive":
		info.SetupHint = "In Pipedrive, open Settings → Webhooks and add this URL for deal.updated events."
	case "hubspot":
		info.SetupHint = "In HubSpot, create a webhook subscription for deal property changes (dealstage) pointing to this URL."
	case "zoho_crm":
		info.SetupHint = "In Zoho CRM, configure a notification/webhook for Deal module stage changes to this URL."
	default:
		info.SetupHint = "Configure your CRM to send deal/opportunity stage change events to this URL."
	}
	return info
}

func crmResponse(conn *Connection, apiBaseURL string) CRMConnectionResponse {
	ids := webhooks.ParseCRMWebhookIDs(conn.Config)
	out := CRMConnectionResponse{Connection: *conn}
	if conn.Status == "active" && ids.InboundSlug != "" {
		out.InboundWebhook = BuildCRMInboundWebhookInfo(apiBaseURL, ids.Inbound, ids.InboundSlug, conn.ProviderSlug)
	}
	return out
}

func (s *Service) FinalizeCRMConnection(ctx context.Context, connectionID int64, config map[string]any) error {
	configJSON, _ := json.Marshal(config)
	_, err := s.pool.Exec(ctx,
		`UPDATE integration_connections SET config=$2, updated_at=now() WHERE id=$1`,
		connectionID, configJSON)
	return err
}

func (s *Service) GetConnectionAccount(ctx context.Context, connectionID int64) (accountID int64, publicID, name, providerSlug string, config map[string]any, err error) {
	var configJSON []byte
	err = s.pool.QueryRow(ctx,
		`SELECT c.account_id, c.public_id::text, c.name, p.slug, c.config
		 FROM integration_connections c
		 JOIN integration_providers p ON p.id = c.provider_id
		 WHERE c.id=$1`, connectionID).Scan(&accountID, &publicID, &name, &providerSlug, &configJSON)
	if err != nil {
		return 0, "", "", "", nil, err
	}
	return accountID, publicID, name, providerSlug, configMap(configJSON), nil
}
