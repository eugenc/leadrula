package intake

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

type InboundLogItem struct {
	Kind               string          `json:"kind"`
	Direction          string          `json:"direction"`
	ID                 int64           `json:"id"`
	CreatedAt          time.Time       `json:"created_at"`
	Origin             string          `json:"origin"`
	OriginSlug         string          `json:"origin_slug"`
	LeadLabel          string          `json:"lead_label"`
	LeadID             *int64          `json:"lead_id,omitempty"`
	Status             string          `json:"status"`
	UnmappedKeys       []string        `json:"unmapped_keys,omitempty"`
	FirstName          string          `json:"first_name,omitempty"`
	LastName           string          `json:"last_name,omitempty"`
	Phone              *string         `json:"phone,omitempty"`
	Source             *string         `json:"source,omitempty"`
	RawPayload         json.RawMessage `json:"raw_payload,omitempty"`
	WebhookID          int64           `json:"webhook_id,omitempty"`
	ErrorMessage       *string         `json:"error_message,omitempty"`
	ProviderSlug       string          `json:"provider_slug,omitempty"`
	ConnectionName     string          `json:"connection_name,omitempty"`
	Attempts           int             `json:"attempts,omitempty"`
	RouteID            *int64          `json:"route_id,omitempty"`
	RouteName          string          `json:"route_name,omitempty"`
	TriggerType        string          `json:"trigger_type,omitempty"`
	TriggerLabel       string          `json:"trigger_label,omitempty"`
	TargetAccountName  string          `json:"target_account_name,omitempty"`
	Destination        string          `json:"destination,omitempty"`
	BranchPosition     int             `json:"branch_position,omitempty"`
	TargetPipelineName *string         `json:"target_pipeline_name,omitempty"`
	TargetStageName    *string         `json:"target_stage_name,omitempty"`
	Delivery           string          `json:"delivery,omitempty"`
}

type InboundLogListResponse struct {
	Items []InboundLogItem `json:"items"`
	Total int64            `json:"total"`
	Page  int              `json:"page"`
	Limit int              `json:"limit"`
}

type ListInboundLogParams struct {
	AccountType string
	Type        string
	Status      string
	Search      string
	Source      string
	WebhookID   int64
	LeadID      int64
	Page        int
	Limit       int
}

func (s *Service) ListInboundLog(ctx context.Context, accountID int64, p ListInboundLogParams) (*InboundLogListResponse, error) {
	switch p.Type {
	case "source":
		return s.listInboundLogSources(ctx, accountID, p)
	case "webhook":
		return s.listInboundLogWebhooks(ctx, accountID, p)
	case "integration":
		return s.listInboundLogIntegrations(ctx, accountID, p)
	case "route":
		return s.listInboundLogRoutes(ctx, accountID, p)
	default:
		return s.listInboundLogAll(ctx, accountID, p)
	}
}

func (s *Service) listInboundLogSources(ctx context.Context, accountID int64, p ListInboundLogParams) (*InboundLogListResponse, error) {
	limit := p.Limit
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	page := p.Page
	if page < 1 {
		page = 1
	}
	offset := (page - 1) * limit
	status := firstOr(p.Status, "all")
	includeRoutes := status == "all" || status == "routed"

	args := []any{accountID}
	queueWhere := "l.publisher_id = $1"
	routeWhere := "e.owner_account_id = $1 AND e.trigger_type = 'source_ingest'"

	if status != "all" {
		args = append(args, status)
		queueWhere += fmt.Sprintf(" AND q.status = $%d::intake_status", len(args))
	}
	if p.Source != "" {
		args = append(args, p.Source)
		n := len(args)
		queueWhere += fmt.Sprintf(" AND q.source = $%d", n)
		routeWhere += fmt.Sprintf(" AND e.trigger_label = $%d", n)
	}
	if clause, extra := logLeadFilterClause(len(args)+1, p.LeadID, p.Search, "q.lead_id"); clause != "" {
		args = append(args, extra...)
		queueWhere += clause
		routeWhere += strings.Replace(clause, "q.lead_id", "e.lead_id", 1)
	}

	countSQL := `(SELECT COUNT(*) FROM lead_intake_queue q JOIN leads l ON l.id = q.lead_id WHERE ` + queueWhere + `)`
	if includeRoutes {
		countSQL += ` + (SELECT COUNT(*) FROM route_executions e JOIN leads l ON l.id = e.lead_id WHERE ` + routeWhere + `)`
	}

	var total int64
	if err := s.pool.QueryRow(ctx, `SELECT `+countSQL, args...).Scan(&total); err != nil {
		return nil, err
	}

	queueSelect := `
	   SELECT
	     'queue'::text AS src_kind,
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
	     q.raw_payload
	   FROM lead_intake_queue q
	   JOIN leads l ON l.id = q.lead_id
	   WHERE ` + queueWhere

	combined := queueSelect
	if includeRoutes {
		routeSelect := `
	   SELECT
	     'route'::text AS src_kind,
	     e.id,
	     e.created_at,
	     COALESCE(e.trigger_label, '') AS origin,
	     COALESCE(e.trigger_label, '') AS origin_slug,
	     TRIM(CONCAT(l.first_name, ' ', l.last_name)) AS lead_label,
	     e.lead_id,
	     CASE WHEN e.status = 'success' THEN 'routed' ELSE e.status::text END AS status,
	     l.first_name,
	     l.last_name,
	     l.phone,
	     NULL::text AS source,
	     NULL::jsonb AS raw_payload
	   FROM route_executions e
	   JOIN leads l ON l.id = e.lead_id
	   WHERE ` + routeWhere
		combined += `
		   UNION ALL` + routeSelect
	}

	limitArg := len(args) + 1
	offsetArg := len(args) + 2
	queryArgs := append(append([]any{}, args...), limit, offset)

	query := `SELECT src_kind, id, created_at, origin, origin_slug, lead_label, lead_id, status,
		        first_name, last_name, phone, source, raw_payload
		 FROM (` + combined + `
		 ) combined
		 ORDER BY created_at DESC
		 LIMIT $` + fmt.Sprint(limitArg) + ` OFFSET $` + fmt.Sprint(offsetArg)

	rows, err := s.pool.Query(ctx, query, queryArgs...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []InboundLogItem
	for rows.Next() {
		var it InboundLogItem
		var srcKind string
		var leadID int64
		var raw []byte
		if err := rows.Scan(
			&srcKind, &it.ID, &it.CreatedAt, &it.Origin, &it.OriginSlug,
			&it.LeadLabel, &leadID, &it.Status,
			&it.FirstName, &it.LastName, &it.Phone, &it.Source, &raw,
		); err != nil {
			return nil, err
		}
		it.Kind = "source"
		it.Direction = "inbound"
		it.LeadID = &leadID
		if len(raw) > 0 {
			it.RawPayload = raw
		}
		if srcKind == "queue" {
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

	args := []any{accountID, status, webhookID}
	where := `w.account_id = $1
		   AND ($2 = '' OR d.status = $2::webhook_delivery_status)
		   AND ($3 = 0 OR d.webhook_id = $3)`
	where, args = appendLogLeadFilter(where, args, p.LeadID, p.Search, "d.lead_id")

	var total int64
	if err := s.pool.QueryRow(ctx,
		`SELECT COUNT(*)
		 FROM webhook_deliveries d
		 JOIN webhooks w ON w.id = d.webhook_id
		 LEFT JOIN leads l ON l.id = d.lead_id
		 WHERE `+where, args...).Scan(&total); err != nil {
		return nil, err
	}

	limitArg := len(args) + 1
	offsetArg := len(args) + 2
	args = append(args, limit, offset)

	rows, err := s.pool.Query(ctx,
		`SELECT d.id, d.webhook_id, w.name, w.slug, d.lead_id, l.public_id::text,
		        COALESCE(l.first_name, ''), COALESCE(l.last_name, ''),
		        d.status, d.error_message, d.created_at
		 FROM webhook_deliveries d
		 JOIN webhooks w ON w.id = d.webhook_id
		 LEFT JOIN leads l ON l.id = d.lead_id
		 WHERE `+where+`
		 ORDER BY d.created_at DESC
		 LIMIT $`+fmt.Sprint(limitArg)+` OFFSET $`+fmt.Sprint(offsetArg),
		args...)
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
			&d.LeadID, &leadPublicID, &d.FirstName, &d.LastName,
			&d.Status, &d.ErrorMessage, &d.CreatedAt,
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

	args := []any{accountID, status}
	integrationWhere := `c.account_id = $1
		      AND q.webhook_trigger_id IS NULL
		      AND ($2 = '' OR q.status = $2::delivery_status)`
	webhookWhere := `c.account_id = $1
		      AND ($2 = '' OR d.status = $2::webhook_delivery_status)`

	if clause, extra := logLeadFilterClause(3, p.LeadID, p.Search, "q.lead_id"); clause != "" {
		args = append(args, extra...)
		integrationWhere += clause
		webhookWhere += strings.Replace(clause, "q.lead_id", "d.lead_id", 1)
	}

	var total int64
	if err := s.pool.QueryRow(ctx,
		`SELECT
		   (SELECT COUNT(*)
		    FROM integration_delivery_queue q
		    JOIN integration_connections c ON c.id = q.connection_id
		    LEFT JOIN leads l ON l.id = q.lead_id
		    WHERE `+integrationWhere+`)
		 + (SELECT COUNT(*)
		    FROM webhook_deliveries d
		    JOIN webhooks w ON w.id = d.webhook_id
		    JOIN integration_connections c ON c.id = w.integration_connection_id
		    LEFT JOIN leads l ON l.id = d.lead_id
		    WHERE `+webhookWhere+`)`,
		args...).Scan(&total); err != nil {
		return nil, err
	}

	limitArg := len(args) + 1
	offsetArg := len(args) + 2
	queryArgs := append(append([]any{}, args...), limit, offset)

	rows, err := s.pool.Query(ctx,
		`SELECT kind, direction, id, created_at, origin, origin_slug, lead_label, lead_id, status,
		        first_name, last_name, webhook_id, error_message, provider_slug, connection_name, attempts
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
		     COALESCE(l.first_name, '') AS first_name,
		     COALESCE(l.last_name, '') AS last_name,
		     0::bigint AS webhook_id,
		     q.last_error AS error_message,
		     p.slug AS provider_slug,
		     c.name AS connection_name,
		     q.attempts
		   FROM integration_delivery_queue q
		   JOIN integration_connections c ON c.id = q.connection_id
		   JOIN integration_providers p ON p.id = c.provider_id
		   LEFT JOIN leads l ON l.id = q.lead_id
		   WHERE `+integrationWhere+`

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
		     COALESCE(l.first_name, '') AS first_name,
		     COALESCE(l.last_name, '') AS last_name,
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
		   WHERE `+webhookWhere+`
		 ) combined
		 ORDER BY created_at DESC
		 LIMIT $`+fmt.Sprint(limitArg)+` OFFSET $`+fmt.Sprint(offsetArg),
		queryArgs...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []InboundLogItem
	for rows.Next() {
		var it InboundLogItem
		if err := rows.Scan(
			&it.Kind, &it.Direction, &it.ID, &it.CreatedAt, &it.Origin, &it.OriginSlug,
			&it.LeadLabel, &it.LeadID, &it.Status,
			&it.FirstName, &it.LastName,
			&it.WebhookID, &it.ErrorMessage,
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

	isPublisher := p.AccountType != "buyer"

	args := []any{accountID}
	leadClauseQ, leadExtra := logLeadFilterClause(2, p.LeadID, p.Search, "q.lead_id")
	leadClauseD, _ := logLeadFilterClause(2, p.LeadID, p.Search, "d.lead_id")
	leadClauseE, _ := logLeadFilterClause(2, p.LeadID, p.Search, "e.lead_id")
	if leadClauseQ != "" {
		args = append(args, leadExtra...)
		leadClauseD = strings.Replace(leadClauseQ, "q.lead_id", "d.lead_id", 1)
		leadClauseE = strings.Replace(leadClauseQ, "q.lead_id", "e.lead_id", 1)
	}

	routeVis := routeVisibilitySQL(p.AccountType, 1)
	routeCountJoin := ""
	if leadClauseE != "" {
		routeCountJoin = " JOIN leads l ON l.id = e.lead_id"
	}

	var countSQL string
	if isPublisher {
		countSQL = `SELECT
		   (SELECT COUNT(*) FROM lead_intake_queue q JOIN leads l ON l.id = q.lead_id WHERE l.publisher_id = $1` + leadClauseQ + `)
		 + (SELECT COUNT(*) FROM webhook_deliveries d JOIN webhooks w ON w.id = d.webhook_id LEFT JOIN leads l ON l.id = d.lead_id WHERE w.account_id = $1` + leadClauseD + `)
		 + (SELECT COUNT(*) FROM integration_delivery_queue q
		      JOIN integration_connections c ON c.id = q.connection_id
		      LEFT JOIN leads l ON l.id = q.lead_id
		      WHERE c.account_id = $1 AND q.webhook_trigger_id IS NULL` + leadClauseQ + `)
		 + (SELECT COUNT(*) FROM integration_delivery_queue q
		      JOIN integration_connections c ON c.id = q.connection_id
		      JOIN webhook_outbound_triggers t ON t.id = q.webhook_trigger_id
		      JOIN webhooks w ON w.id = t.webhook_id
		      LEFT JOIN leads l ON l.id = q.lead_id
		      WHERE w.account_id = $1` + leadClauseQ + `)
		 + (SELECT COUNT(*) FROM route_executions e` + routeCountJoin + ` WHERE ` + routeVis + leadClauseE + `)`
	} else {
		countSQL = `SELECT
		   (SELECT COUNT(*) FROM webhook_deliveries d JOIN webhooks w ON w.id = d.webhook_id LEFT JOIN leads l ON l.id = d.lead_id WHERE w.account_id = $1` + leadClauseD + `)
		 + (SELECT COUNT(*) FROM integration_delivery_queue q
		      JOIN integration_connections c ON c.id = q.connection_id
		      LEFT JOIN leads l ON l.id = q.lead_id
		      WHERE c.account_id = $1 AND q.webhook_trigger_id IS NULL` + leadClauseQ + `)
		 + (SELECT COUNT(*) FROM integration_delivery_queue q
		      JOIN integration_connections c ON c.id = q.connection_id
		      JOIN webhook_outbound_triggers t ON t.id = q.webhook_trigger_id
		      JOIN webhooks w ON w.id = t.webhook_id
		      LEFT JOIN leads l ON l.id = q.lead_id
		      WHERE w.account_id = $1` + leadClauseQ + `)
		 + (SELECT COUNT(*) FROM route_executions e` + routeCountJoin + ` WHERE ` + routeVis + leadClauseE + `)`
	}

	var total int64
	if err := s.pool.QueryRow(ctx, countSQL, args...).Scan(&total); err != nil {
		return nil, err
	}

	nullRouteCols := `,
		     NULL::bigint AS route_id,
		     ''::text AS route_name,
		     ''::text AS trigger_type,
		     ''::text AS trigger_label,
		     ''::text AS target_account_name,
		     ''::text AS destination,
		     0::int AS branch_position,
		     ''::text AS target_pipeline_name,
		     ''::text AS target_stage_name,
		     ''::text AS delivery`

	sourceUnion := `
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
		     0::int AS attempts` + nullRouteCols + `
		   FROM lead_intake_queue q
		   JOIN leads l ON l.id = q.lead_id
		   WHERE l.publisher_id = $1` + leadClauseQ

	webhookInboundUnion := `
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
		     COALESCE(l.first_name, '') AS first_name,
		     COALESCE(l.last_name, '') AS last_name,
		     NULL::text AS phone,
		     NULL::text AS source,
		     NULL::jsonb AS raw_payload,
		     d.webhook_id,
		     d.error_message,
		     ''::text AS provider_slug,
		     ''::text AS connection_name,
		     0::int AS attempts` + nullRouteCols + `
		   FROM webhook_deliveries d
		   JOIN webhooks w ON w.id = d.webhook_id
		   LEFT JOIN leads l ON l.id = d.lead_id
		   WHERE w.account_id = $1` + leadClauseD

	webhookOutboundUnion := `
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
		     COALESCE(l.first_name, '') AS first_name,
		     COALESCE(l.last_name, '') AS last_name,
		     NULL::text AS phone,
		     NULL::text AS source,
		     NULL::jsonb AS raw_payload,
		     w.id AS webhook_id,
		     q.last_error AS error_message,
		     'webhook'::text AS provider_slug,
		     ''::text AS connection_name,
		     q.attempts` + nullRouteCols + `
		   FROM integration_delivery_queue q
		   JOIN webhook_outbound_triggers t ON t.id = q.webhook_trigger_id
		   JOIN webhooks w ON w.id = t.webhook_id
		   LEFT JOIN leads l ON l.id = q.lead_id
		   WHERE w.account_id = $1` + leadClauseQ

	integrationUnion := `
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
		     COALESCE(l.first_name, '') AS first_name,
		     COALESCE(l.last_name, '') AS last_name,
		     NULL::text AS phone,
		     NULL::text AS source,
		     NULL::jsonb AS raw_payload,
		     0::bigint AS webhook_id,
		     q.last_error AS error_message,
		     p.slug AS provider_slug,
		     c.name AS connection_name,
		     q.attempts` + nullRouteCols + `
		   FROM integration_delivery_queue q
		   JOIN integration_connections c ON c.id = q.connection_id
		   JOIN integration_providers p ON p.id = c.provider_id
		   LEFT JOIN leads l ON l.id = q.lead_id
		   WHERE c.account_id = $1
		     AND q.webhook_trigger_id IS NULL` + leadClauseQ

	combined := webhookInboundUnion + `
		   UNION ALL` + webhookOutboundUnion + `
		   UNION ALL` + integrationUnion + `
		   UNION ALL` + buildRouteLogUnionSQL(p.AccountType, leadClauseE)

	if isPublisher {
		combined = sourceUnion + `
		   UNION ALL` + combined
	}

	limitArg := len(args) + 1
	offsetArg := len(args) + 2
	queryArgs := append(append([]any{}, args...), limit, offset)

	query := `SELECT kind, direction, id, created_at, origin, origin_slug, lead_label, lead_id, status,
		        first_name, last_name, phone, source, raw_payload, webhook_id, error_message,
		        provider_slug, connection_name, attempts,
		        route_id, route_name, trigger_type, trigger_label, target_account_name, destination, branch_position,
		        target_pipeline_name, target_stage_name, delivery
		 FROM (` + combined + `
		 ) combined
		 ORDER BY created_at DESC
		 LIMIT $` + fmt.Sprint(limitArg) + ` OFFSET $` + fmt.Sprint(offsetArg)

	rows, err := s.pool.Query(ctx, query, queryArgs...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []InboundLogItem
	for rows.Next() {
		var it InboundLogItem
		var raw []byte
		var routeID *int64
		var targetPipelineName, targetStageName string
		if err := rows.Scan(
			&it.Kind, &it.Direction, &it.ID, &it.CreatedAt, &it.Origin, &it.OriginSlug,
			&it.LeadLabel, &it.LeadID, &it.Status,
			&it.FirstName, &it.LastName, &it.Phone, &it.Source, &raw,
			&it.WebhookID, &it.ErrorMessage,
			&it.ProviderSlug, &it.ConnectionName, &it.Attempts,
			&routeID, &it.RouteName, &it.TriggerType, &it.TriggerLabel,
			&it.TargetAccountName, &it.Destination, &it.BranchPosition,
			&targetPipelineName, &targetStageName, &it.Delivery,
		); err != nil {
			return nil, err
		}
		if targetPipelineName != "" {
			it.TargetPipelineName = &targetPipelineName
		}
		if targetStageName != "" {
			it.TargetStageName = &targetStageName
		}
		if len(raw) > 0 {
			it.RawPayload = raw
		}
		it.RouteID = routeID
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
