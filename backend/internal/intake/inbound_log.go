package intake

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

type InboundLogItem struct {
	Kind           string          `json:"kind"`
	Direction      string          `json:"direction"`
	ID             int64           `json:"id"`
	CreatedAt      time.Time       `json:"created_at"`
	Origin         string          `json:"origin"`
	OriginSlug     string          `json:"origin_slug"`
	LeadLabel      string          `json:"lead_label"`
	LeadID         *int64          `json:"lead_id,omitempty"`
	Status         string          `json:"status"`
	UnmappedKeys   []string        `json:"unmapped_keys,omitempty"`
	FirstName      string          `json:"first_name,omitempty"`
	LastName       string          `json:"last_name,omitempty"`
	Phone          *string         `json:"phone,omitempty"`
	Source         *string         `json:"source,omitempty"`
	RawPayload     json.RawMessage `json:"raw_payload,omitempty"`
	WebhookID      int64           `json:"webhook_id,omitempty"`
	ErrorMessage   *string         `json:"error_message,omitempty"`
	ProviderSlug   string          `json:"provider_slug,omitempty"`
	ConnectionName string          `json:"connection_name,omitempty"`
	Attempts       int             `json:"attempts,omitempty"`
}

type InboundLogListResponse struct {
	Items []InboundLogItem `json:"items"`
	Total int64            `json:"total"`
	Page  int              `json:"page"`
	Limit int              `json:"limit"`
}

type ListInboundLogParams struct {
	Type      string
	Status    string
	Search    string
	Source    string
	WebhookID int64
	Page      int
	Limit     int
}

func (s *Service) ListInboundLog(ctx context.Context, accountID int64, p ListInboundLogParams) (*InboundLogListResponse, error) {
	switch p.Type {
	case "source":
		return s.listInboundLogSources(ctx, accountID, p)
	case "webhook":
		return s.listInboundLogWebhooks(ctx, accountID, p)
	case "integration":
		return s.listInboundLogIntegrations(ctx, accountID, p)
	default:
		return s.listInboundLogAll(ctx, accountID, p)
	}
}

func (s *Service) listInboundLogSources(ctx context.Context, accountID int64, p ListInboundLogParams) (*InboundLogListResponse, error) {
	q, err := s.ListQueue(ctx, accountID, ListQueueParams{
		Status: firstOr(p.Status, "all"),
		Page:   p.Page,
		Limit:  p.Limit,
		Search: p.Search,
		Source: p.Source,
	})
	if err != nil {
		return nil, err
	}
	items := make([]InboundLogItem, 0, len(q.Items))
	for _, it := range q.Items {
		items = append(items, queueItemToInbound(it))
	}
	return &InboundLogListResponse{Items: items, Total: q.Total, Page: q.Page, Limit: q.Limit}, nil
}

func queueItemToInbound(it QueueItem) InboundLogItem {
	origin := ""
	originSlug := ""
	if it.Source != nil {
		origin = *it.Source
		originSlug = *it.Source
	}
	leadID := it.LeadID
	return InboundLogItem{
		Kind:         "source",
		Direction:    "inbound",
		ID:           it.ID,
		CreatedAt:    it.CreatedAt,
		Origin:       origin,
		OriginSlug:   originSlug,
		LeadLabel:    fmt.Sprintf("%s %s", it.FirstName, it.LastName),
		LeadID:       &leadID,
		Status:       it.Status,
		UnmappedKeys: it.UnmappedKeys,
		FirstName:    it.FirstName,
		LastName:     it.LastName,
		Phone:        it.Phone,
		Source:       it.Source,
		RawPayload:   it.RawPayload,
	}
}

func (s *Service) listInboundLogWebhooks(ctx context.Context, accountID int64, p ListInboundLogParams) (*InboundLogListResponse, error) {
	limit := p.Limit
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	page := p.Page
	if page < 1 {
		page = 1
	}
	offset := (page - 1) * limit
	status := p.Status
	webhookID := p.WebhookID

	var total int64
	if err := s.pool.QueryRow(ctx,
		`SELECT COUNT(*)
		 FROM webhook_deliveries d
		 JOIN webhooks w ON w.id = d.webhook_id
		 WHERE w.account_id = $1
		   AND ($2 = '' OR d.status = $2::webhook_delivery_status)
		   AND ($3 = 0 OR d.webhook_id = $3)`,
		accountID, status, webhookID).Scan(&total); err != nil {
		return nil, err
	}

	rows, err := s.pool.Query(ctx,
		`SELECT d.id, d.webhook_id, w.name, w.slug, d.lead_id, l.public_id::text,
		        d.status, d.error_message, d.created_at
		 FROM webhook_deliveries d
		 JOIN webhooks w ON w.id = d.webhook_id
		 LEFT JOIN leads l ON l.id = d.lead_id
		 WHERE w.account_id = $1
		   AND ($2 = '' OR d.status = $2::webhook_delivery_status)
		   AND ($3 = 0 OR d.webhook_id = $3)
		 ORDER BY d.created_at DESC
		 LIMIT $4 OFFSET $5`,
		accountID, status, webhookID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []InboundLogItem
	for rows.Next() {
		var d InboundLogItem
		var leadPublicID *string
		if err := rows.Scan(
			&d.ID, &d.WebhookID, &d.Origin, &d.OriginSlug,
			&d.LeadID, &leadPublicID, &d.Status, &d.ErrorMessage, &d.CreatedAt,
		); err != nil {
			return nil, err
		}
		d.Kind = "webhook"
		d.Direction = "inbound"
		if leadPublicID != nil {
			d.LeadLabel = *leadPublicID
		}
		items = append(items, d)
	}
	if items == nil {
		items = []InboundLogItem{}
	}
	return &InboundLogListResponse{Items: items, Total: total, Page: page, Limit: limit}, rows.Err()
}

func (s *Service) listInboundLogIntegrations(ctx context.Context, accountID int64, p ListInboundLogParams) (*InboundLogListResponse, error) {
	limit := p.Limit
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	page := p.Page
	if page < 1 {
		page = 1
	}
	offset := (page - 1) * limit
	status := p.Status

	var total int64
	if err := s.pool.QueryRow(ctx,
		`SELECT
		   (SELECT COUNT(*)
		    FROM integration_delivery_queue q
		    JOIN integration_connections c ON c.id = q.connection_id
		    WHERE c.account_id = $1
		      AND q.webhook_trigger_id IS NULL
		      AND ($2 = '' OR q.status = $2::delivery_status))
		 + (SELECT COUNT(*)
		    FROM webhook_deliveries d
		    JOIN webhooks w ON w.id = d.webhook_id
		    JOIN integration_connections c ON c.id = w.integration_connection_id
		    WHERE c.account_id = $1
		      AND ($2 = '' OR d.status = $2::webhook_delivery_status))`,
		accountID, status).Scan(&total); err != nil {
		return nil, err
	}

	rows, err := s.pool.Query(ctx,
		`SELECT kind, direction, id, created_at, origin, origin_slug, lead_label, lead_id, status,
		        webhook_id, error_message, provider_slug, connection_name, attempts
		 FROM (
		   SELECT
		     'integration'::text AS kind,
		     'outbound'::text AS direction,
		     q.id,
		     q.created_at,
		     c.name AS origin,
		     p.slug AS origin_slug,
		     COALESCE(l.public_id::text, '') AS lead_label,
		     q.lead_id,
		     q.status::text AS status,
		     0::bigint AS webhook_id,
		     q.last_error AS error_message,
		     p.slug AS provider_slug,
		     c.name AS connection_name,
		     q.attempts
		   FROM integration_delivery_queue q
		   JOIN integration_connections c ON c.id = q.connection_id
		   JOIN integration_providers p ON p.id = c.provider_id
		   LEFT JOIN leads l ON l.id = q.lead_id
		   WHERE c.account_id = $1
		     AND q.webhook_trigger_id IS NULL
		     AND ($2 = '' OR q.status = $2::delivery_status)

		   UNION ALL

		   SELECT
		     'webhook'::text AS kind,
		     'inbound'::text AS direction,
		     d.id,
		     d.created_at,
		     c.name AS origin,
		     p.slug AS origin_slug,
		     COALESCE(l.public_id::text, '') AS lead_label,
		     d.lead_id,
		     d.status::text AS status,
		     d.webhook_id,
		     d.error_message,
		     p.slug AS provider_slug,
		     c.name AS connection_name,
		     0::int AS attempts
		   FROM webhook_deliveries d
		   JOIN webhooks w ON w.id = d.webhook_id
		   JOIN integration_connections c ON c.id = w.integration_connection_id
		   JOIN integration_providers p ON p.id = c.provider_id
		   LEFT JOIN leads l ON l.id = d.lead_id
		   WHERE c.account_id = $1
		     AND ($2 = '' OR d.status = $2::webhook_delivery_status)
		 ) combined
		 ORDER BY created_at DESC
		 LIMIT $3 OFFSET $4`,
		accountID, status, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []InboundLogItem
	for rows.Next() {
		var it InboundLogItem
		if err := rows.Scan(
			&it.Kind, &it.Direction, &it.ID, &it.CreatedAt, &it.Origin, &it.OriginSlug,
			&it.LeadLabel, &it.LeadID, &it.Status, &it.WebhookID, &it.ErrorMessage,
			&it.ProviderSlug, &it.ConnectionName, &it.Attempts,
		); err != nil {
			return nil, err
		}
		items = append(items, it)
	}
	if items == nil {
		items = []InboundLogItem{}
	}
	return &InboundLogListResponse{Items: items, Total: total, Page: page, Limit: limit}, rows.Err()
}

func (s *Service) listInboundLogAll(ctx context.Context, accountID int64, p ListInboundLogParams) (*InboundLogListResponse, error) {
	limit := p.Limit
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	page := p.Page
	if page < 1 {
		page = 1
	}
	offset := (page - 1) * limit

	var total int64
	if err := s.pool.QueryRow(ctx,
		`SELECT
		   (SELECT COUNT(*) FROM lead_intake_queue q JOIN leads l ON l.id = q.lead_id WHERE l.publisher_id = $1)
		 + (SELECT COUNT(*) FROM webhook_deliveries d JOIN webhooks w ON w.id = d.webhook_id WHERE w.account_id = $1)
		 + (SELECT COUNT(*) FROM integration_delivery_queue q
		      JOIN integration_connections c ON c.id = q.connection_id
		      WHERE c.account_id = $1 AND q.webhook_trigger_id IS NULL)
		 + (SELECT COUNT(*) FROM integration_delivery_queue q
		      JOIN integration_connections c ON c.id = q.connection_id
		      JOIN webhook_outbound_triggers t ON t.id = q.webhook_trigger_id
		      JOIN webhooks w ON w.id = t.webhook_id
		      WHERE w.account_id = $1)`,
		accountID).Scan(&total); err != nil {
		return nil, err
	}

	rows, err := s.pool.Query(ctx,
		`SELECT kind, direction, id, created_at, origin, origin_slug, lead_label, lead_id, status,
		        first_name, last_name, phone, source, raw_payload, webhook_id, error_message,
		        provider_slug, connection_name, attempts
		 FROM (
		   SELECT
		     'source'::text AS kind,
		     'inbound'::text AS direction,
		     q.id,
		     q.created_at,
		     COALESCE(q.source, '') AS origin,
		     COALESCE(q.source, '') AS origin_slug,
		     TRIM(CONCAT(l.first_name, ' ', l.last_name)) AS lead_label,
		     q.lead_id,
		     q.status::text AS status,
		     l.first_name,
		     l.last_name,
		     l.phone,
		     q.source,
		     q.raw_payload,
		     0::bigint AS webhook_id,
		     NULL::text AS error_message,
		     ''::text AS provider_slug,
		     ''::text AS connection_name,
		     0::int AS attempts
		   FROM lead_intake_queue q
		   JOIN leads l ON l.id = q.lead_id
		   WHERE l.publisher_id = $1

		   UNION ALL

		   SELECT
		     'webhook'::text AS kind,
		     'inbound'::text AS direction,
		     d.id,
		     d.created_at,
		     w.name AS origin,
		     w.slug AS origin_slug,
		     COALESCE(l.public_id::text, '') AS lead_label,
		     d.lead_id,
		     d.status::text AS status,
		     ''::text AS first_name,
		     ''::text AS last_name,
		     NULL::text AS phone,
		     NULL::text AS source,
		     NULL::jsonb AS raw_payload,
		     d.webhook_id,
		     d.error_message,
		     ''::text AS provider_slug,
		     ''::text AS connection_name,
		     0::int AS attempts
		   FROM webhook_deliveries d
		   JOIN webhooks w ON w.id = d.webhook_id
		   LEFT JOIN leads l ON l.id = d.lead_id
		   WHERE w.account_id = $1

		   UNION ALL

		   SELECT
		     'webhook'::text AS kind,
		     'outbound'::text AS direction,
		     q.id,
		     q.created_at,
		     w.name AS origin,
		     w.slug AS origin_slug,
		     COALESCE(l.public_id::text, '') AS lead_label,
		     q.lead_id,
		     q.status::text AS status,
		     ''::text AS first_name,
		     ''::text AS last_name,
		     NULL::text AS phone,
		     NULL::text AS source,
		     NULL::jsonb AS raw_payload,
		     w.id AS webhook_id,
		     q.last_error AS error_message,
		     'webhook'::text AS provider_slug,
		     ''::text AS connection_name,
		     q.attempts
		   FROM integration_delivery_queue q
		   JOIN webhook_outbound_triggers t ON t.id = q.webhook_trigger_id
		   JOIN webhooks w ON w.id = t.webhook_id
		   LEFT JOIN leads l ON l.id = q.lead_id
		   WHERE w.account_id = $1

		   UNION ALL

		   SELECT
		     'integration'::text AS kind,
		     'outbound'::text AS direction,
		     q.id,
		     q.created_at,
		     c.name AS origin,
		     p.slug AS origin_slug,
		     COALESCE(l.public_id::text, '') AS lead_label,
		     q.lead_id,
		     q.status::text AS status,
		     ''::text AS first_name,
		     ''::text AS last_name,
		     NULL::text AS phone,
		     NULL::text AS source,
		     NULL::jsonb AS raw_payload,
		     0::bigint AS webhook_id,
		     q.last_error AS error_message,
		     p.slug AS provider_slug,
		     c.name AS connection_name,
		     q.attempts
		   FROM integration_delivery_queue q
		   JOIN integration_connections c ON c.id = q.connection_id
		   JOIN integration_providers p ON p.id = c.provider_id
		   LEFT JOIN leads l ON l.id = q.lead_id
		   WHERE c.account_id = $1
		     AND q.webhook_trigger_id IS NULL
		 ) combined
		 ORDER BY created_at DESC
		 LIMIT $2 OFFSET $3`,
		accountID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []InboundLogItem
	for rows.Next() {
		var it InboundLogItem
		var raw []byte
		if err := rows.Scan(
			&it.Kind, &it.Direction, &it.ID, &it.CreatedAt, &it.Origin, &it.OriginSlug,
			&it.LeadLabel, &it.LeadID, &it.Status,
			&it.FirstName, &it.LastName, &it.Phone, &it.Source, &raw,
			&it.WebhookID, &it.ErrorMessage,
			&it.ProviderSlug, &it.ConnectionName, &it.Attempts,
		); err != nil {
			return nil, err
		}
		if len(raw) > 0 {
			it.RawPayload = raw
		}
		if it.Kind == "source" {
			if err := s.enrichInboundSourceItem(ctx, accountID, &it); err != nil {
				return nil, err
			}
		}
		items = append(items, it)
	}
	if items == nil {
		items = []InboundLogItem{}
	}
	return &InboundLogListResponse{Items: items, Total: total, Page: page, Limit: limit}, rows.Err()
}

func (s *Service) enrichInboundSourceItem(ctx context.Context, accountID int64, it *InboundLogItem) error {
	q := QueueItem{
		ID:         it.ID,
		LeadID:     derefInt64(it.LeadID),
		FirstName:  it.FirstName,
		LastName:   it.LastName,
		Phone:      it.Phone,
		Source:     it.Source,
		RawPayload: it.RawPayload,
		Status:     it.Status,
		CreatedAt:  it.CreatedAt,
	}
	if err := s.enrichQueueItem(ctx, accountID, &q); err != nil {
		return err
	}
	it.UnmappedKeys = q.UnmappedKeys
	return nil
}

func derefInt64(p *int64) int64 {
	if p == nil {
		return 0
	}
	return *p
}

func firstOr(v, fallback string) string {
	if v == "" {
		return fallback
	}
	return v
}
