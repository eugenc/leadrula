package integrations

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/echayko/leadrula/backend/internal/integrations/providers"
	"github.com/echayko/leadrula/backend/internal/webhooks"
	"github.com/echayko/leadrula/backend/pkg/httpx"
)

type GHLConnectionDetail struct {
	Connection                 Connection             `json:"connection"`
	InboundWebhook             *InboundWebhookInfo    `json:"inbound_webhook,omitempty"`
	WebhookIDs                 webhooks.GHLWebhookIDs `json:"webhook_ids,omitempty"`
	HasPrivateIntegrationToken bool                   `json:"has_private_integration_token"`
}

type GHLConnectionResponse struct {
	Connection
	InboundWebhook *InboundWebhookInfo `json:"inbound_webhook,omitempty"`
}

func (s *Service) UpdateGHLConnection(
	ctx context.Context,
	accountID, id int64,
	credentialsRaw json.RawMessage,
	config map[string]any,
) (*Connection, error) {
	conn, err := s.GetConnection(ctx, accountID, id)
	if err != nil {
		return nil, err
	}
	if conn.ProviderSlug != "ghl" {
		return nil, httpx.Validation("not a ghl connection")
	}
	if config == nil {
		config = map[string]any{}
	}
	existingCfg := configMap(conn.Config)
	for k, v := range existingCfg {
		if _, ok := config[k]; !ok {
			config[k] = v
		}
	}
	config = providers.MergeGHLConfigDefaults(config)
	if err := providers.ValidateGHLConfigJSON(config); err != nil {
		return nil, httpx.Validation(err.Error())
	}

	p := s.providers["ghl"]
	credsToValidate := credentialsRaw
	if credentialsRaw != nil && len(credentialsRaw) > 0 && string(credentialsRaw) != "null" {
		encrypted, err := encrypt(s.encKey, credentialsRaw)
		if err != nil {
			return nil, err
		}
		if _, err := s.pool.Exec(ctx,
			`UPDATE integration_connections SET credentials=$2, updated_at=now() WHERE id=$1`,
			id, encrypted); err != nil {
			return nil, err
		}
	} else {
		var encCreds []byte
		_ = s.pool.QueryRow(ctx, `SELECT credentials FROM integration_connections WHERE id=$1`, id).Scan(&encCreds)
		if len(encCreds) > 0 {
			credsToValidate, _ = decrypt(s.encKey, encCreds)
		}
	}
	if err := p.ValidateCredentials(ctx, credsToValidate, config); err != nil {
		return nil, httpx.Validation("credential validation failed: " + err.Error())
	}

	configJSON, _ := json.Marshal(config)
	var updated Connection
	err = s.pool.QueryRow(ctx,
		`UPDATE integration_connections SET config=$2, updated_at=now() WHERE id=$1
		 RETURNING id, public_id::text, account_id, name, config, status, created_at`,
		id, configJSON).Scan(
		&updated.ID, &updated.PublicID, &updated.AccountID, &updated.Name,
		&updated.Config, &updated.Status, &updated.CreatedAt)
	if err != nil {
		return nil, err
	}
	updated.ProviderSlug = conn.ProviderSlug
	updated.ProviderName = conn.ProviderName
	return &updated, nil
}

func (s *Service) FinalizeGHLConnection(ctx context.Context, connectionID int64, config map[string]any) error {
	configJSON, _ := json.Marshal(config)
	_, err := s.pool.Exec(ctx,
		`UPDATE integration_connections SET config=$2, updated_at=now() WHERE id=$1`,
		connectionID, configJSON)
	return err
}

func (s *Service) GHLMetadata(ctx context.Context, accountID, connectionID int64, kind string) (any, error) {
	conn, err := s.GetConnection(ctx, accountID, connectionID)
	if err != nil {
		return nil, err
	}
	if conn.ProviderSlug != "ghl" {
		return nil, httpx.Validation("not a ghl connection")
	}
	if providers.ParseGHLDeliveryModeFromConfig(configMap(conn.Config)) == "webhook" {
		return nil, httpx.Validation("metadata not available in webhook delivery mode")
	}
	cfg := configMap(conn.Config)
	locationID, _ := cfg["location_id"].(string)
	if locationID == "" {
		return nil, httpx.Validation("location_id not configured")
	}
	var encCreds []byte
	if err := s.pool.QueryRow(ctx, `SELECT credentials FROM integration_connections WHERE id=$1`, connectionID).Scan(&encCreds); err != nil {
		return nil, err
	}
	credentials, err := decrypt(s.encKey, encCreds)
	if err != nil {
		return nil, err
	}
	token, err := providers.ParseGHLCredentials(credentials)
	if err != nil {
		return nil, httpx.Validation(err.Error())
	}
	return s.ghlMetadataByKind(ctx, token, locationID, kind)
}

func (s *Service) GHLMetadataFromCredentials(ctx context.Context, credentialsRaw json.RawMessage, config map[string]any) (map[string]any, error) {
	if config == nil {
		config = map[string]any{}
	}
	locationID, _ := config["location_id"].(string)
	if locationID == "" {
		return nil, httpx.Validation("location_id is required")
	}
	token, err := providers.ParseGHLCredentials(credentialsRaw)
	if err != nil {
		return nil, httpx.Validation(err.Error())
	}
	pipelines, err := providers.FetchGHLPipelines(ctx, token, locationID)
	if err != nil {
		return nil, httpx.Validation(err.Error())
	}
	calendars, err := providers.FetchGHLCalendars(ctx, token, locationID)
	if err != nil {
		return nil, httpx.Validation(err.Error())
	}
	customFields, err := providers.FetchGHLCustomFields(ctx, token, locationID)
	if err != nil {
		return nil, httpx.Validation(err.Error())
	}
	return map[string]any{
		"pipelines":     pipelines,
		"calendars":     calendars,
		"custom_fields": providers.GHLCustomFieldsToResponse(customFields),
	}, nil
}

func (s *Service) ghlMetadataByKind(ctx context.Context, token, locationID, kind string) (any, error) {
	switch kind {
	case "pipelines":
		pipelines, err := providers.FetchGHLPipelines(ctx, token, locationID)
		if err != nil {
			return nil, httpx.Validation(err.Error())
		}
		return map[string]any{"pipelines": pipelines}, nil
	case "calendars":
		calendars, err := providers.FetchGHLCalendars(ctx, token, locationID)
		if err != nil {
			return nil, httpx.Validation(err.Error())
		}
		return map[string]any{"calendars": calendars}, nil
	case "custom_fields":
		customFields, err := providers.FetchGHLCustomFields(ctx, token, locationID)
		if err != nil {
			return nil, httpx.Validation(err.Error())
		}
		return map[string]any{"custom_fields": providers.GHLCustomFieldsToResponse(customFields)}, nil
	default:
		return nil, httpx.Validation("unknown metadata kind")
	}
}

func BuildGHLInboundWebhookInfo(apiBaseURL string, webhookID int64, slug string) *InboundWebhookInfo {
	info := BuildInboundWebhookInfo(apiBaseURL, webhookID, slug)
	if info != nil {
		info.SetupHint = "In GoHighLevel, add this URL to a Workflow triggered on Opportunity Stage Changed using the default webhook payload (no custom body required)."
	}
	return info
}

func GHLDetailFromConnection(conn *Connection, apiBaseURL string, hasPIT bool) *GHLConnectionDetail {
	ids := webhooks.ParseGHLWebhookIDs(conn.Config)
	detail := &GHLConnectionDetail{
		Connection:                 *conn,
		WebhookIDs:                 ids,
		HasPrivateIntegrationToken: hasPIT,
	}
	if conn.Status == "active" && ids.InboundSlug != "" {
		detail.InboundWebhook = BuildGHLInboundWebhookInfo(apiBaseURL, ids.Inbound, ids.InboundSlug)
	}
	return detail
}

func ghlCredentialsHavePIT(encKey, encCreds []byte) bool {
	if len(encCreds) == 0 {
		return false
	}
	plain, err := decrypt(encKey, encCreds)
	if err != nil {
		return false
	}
	_, err = providers.ParseGHLCredentials(plain)
	return err == nil
}

func (s *Service) GHLHasPrivateIntegrationToken(ctx context.Context, connectionID int64) bool {
	var encCreds []byte
	if err := s.pool.QueryRow(ctx,
		`SELECT credentials FROM integration_connections WHERE id=$1`, connectionID,
	).Scan(&encCreds); err != nil {
		return false
	}
	return ghlCredentialsHavePIT(s.encKey, encCreds)
}

func (s *Service) GHLConnectionDetail(ctx context.Context, accountID, connectionID int64, apiBaseURL string) (*GHLConnectionDetail, error) {
	conn, err := s.GetConnection(ctx, accountID, connectionID)
	if err != nil {
		return nil, err
	}
	if conn.ProviderSlug != "ghl" {
		return nil, httpx.Validation("not a ghl connection")
	}
	hasPIT := s.GHLHasPrivateIntegrationToken(ctx, connectionID)
	return GHLDetailFromConnection(conn, apiBaseURL, hasPIT), nil
}

func (s *Service) TestStoredConnection(ctx context.Context, accountID, connectionID int64) error {
	conn, err := s.GetConnection(ctx, accountID, connectionID)
	if err != nil {
		return err
	}
	var encCreds []byte
	if err := s.pool.QueryRow(ctx,
		`SELECT credentials FROM integration_connections WHERE id=$1`, connectionID,
	).Scan(&encCreds); err != nil {
		return err
	}
	credentials, err := decrypt(s.encKey, encCreds)
	if err != nil {
		return err
	}
	config := configMap(conn.Config)
	switch conn.ProviderSlug {
	case "sunbase":
		return (&providers.SunbaseProvider{}).TestConnection(ctx, credentials, config)
	case "ghl":
		config = providers.MergeGHLConfigDefaults(config)
		return (&providers.GHLProvider{}).TestConnection(ctx, credentials, config)
	default:
		return httpx.Validation("test connection not supported for " + conn.ProviderSlug)
	}
}

func wrapGHLProvisionErr(step string, err error) error {
	if err == nil {
		return nil
	}
	var appErr *httpx.AppError
	if errors.As(err, &appErr) {
		return appErr
	}
	return httpx.ServiceUnavailable("failed to set up GHL webhooks")
}
