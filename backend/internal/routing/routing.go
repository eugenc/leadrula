// Package routing manages inbound sources, routes, and field mapping.
package routing

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/echayko/leadrula/backend/internal/customfields"
	"github.com/echayko/leadrula/backend/internal/database"
	"github.com/echayko/leadrula/backend/pkg/httpx"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Source struct {
	ID             int64     `json:"id"`
	PublisherID    int64     `json:"-"`
	Name           string    `json:"name"`
	Slug           string    `json:"slug"`
	Type           string    `json:"type"`
	IsActive       bool      `json:"is_active"`
	APIKeyRequired bool      `json:"api_key_required"`
	CreatedAt      time.Time `json:"created_at"`
}

type SourceFieldMapEntry struct {
	ID            int64     `json:"id"`
	SourceID      int64     `json:"source_id"`
	SourceKey     string    `json:"source_key"`
	TargetType    string    `json:"target_type"`
	BuiltinField  *string   `json:"builtin_field"`
	CustomFieldID *int64    `json:"custom_field_id"`
	CreatedAt     time.Time `json:"created_at"`
}

// SourceSamplePayload is the most recent webhook body for a source.
type SourceSamplePayload struct {
	Payload    json.RawMessage `json:"payload"`
	ReceivedAt *time.Time      `json:"received_at,omitempty"`
}

type Route struct {
	ID                   int64     `json:"id"`
	PublisherID          *int64    `json:"-"`
	BuyerID              *int64    `json:"buyer_id,omitempty"`
	Name                 string    `json:"name"`
	Origin               string    `json:"origin"`
	SourceID             *int64    `json:"source_id"`
	SourceName           *string   `json:"source_name,omitempty"`
	OriginPipelineID     *int64    `json:"origin_pipeline_id"`
	OriginStageID        *int64    `json:"origin_stage_id"`
	OriginPipelineName   *string   `json:"origin_pipeline_name,omitempty"`
	OriginStageName      *string   `json:"origin_stage_name,omitempty"`
	OriginWebhookID      *int64    `json:"origin_webhook_id"`
	OriginWebhookName    *string   `json:"origin_webhook_name,omitempty"`
	OriginConnectionID   *int64    `json:"origin_connection_id"`
	OriginConnectionName *string   `json:"origin_connection_name,omitempty"`
	Destination          string    `json:"destination"`
	ContractID           *int64    `json:"contract_id"`
	CompensationID       *int64    `json:"compensation_id,omitempty"`
	ContractName         *string   `json:"contract_name,omitempty"`
	BuyerName            *string   `json:"buyer_name,omitempty"`
	Delivery             string    `json:"delivery"`
	TargetPipelineID     *int64    `json:"target_pipeline_id"`
	TargetStageID        *int64    `json:"target_stage_id"`
	TargetPipelineName   *string   `json:"target_pipeline_name,omitempty"`
	TargetStageName      *string   `json:"target_stage_name,omitempty"`
	DestWebhookID        *int64          `json:"dest_webhook_id"`
	DestWebhookName      *string         `json:"dest_webhook_name,omitempty"`
	Branches             json.RawMessage `json:"branches"`
	IsActive             bool            `json:"is_active"`
	CreatedAt            time.Time       `json:"created_at"`
	MatchedBranchPosition int            `json:"-"`
}

// OwnerAccountID returns the account that owns this route definition.
func (rt *Route) OwnerAccountID() int64 {
	if rt.BuyerID != nil {
		return *rt.BuyerID
	}
	if rt.PublisherID != nil {
		return *rt.PublisherID
	}
	return 0
}

// BuyerOwned reports whether the buyer account created this route.
func (rt *Route) BuyerOwned() bool { return rt.BuyerID != nil }

type RouteFieldMapEntry struct {
	ID               int64     `json:"id"`
	RouteID          int64     `json:"route_id"`
	SrcType          string    `json:"src_type"`
	SrcBuiltin       *string   `json:"src_builtin"`
	SrcCustomFieldID *int64    `json:"src_custom_field_id"`
	SrcLabel         *string   `json:"src_label,omitempty"`
	DstType          string    `json:"dst_type"`
	DstBuiltin       *string   `json:"dst_builtin"`
	DstCustomFieldID *int64    `json:"dst_custom_field_id"`
	DstLabel         *string   `json:"dst_label,omitempty"`
	CreatedAt        time.Time `json:"created_at"`
}

type RouteFieldMapOptions struct {
	BuyerName       string                  `json:"buyer_name"`
	PublisherFields []customfields.CustomField `json:"publisher_fields"`
	BuyerFields     []customfields.CustomField `json:"buyer_fields"`
}

type CreateRouteParams struct {
	Name               string `json:"name"`
	Origin             string `json:"origin"`
	SourceID           *int64 `json:"source_id"`
	OriginPipelineID   *int64 `json:"origin_pipeline_id"`
	OriginStageID      *int64 `json:"origin_stage_id"`
	OriginWebhookID    *int64 `json:"origin_webhook_id"`
	OriginConnectionID *int64 `json:"origin_connection_id"`
	Destination        string `json:"destination"`
	ContractID         *int64 `json:"contract_id"`
	CompensationID     *int64 `json:"compensation_id"`
	Delivery           string `json:"delivery"`
	TargetPipelineID   *int64 `json:"target_pipeline_id"`
	TargetStageID      *int64 `json:"target_stage_id"`
	DestWebhookID      *int64        `json:"dest_webhook_id"`
	Branches           []RouteBranch `json:"branches"`
}

type UpdateRouteParams struct {
	Name               *string `json:"name"`
	IsActive           *bool   `json:"is_active"`
	SourceID           *int64  `json:"source_id"`
	OriginPipelineID   *int64  `json:"origin_pipeline_id"`
	OriginStageID      *int64  `json:"origin_stage_id"`
	OriginWebhookID    *int64  `json:"origin_webhook_id"`
	OriginConnectionID *int64  `json:"origin_connection_id"`
	Destination        *string `json:"destination"`
	ContractID         *int64  `json:"contract_id"`
	CompensationID     *int64  `json:"compensation_id"`
	Delivery           *string `json:"delivery"`
	TargetPipelineID   *int64  `json:"target_pipeline_id"`
	TargetStageID      *int64  `json:"target_stage_id"`
	DestWebhookID      *int64         `json:"dest_webhook_id"`
	Branches           *[]RouteBranch `json:"branches"`
}

type Service struct {
	pool *pgxpool.Pool
}

func NewService(pool *pgxpool.Pool) *Service { return &Service{pool: pool} }

const routeFrom = `
 FROM routes r
 LEFT JOIN routing_sources s ON s.id = r.source_id
 LEFT JOIN contracts c ON c.id = r.contract_id
 LEFT JOIN accounts ba ON ba.id = c.buyer_id
 LEFT JOIN pipelines op ON op.id = r.origin_pipeline_id
 LEFT JOIN pipeline_stages os ON os.id = r.origin_stage_id
 LEFT JOIN webhooks ow ON ow.id = r.origin_webhook_id
 LEFT JOIN integration_connections oc ON oc.id = r.origin_connection_id
 LEFT JOIN pipelines tp ON tp.id = r.target_pipeline_id AND r.destination = 'pipeline'
 LEFT JOIN pipeline_stages ts ON ts.id = r.target_stage_id AND r.destination = 'pipeline'
 LEFT JOIN pipelines bp ON bp.id = c.buyer_pipeline_id
 LEFT JOIN pipeline_stages bs ON bs.id = r.target_stage_id AND r.destination = 'contract'
 LEFT JOIN webhooks dw ON dw.id = r.dest_webhook_id`

const routeCols = `r.id, r.publisher_id, r.buyer_id, r.name, r.origin, r.source_id, s.name,
	r.origin_pipeline_id, r.origin_stage_id, r.origin_webhook_id, ow.name,
	r.origin_connection_id, oc.name,
	r.destination, r.contract_id, r.compensation_id, c.name,
	r.delivery, r.target_pipeline_id, r.target_stage_id, r.dest_webhook_id, dw.name,
	r.branches,
	r.is_active, r.created_at,
	ba.name, op.name, os.name,
	CASE WHEN r.destination = 'pipeline' THEN tp.name WHEN r.destination = 'contract' AND r.delivery = 'leads_pipeline' THEN bp.name ELSE NULL END,
	CASE WHEN r.destination = 'pipeline' THEN ts.name WHEN r.destination = 'contract' AND r.delivery = 'leads_pipeline' THEN bs.name ELSE NULL END`

func scanRoute(row pgx.Row) (*Route, error) {
	rt := &Route{}
	err := row.Scan(&rt.ID, &rt.PublisherID, &rt.BuyerID, &rt.Name, &rt.Origin, &rt.SourceID, &rt.SourceName,
		&rt.OriginPipelineID, &rt.OriginStageID, &rt.OriginWebhookID, &rt.OriginWebhookName,
		&rt.OriginConnectionID, &rt.OriginConnectionName,
		&rt.Destination, &rt.ContractID, &rt.CompensationID, &rt.ContractName,
		&rt.Delivery, &rt.TargetPipelineID, &rt.TargetStageID, &rt.DestWebhookID, &rt.DestWebhookName,
		&rt.Branches,
		&rt.IsActive, &rt.CreatedAt,
		&rt.BuyerName, &rt.OriginPipelineName, &rt.OriginStageName, &rt.TargetPipelineName, &rt.TargetStageName)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, httpx.NotFound("route not found")
		}
		return nil, err
	}
	return rt, nil
}

const sourceCols = `id, publisher_id, name, slug, type, is_active, api_key_required, created_at`

func scanSource(row pgx.Row) (*Source, error) {
	s := &Source{}
	err := row.Scan(&s.ID, &s.PublisherID, &s.Name, &s.Slug, &s.Type, &s.IsActive, &s.APIKeyRequired, &s.CreatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return s, nil
}

// MatchSourceBySlug finds an active source by slug. nil means no match.
func MatchSourceBySlug(ctx context.Context, q database.Querier, publisherID int64, slug string) (*Source, error) {
	if slug == "" {
		return nil, nil
	}
	return scanSource(q.QueryRow(ctx,
		`SELECT `+sourceCols+`
		 FROM routing_sources WHERE publisher_id=$1 AND slug=$2 AND is_active`,
		publisherID, slug))
}

// ResolveSourceBySlug finds an active source by globally unique slug.
func ResolveSourceBySlug(ctx context.Context, q database.Querier, slug string) (*Source, error) {
	if slug == "" {
		return nil, nil
	}
	return scanSource(q.QueryRow(ctx,
		`SELECT `+sourceCols+`
		 FROM routing_sources WHERE slug=$1 AND is_active`,
		slug))
}

// SourceFieldMap returns payload mapping rows for a source.
func SourceFieldMap(ctx context.Context, q database.Querier, sourceID int64) ([]SourceFieldMapEntry, error) {
	rows, err := q.Query(ctx,
		`SELECT id, source_id, source_key, target_type, builtin_field, custom_field_id, created_at
		 FROM routing_source_field_map WHERE source_id=$1`, sourceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []SourceFieldMapEntry
	for rows.Next() {
		var e SourceFieldMapEntry
		if err := rows.Scan(&e.ID, &e.SourceID, &e.SourceKey, &e.TargetType, &e.BuiltinField, &e.CustomFieldID, &e.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// RouteForSource finds the first matching active route whose origin is the given source.
func RouteForSource(ctx context.Context, q database.Querier, sourceID, leadID int64, payloadFlat map[string]any) (*Route, error) {
	routes, err := listRoutesForSource(ctx, q, sourceID)
	if err != nil {
		return nil, err
	}
	var accountID int64
	if len(routes) > 0 {
		if routes[0].PublisherID != nil {
			accountID = *routes[0].PublisherID
		}
	}
	return MatchOriginRoutes(ctx, q, accountID, leadID, routes, payloadFlat)
}

func listRoutesForSource(ctx context.Context, q database.Querier, sourceID int64) ([]Route, error) {
	rows, err := q.Query(ctx,
		`SELECT `+routeCols+routeFrom+`
		 WHERE r.source_id=$1 AND r.origin='source' AND r.is_active
		   AND (r.destination <> 'contract' OR (c.status = 'active' AND c.deleted_at IS NULL))
		 ORDER BY r.id ASC`, sourceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanRouteList(rows)
}

// BuyerRouteForSourceAndBuyer finds the first matching active contract route for a source and contract buyer.
func BuyerRouteForSourceAndBuyer(ctx context.Context, q database.Querier, publisherID, sourceID, buyerID, leadID int64, payloadFlat map[string]any) (*Route, error) {
	rows, err := q.Query(ctx,
		`SELECT `+routeCols+routeFrom+`
		 JOIN contracts c ON c.id = r.contract_id
		 WHERE r.publisher_id=$1 AND r.source_id=$2 AND r.origin='source' AND r.destination='contract'
		   AND r.is_active AND c.buyer_id=$3 AND c.deleted_at IS NULL AND c.status = 'active'
		 ORDER BY r.id ASC`, publisherID, sourceID, buyerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	routes, err := scanRouteList(rows)
	if err != nil {
		return nil, err
	}
	return MatchOriginRoutes(ctx, q, publisherID, leadID, routes, payloadFlat)
}

// MatchRouteByStage finds the first matching active pipeline-origin route for a publisher trigger stage.
func MatchRouteByStage(ctx context.Context, q database.Querier, publisherID, stageID, leadID int64, payloadFlat map[string]any) (*Route, error) {
	rows, err := q.Query(ctx,
		`SELECT `+routeCols+routeFrom+`
		 WHERE r.publisher_id=$1 AND r.origin='pipeline' AND r.origin_stage_id=$2 AND r.is_active
		   AND (r.destination <> 'contract' OR (c.status = 'active' AND c.deleted_at IS NULL))
		 ORDER BY r.id ASC`, publisherID, stageID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	routes, err := scanRouteList(rows)
	if err != nil {
		return nil, err
	}
	return MatchOriginRoutes(ctx, q, publisherID, leadID, routes, payloadFlat)
}

// MatchBuyerRouteByStage finds the first matching buyer-owned pipeline-origin route.
func MatchBuyerRouteByStage(ctx context.Context, q database.Querier, buyerID, stageID, leadID int64, payloadFlat map[string]any) (*Route, error) {
	rows, err := q.Query(ctx,
		`SELECT `+routeCols+routeFrom+`
		 WHERE r.buyer_id=$1 AND r.origin='pipeline' AND r.origin_stage_id=$2 AND r.is_active
		 ORDER BY r.id ASC`, buyerID, stageID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	routes, err := scanRouteList(rows)
	if err != nil {
		return nil, err
	}
	return MatchOriginRoutes(ctx, q, buyerID, leadID, routes, payloadFlat)
}

// MatchRouteByOriginWebhook finds the first matching active route triggered by a webhook.
func MatchRouteByOriginWebhook(ctx context.Context, q database.Querier, accountID, webhookID, leadID int64, payloadFlat map[string]any) (*Route, error) {
	rows, err := q.Query(ctx,
		`SELECT `+routeCols+routeFrom+`
		 WHERE r.origin='webhook' AND r.origin_webhook_id=$2 AND r.is_active
		   AND (r.publisher_id=$1 OR r.buyer_id=$1)
		 ORDER BY r.id ASC`, accountID, webhookID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	routes, err := scanRouteList(rows)
	if err != nil {
		return nil, err
	}
	return MatchOriginRoutes(ctx, q, accountID, leadID, routes, payloadFlat)
}

// MatchRouteByOriginConnection finds the first matching active route triggered by an integration connection.
func MatchRouteByOriginConnection(ctx context.Context, q database.Querier, accountID, connectionID, leadID int64, payloadFlat map[string]any) (*Route, error) {
	rows, err := q.Query(ctx,
		`SELECT `+routeCols+routeFrom+`
		 WHERE r.origin='integration' AND r.origin_connection_id=$2 AND r.is_active
		   AND (r.publisher_id=$1 OR r.buyer_id=$1)
		 ORDER BY r.id ASC`, accountID, connectionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	routes, err := scanRouteList(rows)
	if err != nil {
		return nil, err
	}
	return MatchOriginRoutes(ctx, q, accountID, leadID, routes, payloadFlat)
}

func scanRouteOptional(row pgx.Row) (*Route, error) {
	rt, err := scanRoute(row)
	if err != nil {
		var appErr *httpx.AppError
		if errors.As(err, &appErr) && appErr.Code == httpx.CodeNotFound {
			return nil, nil
		}
		return nil, err
	}
	return rt, nil
}

// GetByID loads a route by primary key.
func GetByID(ctx context.Context, q database.Querier, id int64) (*Route, error) {
	return scanRoute(q.QueryRow(ctx, `SELECT `+routeCols+routeFrom+` WHERE r.id=$1`, id))
}

// RouteFieldMap returns lead-field mapping rows for a route.
func RouteFieldMap(ctx context.Context, q database.Querier, routeID int64) ([]RouteFieldMapEntry, error) {
	rows, err := q.Query(ctx,
		`SELECT m.id, m.route_id, m.src_type, m.src_builtin, m.src_custom_field_id,
		        m.dst_type, m.dst_builtin, m.dst_custom_field_id, m.created_at,
		        COALESCE(m.src_builtin, src_cf.name),
		        COALESCE(m.dst_builtin, dst_cf.name)
		 FROM route_field_map m
		 LEFT JOIN custom_fields src_cf ON src_cf.id = m.src_custom_field_id
		 LEFT JOIN custom_fields dst_cf ON dst_cf.id = m.dst_custom_field_id
		 WHERE m.route_id=$1`, routeID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []RouteFieldMapEntry
	for rows.Next() {
		var e RouteFieldMapEntry
		if err := rows.Scan(&e.ID, &e.RouteID, &e.SrcType, &e.SrcBuiltin, &e.SrcCustomFieldID,
			&e.DstType, &e.DstBuiltin, &e.DstCustomFieldID, &e.CreatedAt, &e.SrcLabel, &e.DstLabel); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

func (s *Service) ListSources(ctx context.Context, publisherID int64) ([]Source, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT `+sourceCols+`
		 FROM routing_sources WHERE publisher_id=$1 ORDER BY name`, publisherID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Source
	for rows.Next() {
		var src Source
		if err := rows.Scan(&src.ID, &src.PublisherID, &src.Name, &src.Slug, &src.Type, &src.IsActive, &src.APIKeyRequired, &src.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, src)
	}
	return out, rows.Err()
}

func (s *Service) CreateSource(ctx context.Context, publisherID int64, name, slug, sourceType string, apiKeyRequired *bool) (*Source, error) {
	if sourceType == "" {
		return nil, httpx.Validation("type is required")
	}
	if sourceType != "webhook" {
		return nil, httpx.Validation("type must be webhook")
	}
	required := true
	if apiKeyRequired != nil {
		required = *apiKeyRequired
	}
	return scanSource(s.pool.QueryRow(ctx,
		`INSERT INTO routing_sources(publisher_id, name, slug, type, api_key_required) VALUES ($1,$2,$3,$4,$5)
		 RETURNING `+sourceCols,
		publisherID, name, slug, sourceType, required))
}

func (s *Service) UpdateSource(ctx context.Context, publisherID, id int64, name, slug *string, isActive, apiKeyRequired *bool) (*Source, error) {
	src, err := scanSource(s.pool.QueryRow(ctx,
		`UPDATE routing_sources SET
		   name = COALESCE($3, name),
		   slug = COALESCE($4, slug),
		   is_active = COALESCE($5, is_active),
		   api_key_required = COALESCE($6, api_key_required)
		 WHERE id=$1 AND publisher_id=$2
		 RETURNING `+sourceCols,
		id, publisherID, name, slug, isActive, apiKeyRequired))
	if err == nil && src == nil {
		return nil, httpx.NotFound("source not found")
	}
	if err != nil && database.IsUniqueViolation(err) {
		return nil, httpx.Conflict("a source with this slug already exists")
	}
	return src, err
}

func (s *Service) DeleteSource(ctx context.Context, publisherID, id int64) error {
	ct, err := s.pool.Exec(ctx, `DELETE FROM routing_sources WHERE id=$1 AND publisher_id=$2`, id, publisherID)
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return httpx.NotFound("source not found")
	}
	return nil
}

func (s *Service) SourceOwnedBy(ctx context.Context, publisherID, sourceID int64) (bool, error) {
	var ok bool
	err := s.pool.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM routing_sources WHERE id=$1 AND publisher_id=$2)`, sourceID, publisherID).Scan(&ok)
	return ok, err
}

func (s *Service) ListSourceFieldMap(ctx context.Context, sourceID int64) ([]SourceFieldMapEntry, error) {
	return SourceFieldMap(ctx, s.pool, sourceID)
}

// LatestSourceSamplePayload returns the newest lead raw_payload ingested via the source slug.
func (s *Service) LatestSourceSamplePayload(ctx context.Context, publisherID, sourceID int64) (*SourceSamplePayload, error) {
	var payload json.RawMessage
	var receivedAt time.Time
	err := s.pool.QueryRow(ctx,
		`SELECT l.raw_payload, l.created_at
		 FROM leads l
		 JOIN routing_sources s ON s.slug = l.source AND s.publisher_id = l.publisher_id
		 WHERE s.id = $1 AND s.publisher_id = $2
		 ORDER BY l.created_at DESC
		 LIMIT 1`,
		sourceID, publisherID).Scan(&payload, &receivedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return &SourceSamplePayload{Payload: json.RawMessage("null")}, nil
	}
	if err != nil {
		return nil, err
	}
	return &SourceSamplePayload{Payload: payload, ReceivedAt: &receivedAt}, nil
}

func (s *Service) AddSourceFieldMap(ctx context.Context, publisherID, sourceID int64, sourceKey, targetType string, builtinField *string, customFieldID *int64) (*SourceFieldMapEntry, error) {
	if targetType != "builtin" && targetType != "custom" && targetType != "ignore" {
		return nil, httpx.Validation("target_type must be builtin, custom, or ignore")
	}
	if sourceKey == "" {
		return nil, httpx.Validation("source_key is required")
	}
	ok, err := s.SourceOwnedBy(ctx, publisherID, sourceID)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, httpx.NotFound("source not found")
	}
	if targetType == "custom" {
		if customFieldID == nil || *customFieldID == 0 {
			return nil, httpx.Validation("custom_field_id required for custom target")
		}
		if err := s.customFieldOwnedBy(ctx, *customFieldID, publisherID); err != nil {
			return nil, err
		}
	}
	if targetType == "ignore" {
		builtinField = nil
		customFieldID = nil
	}
	if _, err := s.pool.Exec(ctx,
		`DELETE FROM routing_source_field_map WHERE source_id=$1 AND source_key=$2`,
		sourceID, sourceKey); err != nil {
		return nil, err
	}
	e := &SourceFieldMapEntry{}
	err = s.pool.QueryRow(ctx,
		`INSERT INTO routing_source_field_map(source_id, source_key, target_type, builtin_field, custom_field_id)
		 VALUES ($1,$2,$3,$4,$5)
		 RETURNING id, source_id, source_key, target_type, builtin_field, custom_field_id, created_at`,
		sourceID, sourceKey, targetType, builtinField, customFieldID).Scan(
		&e.ID, &e.SourceID, &e.SourceKey, &e.TargetType, &e.BuiltinField, &e.CustomFieldID, &e.CreatedAt)
	if err != nil {
		if database.IsCheckViolation(err) {
			return nil, httpx.Validation("provide builtin_field for builtin target, custom_field_id for custom target, or neither for ignore")
		}
		return nil, err
	}
	return e, nil
}

func (s *Service) DeleteSourceFieldMap(ctx context.Context, id int64) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM routing_source_field_map WHERE id=$1`, id)
	return err
}

func validateRouteParams(p CreateRouteParams, publisherOwned bool) error {
	if err := normalizeRouteBranches(&p); err != nil {
		return err
	}
	if err := validateRouteBranches(p.Branches, publisherOwned); err != nil {
		return err
	}
	if p.Origin != "source" && p.Origin != "pipeline" && p.Origin != "webhook" && p.Origin != "integration" {
		return httpx.Validation("origin must be source, pipeline, webhook, or integration")
	}
	if !publisherOwned {
		if p.Origin == "source" {
			return httpx.Validation("source origin is publisher-only")
		}
	}
	switch p.Origin {
	case "source":
		if p.SourceID == nil || *p.SourceID == 0 {
			return httpx.Validation("source_id is required for source origin")
		}
	case "pipeline":
		if p.OriginPipelineID == nil || p.OriginStageID == nil {
			return httpx.Validation("origin_pipeline_id and origin_stage_id are required for pipeline origin")
		}
	case "webhook":
		if p.OriginWebhookID == nil || *p.OriginWebhookID == 0 {
			return httpx.Validation("origin_webhook_id is required for webhook origin")
		}
	case "integration":
		if p.OriginConnectionID == nil || *p.OriginConnectionID == 0 {
			return httpx.Validation("origin_connection_id is required for integration origin")
		}
	}
	return nil
}

func (s *Service) validateRouteOwnership(ctx context.Context, accountID int64, publisherOwned bool, p CreateRouteParams) error {
	if err := validateRouteParams(p, publisherOwned); err != nil {
		return err
	}
	if p.Origin == "source" {
		ok, err := s.SourceOwnedBy(ctx, accountID, *p.SourceID)
		if err != nil || !ok {
			return httpx.Validation("source not found")
		}
	}
	if p.Origin == "pipeline" {
		if err := s.pipelineStageOwnedBy(ctx, accountID, *p.OriginPipelineID, *p.OriginStageID); err != nil {
			return err
		}
	}
	if p.Origin == "webhook" {
		if err := s.webhookOwnedBy(ctx, accountID, *p.OriginWebhookID); err != nil {
			return err
		}
	}
	if p.Origin == "integration" {
		if err := s.connectionOwnedBy(ctx, accountID, *p.OriginConnectionID); err != nil {
			return err
		}
	}
	for _, b := range p.Branches {
		if b.Destination == "contract" {
			var pubID int64
			err := s.pool.QueryRow(ctx,
				`SELECT publisher_id FROM contracts WHERE id=$1 AND deleted_at IS NULL`, *b.ContractID).Scan(&pubID)
			if errors.Is(err, pgx.ErrNoRows) {
				return httpx.Validation("contract not found")
			}
			if err != nil || pubID != accountID {
				return httpx.Validation("contract not found")
			}
		}
		if b.Destination == "pipeline" && b.Delivery == "leads_pipeline" {
			if err := s.pipelineStageOwnedBy(ctx, accountID, *b.TargetPipelineID, *b.TargetStageID); err != nil {
				return err
			}
		}
		if b.Destination == "webhook" {
			if err := s.webhookOwnedBy(ctx, accountID, *b.DestWebhookID); err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *Service) pipelineStageOwnedBy(ctx context.Context, accountID, pipelineID, stageID int64) error {
	var ok bool
	err := s.pool.QueryRow(ctx,
		`SELECT EXISTS(
			SELECT 1 FROM pipeline_stages ps
			JOIN pipelines p ON p.id = ps.pipeline_id
			WHERE ps.id=$1 AND ps.pipeline_id=$2 AND p.account_id=$3)`,
		stageID, pipelineID, accountID).Scan(&ok)
	if err != nil {
		return err
	}
	if !ok {
		return httpx.Validation("pipeline stage not found")
	}
	return nil
}

func (s *Service) webhookOwnedBy(ctx context.Context, accountID, webhookID int64) error {
	var ok bool
	err := s.pool.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM webhooks WHERE id=$1 AND account_id=$2)`, webhookID, accountID).Scan(&ok)
	if err != nil {
		return err
	}
	if !ok {
		return httpx.Validation("webhook not found")
	}
	return nil
}

func (s *Service) connectionOwnedBy(ctx context.Context, accountID, connectionID int64) error {
	var ok bool
	err := s.pool.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM integration_connections WHERE id=$1 AND account_id=$2)`, connectionID, accountID).Scan(&ok)
	if err != nil {
		return err
	}
	if !ok {
		return httpx.Validation("integration connection not found")
	}
	return nil
}

func (s *Service) ListRoutes(ctx context.Context, publisherID int64) ([]Route, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT `+routeCols+routeFrom+`
		 WHERE r.publisher_id=$1 ORDER BY r.name`, publisherID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanRouteList(rows)
}

func (s *Service) ListRoutesForBuyer(ctx context.Context, buyerID int64) ([]Route, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT `+routeCols+routeFrom+`
		 WHERE (r.destination='contract' AND c.buyer_id=$1 AND c.deleted_at IS NULL)
		    OR r.buyer_id=$1
		 ORDER BY r.name`, buyerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanRouteList(rows)
}

func scanRouteList(rows pgx.Rows) ([]Route, error) {
	var out []Route
	for rows.Next() {
		rt, err := scanRoute(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *rt)
	}
	return out, rows.Err()
}

const routeInsertCols = `publisher_id, buyer_id, name, origin, source_id, origin_pipeline_id, origin_stage_id,
	origin_webhook_id, origin_connection_id, destination, contract_id, compensation_id, delivery,
	target_pipeline_id, target_stage_id, dest_webhook_id, branches`

func (s *Service) insertRoute(ctx context.Context, publisherID, buyerID *int64, p CreateRouteParams) (*Route, error) {
	if err := normalizeRouteBranches(&p); err != nil {
		return nil, err
	}
	branchesJSON, err := branchesToJSON(p.Branches)
	if err != nil {
		return nil, err
	}
	var id int64
	err = s.pool.QueryRow(ctx,
		`INSERT INTO routes(`+routeInsertCols+`)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17) RETURNING id`,
		publisherID, buyerID, p.Name, p.Origin, p.SourceID, p.OriginPipelineID, p.OriginStageID,
		p.OriginWebhookID, p.OriginConnectionID, p.Destination, p.ContractID, p.CompensationID, p.Delivery,
		p.TargetPipelineID, p.TargetStageID, p.DestWebhookID, branchesJSON).Scan(&id)
	if err != nil {
		return nil, err
	}
	return s.GetRouteByID(ctx, id)
}

func (s *Service) CreateRoute(ctx context.Context, publisherID int64, p CreateRouteParams) (*Route, error) {
	if err := s.validateRouteOwnership(ctx, publisherID, true, p); err != nil {
		return nil, err
	}
	pid := publisherID
	return s.insertRoute(ctx, &pid, nil, p)
}

func (s *Service) CreateBuyerRoute(ctx context.Context, buyerID int64, p CreateRouteParams) (*Route, error) {
	if err := s.validateRouteOwnership(ctx, buyerID, false, p); err != nil {
		return nil, err
	}
	bid := buyerID
	return s.insertRoute(ctx, nil, &bid, p)
}

func (s *Service) GetRouteByID(ctx context.Context, id int64) (*Route, error) {
	return scanRoute(s.pool.QueryRow(ctx, `SELECT `+routeCols+routeFrom+` WHERE r.id=$1`, id))
}

func (s *Service) GetRoute(ctx context.Context, publisherID, id int64) (*Route, error) {
	return scanRoute(s.pool.QueryRow(ctx,
		`SELECT `+routeCols+routeFrom+`
		 WHERE r.id=$1 AND r.publisher_id=$2`, id, publisherID))
}

func (s *Service) GetBuyerRoute(ctx context.Context, buyerID, id int64) (*Route, error) {
	return scanRoute(s.pool.QueryRow(ctx,
		`SELECT `+routeCols+routeFrom+`
		 WHERE r.id=$1 AND r.buyer_id=$2`, id, buyerID))
}

func (s *Service) UpdateRoute(ctx context.Context, publisherID, id int64, p UpdateRouteParams) (*Route, error) {
	cur, err := s.GetRoute(ctx, publisherID, id)
	if err != nil {
		return nil, err
	}
	merged := mergeRouteParams(cur, p)
	if err := s.validateRouteOwnership(ctx, publisherID, true, merged); err != nil {
		return nil, err
	}
	return s.applyRouteUpdate(ctx, id, &publisherID, nil, cur.Name, p, merged)
}

func (s *Service) UpdateBuyerRoute(ctx context.Context, buyerID, id int64, p UpdateRouteParams) (*Route, error) {
	cur, err := s.GetBuyerRoute(ctx, buyerID, id)
	if err != nil {
		return nil, err
	}
	merged := mergeRouteParams(cur, p)
	if err := s.validateRouteOwnership(ctx, buyerID, false, merged); err != nil {
		return nil, err
	}
	return s.applyRouteUpdate(ctx, id, nil, &buyerID, cur.Name, p, merged)
}

func mergeRouteParams(cur *Route, p UpdateRouteParams) CreateRouteParams {
	branches, _ := parseRouteBranches(cur.Branches)
	merged := CreateRouteParams{
		Name:               cur.Name,
		Origin:             cur.Origin,
		SourceID:           cur.SourceID,
		OriginPipelineID:   cur.OriginPipelineID,
		OriginStageID:      cur.OriginStageID,
		OriginWebhookID:    cur.OriginWebhookID,
		OriginConnectionID: cur.OriginConnectionID,
		Destination:        cur.Destination,
		ContractID:         cur.ContractID,
		CompensationID:     cur.CompensationID,
		Delivery:           cur.Delivery,
		TargetPipelineID:   cur.TargetPipelineID,
		TargetStageID:      cur.TargetStageID,
		DestWebhookID:      cur.DestWebhookID,
		Branches:           branches,
	}
	if p.SourceID != nil {
		merged.SourceID = p.SourceID
	}
	if p.OriginPipelineID != nil {
		merged.OriginPipelineID = p.OriginPipelineID
	}
	if p.OriginStageID != nil {
		merged.OriginStageID = p.OriginStageID
	}
	if p.OriginWebhookID != nil {
		merged.OriginWebhookID = p.OriginWebhookID
	}
	if p.OriginConnectionID != nil {
		merged.OriginConnectionID = p.OriginConnectionID
	}
	if p.Destination != nil {
		merged.Destination = *p.Destination
	}
	if p.ContractID != nil {
		merged.ContractID = p.ContractID
	}
	if p.CompensationID != nil {
		merged.CompensationID = p.CompensationID
	}
	if p.Delivery != nil {
		merged.Delivery = *p.Delivery
	}
	if p.TargetPipelineID != nil {
		merged.TargetPipelineID = p.TargetPipelineID
	}
	if p.TargetStageID != nil {
		merged.TargetStageID = p.TargetStageID
	}
	if p.DestWebhookID != nil {
		merged.DestWebhookID = p.DestWebhookID
	}
	if p.Branches != nil {
		merged.Branches = *p.Branches
	}
	return merged
}

func (s *Service) applyRouteUpdate(ctx context.Context, id int64, publisherID, buyerID *int64, curName string, p UpdateRouteParams, merged CreateRouteParams) (*Route, error) {
	name := curName
	if p.Name != nil {
		name = *p.Name
	}
	if err := normalizeRouteBranches(&merged); err != nil {
		return nil, err
	}
	branchesJSON, err := branchesToJSON(merged.Branches)
	if err != nil {
		return nil, err
	}
	var ownerClause string
	var ownerArg int64
	if publisherID != nil {
		ownerClause = "publisher_id=$2"
		ownerArg = *publisherID
	} else {
		ownerClause = "buyer_id=$2"
		ownerArg = *buyerID
	}
	_, err = s.pool.Exec(ctx,
		`UPDATE routes SET
		   name = $3,
		   source_id = COALESCE($4, source_id),
		   origin_pipeline_id = COALESCE($5, origin_pipeline_id),
		   origin_stage_id = COALESCE($6, origin_stage_id),
		   origin_webhook_id = COALESCE($7, origin_webhook_id),
		   origin_connection_id = COALESCE($8, origin_connection_id),
		   destination = $9,
		   contract_id = $10,
		   compensation_id = $11,
		   delivery = $12,
		   target_pipeline_id = $13,
		   target_stage_id = $14,
		   dest_webhook_id = $15,
		   branches = $16,
		   is_active = COALESCE($17, is_active)
		 WHERE id=$1 AND `+ownerClause,
		id, ownerArg, name, p.SourceID, p.OriginPipelineID, p.OriginStageID,
		p.OriginWebhookID, p.OriginConnectionID, merged.Destination, merged.ContractID, merged.CompensationID, merged.Delivery,
		merged.TargetPipelineID, merged.TargetStageID, merged.DestWebhookID, branchesJSON, p.IsActive)
	if err != nil {
		return nil, err
	}
	return s.GetRouteByID(ctx, id)
}

func (s *Service) DeleteRoute(ctx context.Context, publisherID, id int64) error {
	ct, err := s.pool.Exec(ctx, `DELETE FROM routes WHERE id=$1 AND publisher_id=$2`, id, publisherID)
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return httpx.NotFound("route not found")
	}
	return nil
}

func (s *Service) DeleteBuyerRoute(ctx context.Context, buyerID, id int64) error {
	ct, err := s.pool.Exec(ctx, `DELETE FROM routes WHERE id=$1 AND buyer_id=$2`, id, buyerID)
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return httpx.NotFound("route not found")
	}
	return nil
}

func (s *Service) RouteOwnedBy(ctx context.Context, publisherID, routeID int64) (bool, error) {
	var ok bool
	err := s.pool.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM routes WHERE id=$1 AND publisher_id=$2)`, routeID, publisherID).Scan(&ok)
	return ok, err
}

func (s *Service) RouteOwnedByBuyer(ctx context.Context, buyerID, routeID int64) (bool, error) {
	var ok bool
	err := s.pool.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM routes WHERE id=$1 AND buyer_id=$2)`, routeID, buyerID).Scan(&ok)
	return ok, err
}

func (s *Service) ListRouteFieldMap(ctx context.Context, routeID int64) ([]RouteFieldMapEntry, error) {
	return RouteFieldMap(ctx, s.pool, routeID)
}

func (s *Service) RouteFieldMapOptions(ctx context.Context, publisherID, routeID int64) (*RouteFieldMapOptions, error) {
	rt, err := s.GetRoute(ctx, publisherID, routeID)
	if err != nil {
		return nil, err
	}
	if rt.Destination != "contract" {
		return nil, httpx.Validation("field map is only for contract routes")
	}
	if rt.ContractID == nil {
		return nil, httpx.Validation("route missing contract")
	}
	var buyerID int64
	var buyerName string
	err = s.pool.QueryRow(ctx,
		`SELECT c.buyer_id, a.name FROM contracts c
		 JOIN accounts a ON a.id = c.buyer_id
		 WHERE c.id = $1 AND c.publisher_id = $2 AND c.deleted_at IS NULL`,
		*rt.ContractID, publisherID).Scan(&buyerID, &buyerName)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, httpx.NotFound("contract not found")
	}
	if err != nil {
		return nil, err
	}
	if rt.BuyerName != nil && *rt.BuyerName != "" {
		buyerName = *rt.BuyerName
	}
	cf := customfields.NewService(s.pool)
	pubFields, err := cf.ListFields(ctx, publisherID)
	if err != nil {
		return nil, err
	}
	buyerFields, err := cf.ListFields(ctx, buyerID)
	if err != nil {
		return nil, err
	}
	if pubFields == nil {
		pubFields = []customfields.CustomField{}
	}
	if buyerFields == nil {
		buyerFields = []customfields.CustomField{}
	}
	return &RouteFieldMapOptions{
		BuyerName:       buyerName,
		PublisherFields: pubFields,
		BuyerFields:     buyerFields,
	}, nil
}

func (s *Service) customFieldOwnedBy(ctx context.Context, fieldID, accountID int64) error {
	var ok bool
	err := s.pool.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM custom_fields WHERE id=$1 AND account_id=$2)`, fieldID, accountID).Scan(&ok)
	if err != nil {
		return err
	}
	if !ok {
		return httpx.Validation("custom field does not belong to the expected account")
	}
	return nil
}

func (s *Service) AddRouteFieldMap(ctx context.Context, publisherID, routeID int64, srcType string, srcBuiltin *string, srcCustomID *int64, dstType string, dstBuiltin *string, dstCustomID *int64) (*RouteFieldMapEntry, error) {
	if srcType != "builtin" && srcType != "custom" {
		return nil, httpx.Validation("src_type must be builtin or custom")
	}
	if dstType != "builtin" && dstType != "custom" {
		return nil, httpx.Validation("dst_type must be builtin or custom")
	}
	rt, err := s.GetRoute(ctx, publisherID, routeID)
	if err != nil {
		return nil, err
	}
	if rt.Destination != "contract" {
		return nil, httpx.Validation("field map is only for contract routes")
	}
	if rt.ContractID == nil {
		return nil, httpx.Validation("route missing contract")
	}
	var buyerID int64
	err = s.pool.QueryRow(ctx,
		`SELECT buyer_id FROM contracts WHERE id=$1 AND publisher_id=$2 AND deleted_at IS NULL`,
		*rt.ContractID, publisherID).Scan(&buyerID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, httpx.NotFound("contract not found")
	}
	if err != nil {
		return nil, err
	}
	if srcType == "custom" {
		if srcCustomID == nil || *srcCustomID == 0 {
			return nil, httpx.Validation("src_custom_field_id required for custom src")
		}
		if err := s.customFieldOwnedBy(ctx, *srcCustomID, publisherID); err != nil {
			return nil, err
		}
	}
	if dstType == "custom" {
		if dstCustomID == nil || *dstCustomID == 0 {
			return nil, httpx.Validation("dst_custom_field_id required for custom dst")
		}
		if err := s.customFieldOwnedBy(ctx, *dstCustomID, buyerID); err != nil {
			return nil, err
		}
	}
	e := &RouteFieldMapEntry{}
	err = s.pool.QueryRow(ctx,
		`INSERT INTO route_field_map(route_id, src_type, src_builtin, src_custom_field_id, dst_type, dst_builtin, dst_custom_field_id)
		 VALUES ($1,$2,$3,$4,$5,$6,$7)
		 RETURNING id, route_id, src_type, src_builtin, src_custom_field_id, dst_type, dst_builtin, dst_custom_field_id, created_at`,
		routeID, srcType, srcBuiltin, srcCustomID, dstType, dstBuiltin, dstCustomID).Scan(
		&e.ID, &e.RouteID, &e.SrcType, &e.SrcBuiltin, &e.SrcCustomFieldID,
		&e.DstType, &e.DstBuiltin, &e.DstCustomFieldID, &e.CreatedAt)
	if err != nil {
		if database.IsCheckViolation(err) {
			return nil, httpx.Validation("provide builtin or custom field id for each side of the map")
		}
		return nil, err
	}
	if e.SrcType == "builtin" && e.SrcBuiltin != nil {
		e.SrcLabel = e.SrcBuiltin
	} else if e.SrcCustomFieldID != nil {
		var name string
		if err := s.pool.QueryRow(ctx, `SELECT name FROM custom_fields WHERE id=$1`, *e.SrcCustomFieldID).Scan(&name); err == nil {
			e.SrcLabel = &name
		}
	}
	if e.DstType == "builtin" && e.DstBuiltin != nil {
		e.DstLabel = e.DstBuiltin
	} else if e.DstCustomFieldID != nil {
		var name string
		if err := s.pool.QueryRow(ctx, `SELECT name FROM custom_fields WHERE id=$1`, *e.DstCustomFieldID).Scan(&name); err == nil {
			e.DstLabel = &name
		}
	}
	return e, nil
}

func (s *Service) DeleteRouteFieldMap(ctx context.Context, id int64) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM route_field_map WHERE id=$1`, id)
	return err
}

func (s *Service) CreateBuyerCustomField(ctx context.Context, publisherID, routeID int64, name, fieldKey, ftype string, options json.RawMessage) (*customfields.CustomField, error) {
	rt, err := s.GetRoute(ctx, publisherID, routeID)
	if err != nil {
		return nil, err
	}
	if rt.Destination != "contract" {
		return nil, httpx.Validation("field map is only for contract routes")
	}
	if rt.ContractID == nil {
		return nil, httpx.Validation("route missing contract")
	}
	var buyerID int64
	err = s.pool.QueryRow(ctx,
		`SELECT buyer_id FROM contracts WHERE id=$1 AND publisher_id=$2 AND deleted_at IS NULL`,
		*rt.ContractID, publisherID).Scan(&buyerID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, httpx.NotFound("contract not found")
	}
	if err != nil {
		return nil, err
	}
	cf := customfields.NewService(s.pool)
	return cf.CreateField(ctx, buyerID, name, fieldKey, ftype, options, nil)
}
