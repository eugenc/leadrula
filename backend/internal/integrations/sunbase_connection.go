package integrations

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/echayko/leadrula/backend/internal/integrations/providers"
	"github.com/echayko/leadrula/backend/internal/webhooks"
	"github.com/echayko/leadrula/backend/pkg/httpx"
	"github.com/jackc/pgx/v5"
)

type InboundWebhookInfo struct {
	ID             int64   `json:"id"`
	Slug           string  `json:"slug"`
	Endpoint       string  `json:"endpoint"`
	Secret         *string `json:"secret,omitempty"`
	SecretRequired bool    `json:"secret_required"`
	SetupHint      string  `json:"setup_hint"`
}

type SunbaseConnectionDetail struct {
	Connection     Connection          `json:"connection"`
	InboundWebhook *InboundWebhookInfo `json:"inbound_webhook,omitempty"`
	WebhookIDs     webhooks.SunbaseWebhookIDs `json:"webhook_ids,omitempty"`
}

type SunbaseConnectionResponse struct {
	Connection
	InboundWebhook *InboundWebhookInfo `json:"inbound_webhook,omitempty"`
}

func (s *Service) GetConnection(ctx context.Context, accountID, id int64) (*Connection, error) {
	var conn Connection
	err := s.pool.QueryRow(ctx,
		`SELECT c.id, c.public_id::text, c.account_id, p.slug, p.name, c.name,
		        c.config, c.status, c.last_error, c.last_used_at, c.created_at
		 FROM integration_connections c
		 JOIN integration_providers p ON p.id = c.provider_id
		 WHERE c.id = $1 AND c.account_id = $2`, id, accountID).Scan(
		&conn.ID, &conn.PublicID, &conn.AccountID,
		&conn.ProviderSlug, &conn.ProviderName, &conn.Name,
		&conn.Config, &conn.Status, &conn.LastError,
		&conn.LastUsedAt, &conn.CreatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, httpx.NotFound("connection not found")
		}
		return nil, err
	}
	return &conn, nil
}

func (s *Service) UpdateSunbaseConnection(
	ctx context.Context,
	accountID, id int64,
	credentialsRaw json.RawMessage,
	config map[string]any,
	syncOutbound func(ctx context.Context, ids webhooks.SunbaseWebhookIDs, endpointURL string, fieldMap json.RawMessage) error,
) (*Connection, error) {
	conn, err := s.GetConnection(ctx, accountID, id)
	if err != nil {
		return nil, err
	}
	if conn.ProviderSlug != "sunbase" {
		return nil, httpx.Validation("not a sunbase connection")
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

	p := s.providers["sunbase"]
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

	endpointURL := sunbaseEndpointFromConfig(config)
	fieldMapJSON, _ := json.Marshal(config["outbound_field_map"])
	ids := webhooks.ParseSunbaseWebhookIDs(config)
	if syncOutbound != nil && (ids.OutboundPost > 0 || ids.OutboundGet > 0) {
		if err := syncOutbound(ctx, ids, endpointURL, fieldMapJSON); err != nil {
			return nil, err
		}
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

func (s *Service) TestConnection(ctx context.Context, providerSlug string, credentialsRaw json.RawMessage, config map[string]any) error {
	if providerSlug != "sunbase" {
		return httpx.Validation("test connection only supported for sunbase")
	}
	if _, ok := s.providers[providerSlug]; !ok {
		return httpx.Validation("unknown provider: " + providerSlug)
	}
	return (&providers.SunbaseProvider{}).TestConnection(ctx, credentialsRaw, config)
}

func (s *Service) FinalizeSunbaseConnection(ctx context.Context, connectionID int64, config map[string]any) error {
	configJSON, _ := json.Marshal(config)
	_, err := s.pool.Exec(ctx,
		`UPDATE integration_connections SET config=$2, updated_at=now() WHERE id=$1`,
		connectionID, configJSON)
	return err
}

func BuildInboundWebhookInfo(apiBaseURL string, webhookID int64, slug string) *InboundWebhookInfo {
	apiBaseURL = strings.TrimRight(apiBaseURL, "/")
	return &InboundWebhookInfo{
		ID:             webhookID,
		Slug:           slug,
		Endpoint:       fmt.Sprintf("POST %s/api/v1/webhooks/%s", apiBaseURL, slug),
		SecretRequired: false,
		SetupHint:      "Configure SunBase or Zapier to POST lead updates to this URL.",
	}
}

func SunbaseDetailFromConnection(conn *Connection, apiBaseURL string) *SunbaseConnectionDetail {
	ids := webhooks.ParseSunbaseWebhookIDs(conn.Config)
	detail := &SunbaseConnectionDetail{
		Connection: *conn,
		WebhookIDs: ids,
	}
	if conn.Status == "active" && ids.InboundSlug != "" {
		detail.InboundWebhook = BuildInboundWebhookInfo(apiBaseURL, ids.Inbound, ids.InboundSlug)
	}
	return detail
}

func configMap(config any) map[string]any {
	out := map[string]any{}
	if config == nil {
		return out
	}
	b, err := json.Marshal(config)
	if err != nil {
		return out
	}
	_ = json.Unmarshal(b, &out)
	return out
}

func sunbaseEndpointFromConfig(config map[string]any) string {
	if u, ok := config["endpoint_url"].(string); ok && strings.TrimSpace(u) != "" {
		return strings.TrimSpace(u)
	}
	return providers.DefaultSunbaseEndpoint
}

func (s *Service) ResolveSunbaseConnectionName(ctx context.Context, accountID int64, name string) string {
	name = strings.TrimSpace(name)
	if name != "" {
		return name
	}
	const base = "SunBase"
	for i := range 20 {
		candidate := base
		if i > 0 {
			candidate = fmt.Sprintf("%s %d", base, i+1)
		}
		var exists bool
		_ = s.pool.QueryRow(ctx,
			`SELECT EXISTS(
				SELECT 1 FROM integration_connections c
				JOIN integration_providers p ON p.id = c.provider_id
				WHERE c.account_id = $1 AND p.slug = 'sunbase' AND c.name = $2
			)`, accountID, candidate).Scan(&exists)
		if !exists {
			return candidate
		}
	}
	return base
}

func schemaNameFromCredentials(credentialsRaw json.RawMessage, config map[string]any) string {
	var creds struct {
		SchemaName string `json:"schema_name"`
	}
	_ = json.Unmarshal(credentialsRaw, &creds)
	if creds.SchemaName != "" {
		return creds.SchemaName
	}
	if s, ok := config["schema_name"].(string); ok {
		return s
	}
	return ""
}
