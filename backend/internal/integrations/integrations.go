package integrations

import (
	"context"
	"encoding/json"
	"errors"
	"maps"
	"time"

	"github.com/echayko/leadrula/backend/internal/integrations/providers"
	"github.com/echayko/leadrula/backend/internal/leads"
	"github.com/echayko/leadrula/backend/internal/webhooks"
	"github.com/echayko/leadrula/backend/pkg/httpx"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Connection struct {
	ID           int64      `json:"id"`
	PublicID     string     `json:"public_id"`
	AccountID    int64      `json:"-"`
	ProviderSlug string     `json:"provider_slug"`
	ProviderName string     `json:"provider_name"`
	Name         string     `json:"name"`
	Config       any        `json:"config"`
	Status       string     `json:"status"`
	LastError    *string    `json:"last_error,omitempty"`
	LastUsedAt   *time.Time `json:"last_used_at,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
}

type RouteIntegration struct {
	ID             int64  `json:"id"`
	RouteID        int64  `json:"route_id"`
	BranchPosition int    `json:"branch_position"`
	ConnectionID   int64  `json:"connection_id"`
	ConnectionName string `json:"connection_name"`
	ProviderSlug   string `json:"provider_slug"`
	DeliveryConfig any    `json:"delivery_config"`
	IsActive       bool   `json:"is_active"`
}

type Service struct {
	pool      *pgxpool.Pool
	encKey    []byte
	oauth     OAuthConfig
	providers map[string]providers.Provider
}

func NewService(pool *pgxpool.Pool, encKey []byte, oauth OAuthConfig) *Service {
	return &Service{
		pool:   pool,
		encKey: encKey,
		oauth:  oauth,
		providers: map[string]providers.Provider{
			"webhook":    &providers.WebhookProvider{},
			"ghl":        &providers.GHLProvider{},
			"pipedrive":  &providers.PipedriveProvider{},
			"hubspot":    &providers.HubSpotProvider{},
			"zoho_crm":   &providers.ZohoCRMProvider{},
			"salesforce": &providers.SalesforceProvider{},
			"sunbase":      &providers.SunbaseProvider{},
			"google_maps":  &providers.GoogleMapsProvider{},
		},
	}
}

func (s *Service) ListProviders(ctx context.Context) ([]map[string]any, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT slug, name, description, auth_type, direction, config_schema
		 FROM integration_providers WHERE is_active
		 ORDER BY CASE slug
		   WHEN 'pipedrive' THEN 1 WHEN 'ghl' THEN 2
		   WHEN 'hubspot' THEN 3 WHEN 'salesforce' THEN 4 WHEN 'sunbase' THEN 5
		   ELSE 99 END, name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []map[string]any
	for rows.Next() {
		var slug, name, desc, authType, direction string
		var configSchema json.RawMessage
		if err := rows.Scan(&slug, &name, &desc, &authType, &direction, &configSchema); err != nil {
			return nil, err
		}
		out = append(out, map[string]any{
			"slug": slug, "name": name, "description": desc,
			"auth_type": authType, "direction": direction, "config_schema": configSchema,
		})
	}
	return out, rows.Err()
}

func (s *Service) ListConnections(ctx context.Context, accountID int64) ([]Connection, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT c.id, c.public_id::text, c.account_id, p.slug, p.name, c.name,
		        c.config, c.status, c.last_error, c.last_used_at, c.created_at
		 FROM integration_connections c
		 JOIN integration_providers p ON p.id = c.provider_id
		 WHERE c.account_id = $1 ORDER BY c.name`, accountID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Connection
	for rows.Next() {
		var conn Connection
		if err := rows.Scan(&conn.ID, &conn.PublicID, &conn.AccountID,
			&conn.ProviderSlug, &conn.ProviderName, &conn.Name,
			&conn.Config, &conn.Status, &conn.LastError,
			&conn.LastUsedAt, &conn.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, conn)
	}
	return out, rows.Err()
}

func (s *Service) CreateConnection(ctx context.Context, accountID int64, providerSlug, name string, credentialsRaw json.RawMessage, config map[string]any) (*Connection, error) {
	if config == nil {
		config = map[string]any{}
	}
	p, ok := s.providers[providerSlug]
	if !ok {
		return nil, httpx.Validation("unknown provider: " + providerSlug)
	}
	var authType string
	if err := s.pool.QueryRow(ctx,
		`SELECT auth_type FROM integration_providers WHERE slug = $1`, providerSlug).Scan(&authType); err != nil {
		return nil, httpx.NotFound("provider not found")
	}
	if authType == "oauth2" {
		return nil, httpx.Validation("use oauth connect flow for " + providerSlug)
	}
	if err := p.ValidateCredentials(ctx, credentialsRaw, config); err != nil {
		return nil, httpx.Validation("credential validation failed: " + err.Error())
	}
	if providerSlug == googleMapsProviderSlug {
		var exists bool
		if err := s.pool.QueryRow(ctx,
			`SELECT EXISTS(
				SELECT 1 FROM integration_connections c
				JOIN integration_providers p ON p.id = c.provider_id
				WHERE c.account_id = $1 AND p.slug = $2
			)`, accountID, googleMapsProviderSlug).Scan(&exists); err != nil {
			return nil, err
		}
		if exists {
			return nil, httpx.Validation("disconnect existing google maps connection first")
		}
	}
	var providerID int64
	if err := s.pool.QueryRow(ctx,
		`SELECT id FROM integration_providers WHERE slug = $1`, providerSlug).Scan(&providerID); err != nil {
		return nil, httpx.NotFound("provider not found")
	}
	encrypted, err := encrypt(s.encKey, credentialsRaw)
	if err != nil {
		return nil, err
	}
	configJSON, _ := json.Marshal(config)
	var conn Connection
	err = s.pool.QueryRow(ctx,
		`INSERT INTO integration_connections (account_id, provider_id, name, credentials, config)
		 VALUES ($1, $2, $3, $4, $5)
		 RETURNING id, public_id::text, account_id, name, config, status, created_at`,
		accountID, providerID, name, encrypted, configJSON).Scan(
		&conn.ID, &conn.PublicID, &conn.AccountID, &conn.Name,
		&conn.Config, &conn.Status, &conn.CreatedAt)
	if err != nil {
		return nil, err
	}
	conn.ProviderSlug = providerSlug
	return &conn, nil
}

func (s *Service) DeleteConnection(ctx context.Context, accountID, id int64) error {
	ct, err := s.pool.Exec(ctx,
		`DELETE FROM integration_connections WHERE id = $1 AND account_id = $2`, id, accountID)
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return httpx.NotFound("connection not found")
	}
	return nil
}

func (s *Service) canAccessRoute(ctx context.Context, accountID int64, accountType string, routeID int64) (bool, error) {
	var ok bool
	switch accountType {
	case "publisher":
		err := s.pool.QueryRow(ctx,
			`SELECT EXISTS(SELECT 1 FROM routes WHERE id = $1 AND publisher_id = $2)`, routeID, accountID).Scan(&ok)
		return ok, err
	case "buyer":
		err := s.pool.QueryRow(ctx,
			`SELECT EXISTS(
				SELECT 1 FROM routes r
				LEFT JOIN contracts c ON c.id = r.contract_id AND c.deleted_at IS NULL
				WHERE r.id = $1 AND (
					r.buyer_id = $2
					OR (r.destination = 'contract' AND c.buyer_id = $2)
				))`,
			routeID, accountID).Scan(&ok)
		return ok, err
	default:
		return false, nil
	}
}

func (s *Service) AttachToRoute(ctx context.Context, accountID int64, accountType string, routeID, connectionID int64, branchPosition int, deliveryConfig map[string]any) error {
	ok, err := s.canAccessRoute(ctx, accountID, accountType, routeID)
	if err != nil {
		return err
	}
	if !ok {
		return httpx.NotFound("route not found")
	}
	var exists bool
	if err := s.pool.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM integration_connections WHERE id = $1 AND account_id = $2)`,
		connectionID, accountID).Scan(&exists); err != nil {
		return err
	}
	if !exists {
		return httpx.NotFound("connection not found")
	}
	var providerSlug string
	if err := s.pool.QueryRow(ctx,
		`SELECT p.slug FROM integration_connections c
		 JOIN integration_providers p ON p.id = c.provider_id
		 WHERE c.id = $1 AND c.account_id = $2`, connectionID, accountID).Scan(&providerSlug); err != nil {
		return err
	}
	if providerSlug == googleMapsProviderSlug {
		return httpx.Validation("google maps cannot be attached to routes")
	}
	if deliveryConfig == nil {
		deliveryConfig = map[string]any{}
	}
	configJSON, _ := json.Marshal(deliveryConfig)
	_, err = s.pool.Exec(ctx,
		`INSERT INTO route_integrations (route_id, connection_id, branch_position, delivery_config)
		 VALUES ($1, $2, $3, $4)
		 ON CONFLICT (route_id, connection_id, branch_position) DO UPDATE
		   SET delivery_config = EXCLUDED.delivery_config, is_active = true`,
		routeID, connectionID, branchPosition, configJSON)
	return err
}

func (s *Service) DetachFromRoute(ctx context.Context, accountID int64, accountType string, routeIntegrationID int64) error {
	var routeID int64
	err := s.pool.QueryRow(ctx,
		`SELECT route_id FROM route_integrations WHERE id = $1`, routeIntegrationID).Scan(&routeID)
	if err != nil {
		return httpx.NotFound("route integration not found")
	}
	ok, err := s.canAccessRoute(ctx, accountID, accountType, routeID)
	if err != nil {
		return err
	}
	if !ok {
		return httpx.NotFound("route integration not found")
	}
	ct, err := s.pool.Exec(ctx, `DELETE FROM route_integrations WHERE id = $1`, routeIntegrationID)
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return httpx.NotFound("route integration not found")
	}
	return nil
}

func (s *Service) ListRouteIntegrations(ctx context.Context, accountID int64, accountType string, routeID int64) ([]RouteIntegration, error) {
	ok, err := s.canAccessRoute(ctx, accountID, accountType, routeID)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, httpx.NotFound("route not found")
	}
	rows, err := s.pool.Query(ctx,
		`SELECT ri.id, ri.route_id, ri.branch_position, ri.connection_id, c.name, p.slug, ri.delivery_config, ri.is_active
		 FROM route_integrations ri
		 JOIN integration_connections c ON c.id = ri.connection_id
		 JOIN integration_providers p ON p.id = c.provider_id
		 WHERE ri.route_id = $1`, routeID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []RouteIntegration
	for rows.Next() {
		var ri RouteIntegration
		if err := rows.Scan(&ri.ID, &ri.RouteID, &ri.BranchPosition, &ri.ConnectionID, &ri.ConnectionName,
			&ri.ProviderSlug, &ri.DeliveryConfig, &ri.IsActive); err != nil {
			return nil, err
		}
		out = append(out, ri)
	}
	return out, rows.Err()
}

func (s *Service) EnqueueDelivery(ctx context.Context, routeID, leadID int64, branchPosition int, payloadJSON []byte, enqueued *[]int64) error {
	rows, err := s.pool.Query(ctx,
		`SELECT ri.connection_id, ri.delivery_config, c.config, p.slug
		 FROM route_integrations ri
		 JOIN integration_connections c ON c.id = ri.connection_id
		 JOIN integration_providers p ON p.id = c.provider_id
		 WHERE ri.route_id = $1 AND ri.branch_position = $2 AND ri.is_active`, routeID, branchPosition)
	if err != nil {
		return err
	}
	defer rows.Close()
	var payload map[string]any
	_ = json.Unmarshal(payloadJSON, &payload)
	pipelineID, stageID := payloadPipelineStage(payload)
	for rows.Next() {
		var connID int64
		var deliveryConfig, connConfig json.RawMessage
		var providerSlug string
		if err := rows.Scan(&connID, &deliveryConfig, &connConfig, &providerSlug); err != nil {
			return err
		}
		if !ghlWebhookShouldEnqueue(providerSlug, connConfig, pipelineID, stageID) {
			continue
		}
		p := maps.Clone(payload)
		var dc map[string]any
		_ = json.Unmarshal(deliveryConfig, &dc)
		p["_config"] = dc
		merged, _ := json.Marshal(p)
		if _, err := s.pool.Exec(ctx,
			`INSERT INTO integration_delivery_queue (lead_id, connection_id, route_id, payload)
			 VALUES ($1, $2, $3, $4)`,
			leadID, connID, routeID, merged); err != nil {
			return err
		}
		trackEnqueuedConn(enqueued, connID)
	}
	return rows.Err()
}

// EnqueueConnectionDelivery enqueues CRM delivery for a participation integration.
func (s *Service) EnqueueConnectionDelivery(ctx context.Context, connectionID, leadID int64, payloadJSON []byte, enqueued *[]int64) error {
	var providerSlug string
	var connConfig json.RawMessage
	err := s.pool.QueryRow(ctx,
		`SELECT p.slug, c.config
		 FROM integration_connections c
		 JOIN integration_providers p ON p.id = c.provider_id
		 WHERE c.id = $1`, connectionID).Scan(&providerSlug, &connConfig)
	if err != nil {
		return err
	}
	var payload map[string]any
	_ = json.Unmarshal(payloadJSON, &payload)
	pipelineID, stageID := payloadPipelineStage(payload)
	if !ghlWebhookShouldEnqueue(providerSlug, connConfig, pipelineID, stageID) {
		return nil
	}
	if _, err := s.pool.Exec(ctx,
		`INSERT INTO integration_delivery_queue (lead_id, connection_id, payload)
		 VALUES ($1, $2, $3)`,
		leadID, connectionID, payloadJSON); err != nil {
		return err
	}
	trackEnqueuedConn(enqueued, connectionID)
	return nil
}

// TryEnqueueGHLWebhookOnStageMove enqueues GHL webhook deliveries when a lead enters a configured trigger stage.
func (s *Service) TryEnqueueGHLWebhookOnStageMove(ctx context.Context, ownerAccountID, pipelineID, stageID, leadID int64, payloadJSON []byte, skipConnIDs []int64) error {
	if pipelineID == 0 || stageID == 0 {
		return nil
	}
	skip := connIDSet(skipConnIDs)
	rows, err := s.pool.Query(ctx,
		`SELECT c.id, c.config
		 FROM integration_connections c
		 JOIN integration_providers p ON p.id = c.provider_id
		 WHERE c.account_id = $1 AND c.status = 'active' AND p.slug = 'ghl'
		   AND COALESCE(c.config->>'delivery_mode', 'api') = 'webhook'`, ownerAccountID)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var connID int64
		var connConfig json.RawMessage
		if err := rows.Scan(&connID, &connConfig); err != nil {
			return err
		}
		if skip[connID] {
			continue
		}
		cfg, err := ghlConfigFromJSON(connConfig)
		if err != nil {
			continue
		}
		if !providers.MatchesGHLWebhookTrigger(cfg.PipelineStageMap, pipelineID, stageID) {
			continue
		}
		if _, err := s.pool.Exec(ctx,
			`INSERT INTO integration_delivery_queue (lead_id, connection_id, payload)
			 VALUES ($1, $2, $3)`,
			leadID, connID, payloadJSON); err != nil {
			return err
		}
	}
	return rows.Err()
}

func payloadPipelineStage(payload map[string]any) (pipelineID, stageID int64) {
	if payload == nil {
		return 0, 0
	}
	switch v := payload["pipeline_id"].(type) {
	case float64:
		pipelineID = int64(v)
	case int64:
		pipelineID = v
	}
	switch v := payload["stage_id"].(type) {
	case float64:
		stageID = int64(v)
	case int64:
		stageID = v
	}
	return pipelineID, stageID
}

func ghlConfigFromJSON(raw json.RawMessage) (providers.GHLConfig, error) {
	cfg := map[string]any{}
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &cfg)
	}
	return providers.ParseGHLConfig(providers.MergeGHLConfigDefaults(cfg))
}

func ghlWebhookShouldEnqueue(providerSlug string, connConfig json.RawMessage, pipelineID, stageID int64) bool {
	if providerSlug != "ghl" {
		return true
	}
	cfg, err := ghlConfigFromJSON(connConfig)
	if err != nil || cfg.DeliveryMode != "webhook" {
		return true
	}
	return providers.MatchesGHLWebhookTrigger(cfg.PipelineStageMap, pipelineID, stageID)
}

func trackEnqueuedConn(enqueued *[]int64, connID int64) {
	if enqueued != nil {
		*enqueued = append(*enqueued, connID)
	}
}

func connIDSet(ids []int64) map[int64]bool {
	out := make(map[int64]bool, len(ids))
	for _, id := range ids {
		out[id] = true
	}
	return out
}

// EnqueueParticipationWebhook fires the first active lead.create trigger on a buyer webhook.
func (s *Service) EnqueueParticipationWebhook(ctx context.Context, webhookID, leadID int64, payloadJSON []byte) error {
	var triggerID, connID, accountID int64
	var fieldMap json.RawMessage
	var format string
	err := s.pool.QueryRow(ctx,
		`SELECT t.id, COALESCE(w.outbound_connection_id, 0), w.outbound_field_map, COALESCE(NULLIF(w.outbound_format, ''), 'json'), w.account_id
		 FROM webhook_outbound_triggers t
		 JOIN webhooks w ON w.id = t.webhook_id
		 WHERE t.webhook_id = $1 AND t.trigger_event = 'lead.create' AND t.is_active AND w.is_active
		 ORDER BY t.position, t.id LIMIT 1`, webhookID).Scan(&triggerID, &connID, &fieldMap, &format, &accountID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		return err
	}
	if connID == 0 {
		return nil
	}
	if format == "url" && len(fieldMap) > 0 && leadID != 0 {
		repo := leads.NewRepository(s.pool)
		lead, err := repo.GetByID(ctx, s.pool, leadID)
		if err != nil {
			return err
		}
		if err := leads.LoadCustomValues(ctx, s.pool, lead); err != nil {
			return err
		}
		mapped, err := webhooks.BuildOutboundURLPayload(ctx, s.pool, accountID, webhooks.EventLeadCreate, lead, fieldMap)
		if err != nil {
			return err
		}
		payloadJSON = mapped
	}
	return s.EnqueueWebhookDelivery(ctx, connID, triggerID, leadID, payloadJSON)
}

// EnqueueWebhookDelivery enqueues a pre-rendered payload for an outbound webhook trigger.
// Unlike EnqueueDelivery (which uses route integrations), this targets a specific connection
// directly and sets webhook_trigger_id so delivery logs can distinguish the source.
func (s *Service) EnqueueWebhookDelivery(ctx context.Context, connectionID, triggerID, leadID int64, payload []byte) error {
	var nullLeadID *int64
	if leadID != 0 {
		nullLeadID = &leadID
	}
	_, err := s.pool.Exec(ctx,
		`INSERT INTO integration_delivery_queue (lead_id, connection_id, payload, webhook_trigger_id)
		 VALUES ($1, $2, $3, $4)`,
		nullLeadID, connectionID, payload, triggerID)
	return err
}

func (s *Service) Provider(slug string) (providers.Provider, bool) {
	p, ok := s.providers[slug]
	return p, ok
}

func (s *Service) Pool() *pgxpool.Pool { return s.pool }

func (s *Service) EncKey() []byte { return s.encKey }

func (s *Service) OAuth() OAuthConfig { return s.oauth }
