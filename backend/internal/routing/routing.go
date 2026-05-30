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
	ID          int64     `json:"id"`
	PublisherID int64     `json:"-"`
	Name        string    `json:"name"`
	Slug        string    `json:"slug"`
	IsActive    bool      `json:"is_active"`
	CreatedAt   time.Time `json:"created_at"`
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
	ID                 int64     `json:"id"`
	PublisherID        int64     `json:"-"`
	Name               string    `json:"name"`
	Origin             string    `json:"origin"`
	SourceID           *int64    `json:"source_id"`
	SourceName         *string   `json:"source_name,omitempty"`
	OriginPipelineID   *int64    `json:"origin_pipeline_id"`
	OriginStageID      *int64    `json:"origin_stage_id"`
	OriginPipelineName *string   `json:"origin_pipeline_name,omitempty"`
	OriginStageName    *string   `json:"origin_stage_name,omitempty"`
	Destination        string    `json:"destination"`
	ContractID         *int64    `json:"contract_id"`
	ContractName       *string   `json:"contract_name,omitempty"`
	BuyerName          *string   `json:"buyer_name,omitempty"`
	Delivery           string    `json:"delivery"`
	TargetPipelineID   *int64    `json:"target_pipeline_id"`
	TargetStageID      *int64    `json:"target_stage_id"`
	TargetPipelineName *string   `json:"target_pipeline_name,omitempty"`
	TargetStageName    *string   `json:"target_stage_name,omitempty"`
	IsActive           bool      `json:"is_active"`
	CreatedAt          time.Time `json:"created_at"`
}

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
	Name             string `json:"name"`
	Origin           string `json:"origin"`
	SourceID         *int64 `json:"source_id"`
	OriginPipelineID *int64 `json:"origin_pipeline_id"`
	OriginStageID    *int64 `json:"origin_stage_id"`
	Destination      string `json:"destination"`
	ContractID       *int64 `json:"contract_id"`
	Delivery         string `json:"delivery"`
	TargetPipelineID *int64 `json:"target_pipeline_id"`
	TargetStageID    *int64 `json:"target_stage_id"`
}

type UpdateRouteParams struct {
	Name             *string `json:"name"`
	IsActive         *bool   `json:"is_active"`
	SourceID         *int64  `json:"source_id"`
	OriginPipelineID *int64  `json:"origin_pipeline_id"`
	OriginStageID    *int64  `json:"origin_stage_id"`
	Destination      *string `json:"destination"`
	ContractID       *int64  `json:"contract_id"`
	Delivery         *string `json:"delivery"`
	TargetPipelineID *int64  `json:"target_pipeline_id"`
	TargetStageID    *int64  `json:"target_stage_id"`
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
 LEFT JOIN pipelines tp ON tp.id = r.target_pipeline_id
 LEFT JOIN pipeline_stages ts ON ts.id = r.target_stage_id AND r.destination = 'publisher'
 LEFT JOIN pipelines bp ON bp.id = c.buyer_pipeline_id
 LEFT JOIN pipeline_stages bs ON bs.id = r.target_stage_id AND r.destination = 'buyer'`

const routeCols = `r.id, r.publisher_id, r.name, r.origin, r.source_id, s.name,
	r.origin_pipeline_id, r.origin_stage_id, r.destination, r.contract_id, c.name,
	r.delivery, r.target_pipeline_id, r.target_stage_id, r.is_active, r.created_at,
	ba.name, op.name, os.name,
	CASE WHEN r.destination = 'publisher' THEN tp.name ELSE bp.name END,
	CASE WHEN r.destination = 'publisher' THEN ts.name ELSE bs.name END`

func scanRoute(row pgx.Row) (*Route, error) {
	rt := &Route{}
	err := row.Scan(&rt.ID, &rt.PublisherID, &rt.Name, &rt.Origin, &rt.SourceID, &rt.SourceName,
		&rt.OriginPipelineID, &rt.OriginStageID, &rt.Destination, &rt.ContractID, &rt.ContractName,
		&rt.Delivery, &rt.TargetPipelineID, &rt.TargetStageID, &rt.IsActive, &rt.CreatedAt,
		&rt.BuyerName, &rt.OriginPipelineName, &rt.OriginStageName, &rt.TargetPipelineName, &rt.TargetStageName)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, httpx.NotFound("route not found")
		}
		return nil, err
	}
	return rt, nil
}

// MatchSourceBySlug finds an active source by slug. nil means no match.
func MatchSourceBySlug(ctx context.Context, q database.Querier, publisherID int64, slug string) (*Source, error) {
	if slug == "" {
		return nil, nil
	}
	s := &Source{}
	err := q.QueryRow(ctx,
		`SELECT id, publisher_id, name, slug, is_active, created_at
		 FROM routing_sources WHERE publisher_id=$1 AND slug=$2 AND is_active`,
		publisherID, slug).Scan(&s.ID, &s.PublisherID, &s.Name, &s.Slug, &s.IsActive, &s.CreatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return s, nil
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

// RouteForSource finds the active route whose origin is the given source.
func RouteForSource(ctx context.Context, q database.Querier, sourceID int64) (*Route, error) {
	return scanRouteOptional(q.QueryRow(ctx,
		`SELECT `+routeCols+routeFrom+`
		 WHERE r.source_id=$1 AND r.origin='source' AND r.is_active`, sourceID))
}

// MatchRouteByStage finds an active pipeline-origin route for a publisher trigger stage.
func MatchRouteByStage(ctx context.Context, q database.Querier, publisherID, stageID int64) (*Route, error) {
	return scanRouteOptional(q.QueryRow(ctx,
		`SELECT `+routeCols+routeFrom+`
		 WHERE r.publisher_id=$1 AND r.origin='pipeline' AND r.origin_stage_id=$2 AND r.is_active`,
		publisherID, stageID))
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
		`SELECT id, publisher_id, name, slug, is_active, created_at
		 FROM routing_sources WHERE publisher_id=$1 ORDER BY name`, publisherID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Source
	for rows.Next() {
		var src Source
		if err := rows.Scan(&src.ID, &src.PublisherID, &src.Name, &src.Slug, &src.IsActive, &src.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, src)
	}
	return out, rows.Err()
}

func (s *Service) CreateSource(ctx context.Context, publisherID int64, name, slug string) (*Source, error) {
	src := &Source{}
	err := s.pool.QueryRow(ctx,
		`INSERT INTO routing_sources(publisher_id, name, slug) VALUES ($1,$2,$3)
		 RETURNING id, publisher_id, name, slug, is_active, created_at`,
		publisherID, name, slug).Scan(&src.ID, &src.PublisherID, &src.Name, &src.Slug, &src.IsActive, &src.CreatedAt)
	if err != nil {
		if database.IsUniqueViolation(err) {
			return nil, httpx.Conflict("a source with this slug already exists")
		}
		return nil, err
	}
	return src, nil
}

func (s *Service) UpdateSource(ctx context.Context, publisherID, id int64, name, slug *string, isActive *bool) (*Source, error) {
	src := &Source{}
	err := s.pool.QueryRow(ctx,
		`UPDATE routing_sources SET
		   name = COALESCE($3, name),
		   slug = COALESCE($4, slug),
		   is_active = COALESCE($5, is_active)
		 WHERE id=$1 AND publisher_id=$2
		 RETURNING id, publisher_id, name, slug, is_active, created_at`,
		id, publisherID, name, slug, isActive).Scan(
		&src.ID, &src.PublisherID, &src.Name, &src.Slug, &src.IsActive, &src.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
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
		 JOIN routing_sources s ON s.slug = l.campaign_name AND s.publisher_id = l.publisher_id
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

func (s *Service) AddSourceFieldMap(ctx context.Context, sourceID int64, sourceKey, targetType string, builtinField *string, customFieldID *int64) (*SourceFieldMapEntry, error) {
	if targetType != "builtin" && targetType != "custom" {
		return nil, httpx.Validation("target_type must be builtin or custom")
	}
	e := &SourceFieldMapEntry{}
	err := s.pool.QueryRow(ctx,
		`INSERT INTO routing_source_field_map(source_id, source_key, target_type, builtin_field, custom_field_id)
		 VALUES ($1,$2,$3,$4,$5)
		 RETURNING id, source_id, source_key, target_type, builtin_field, custom_field_id, created_at`,
		sourceID, sourceKey, targetType, builtinField, customFieldID).Scan(
		&e.ID, &e.SourceID, &e.SourceKey, &e.TargetType, &e.BuiltinField, &e.CustomFieldID, &e.CreatedAt)
	if err != nil {
		if database.IsCheckViolation(err) {
			return nil, httpx.Validation("provide builtin_field for builtin target, or custom_field_id for custom target")
		}
		return nil, err
	}
	return e, nil
}

func (s *Service) DeleteSourceFieldMap(ctx context.Context, id int64) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM routing_source_field_map WHERE id=$1`, id)
	return err
}

func validateRouteParams(p CreateRouteParams) error {
	if p.Origin != "source" && p.Origin != "pipeline" {
		return httpx.Validation("origin must be source or pipeline")
	}
	if p.Destination != "publisher" && p.Destination != "buyer" {
		return httpx.Validation("destination must be publisher or buyer")
	}
	if p.Delivery != "leads" && p.Delivery != "leads_pipeline" {
		return httpx.Validation("delivery must be leads or leads_pipeline")
	}
	if p.Origin == "source" {
		if p.SourceID == nil || *p.SourceID == 0 {
			return httpx.Validation("source_id is required for source origin")
		}
	} else {
		if p.OriginPipelineID == nil || p.OriginStageID == nil {
			return httpx.Validation("origin_pipeline_id and origin_stage_id are required for pipeline origin")
		}
		if p.Destination == "publisher" {
			return httpx.Validation("pipeline origin routes must target a buyer")
		}
	}
	if p.Destination == "buyer" {
		if p.ContractID == nil || *p.ContractID == 0 {
			return httpx.Validation("contract_id is required for buyer destination")
		}
	}
	if p.Destination == "publisher" && p.Delivery == "leads_pipeline" {
		if p.TargetPipelineID == nil || p.TargetStageID == nil {
			return httpx.Validation("target_pipeline_id and target_stage_id are required for publisher pipeline delivery")
		}
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

func (s *Service) CreateRoute(ctx context.Context, publisherID int64, p CreateRouteParams) (*Route, error) {
	if p.Delivery == "" {
		p.Delivery = "leads_pipeline"
	}
	if err := validateRouteParams(p); err != nil {
		return nil, err
	}
	var id int64
	err := s.pool.QueryRow(ctx,
		`INSERT INTO routes(publisher_id, name, origin, source_id, origin_pipeline_id, origin_stage_id,
		    destination, contract_id, delivery, target_pipeline_id, target_stage_id)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11) RETURNING id`,
		publisherID, p.Name, p.Origin, p.SourceID, p.OriginPipelineID, p.OriginStageID,
		p.Destination, p.ContractID, p.Delivery, p.TargetPipelineID, p.TargetStageID).Scan(&id)
	if err != nil {
		return nil, err
	}
	return s.GetRoute(ctx, publisherID, id)
}

func (s *Service) GetRoute(ctx context.Context, publisherID, id int64) (*Route, error) {
	return scanRoute(s.pool.QueryRow(ctx,
		`SELECT `+routeCols+routeFrom+`
		 WHERE r.id=$1 AND r.publisher_id=$2`, id, publisherID))
}

func (s *Service) UpdateRoute(ctx context.Context, publisherID, id int64, p UpdateRouteParams) (*Route, error) {
	cur, err := scanRoute(s.pool.QueryRow(ctx,
		`SELECT `+routeCols+routeFrom+`
		 WHERE r.id=$1 AND r.publisher_id=$2`, id, publisherID))
	if err != nil {
		return nil, err
	}
	merged := CreateRouteParams{
		Name:             cur.Name,
		Origin:           cur.Origin,
		SourceID:         cur.SourceID,
		OriginPipelineID: cur.OriginPipelineID,
		OriginStageID:    cur.OriginStageID,
		Destination:      cur.Destination,
		ContractID:       cur.ContractID,
		Delivery:         cur.Delivery,
		TargetPipelineID: cur.TargetPipelineID,
		TargetStageID:    cur.TargetStageID,
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
	if p.Destination != nil {
		merged.Destination = *p.Destination
	}
	if p.ContractID != nil {
		merged.ContractID = p.ContractID
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
	if err := validateRouteParams(merged); err != nil {
		return nil, err
	}
	name := cur.Name
	if p.Name != nil {
		name = *p.Name
	}
	_, err = s.pool.Exec(ctx,
		`UPDATE routes SET
		   name = $3,
		   source_id = COALESCE($4, source_id),
		   origin_pipeline_id = COALESCE($5, origin_pipeline_id),
		   origin_stage_id = COALESCE($6, origin_stage_id),
		   destination = COALESCE($7, destination),
		   contract_id = COALESCE($8, contract_id),
		   delivery = COALESCE($9, delivery),
		   target_pipeline_id = COALESCE($10, target_pipeline_id),
		   target_stage_id = COALESCE($11, target_stage_id),
		   is_active = COALESCE($12, is_active)
		 WHERE id=$1 AND publisher_id=$2`,
		id, publisherID, name, p.SourceID, p.OriginPipelineID, p.OriginStageID,
		p.Destination, p.ContractID, p.Delivery, p.TargetPipelineID, p.TargetStageID, p.IsActive)
	if err != nil {
		return nil, err
	}
	return s.GetRoute(ctx, publisherID, id)
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

func (s *Service) RouteOwnedBy(ctx context.Context, publisherID, routeID int64) (bool, error) {
	var ok bool
	err := s.pool.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM routes WHERE id=$1 AND publisher_id=$2)`, routeID, publisherID).Scan(&ok)
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
	if rt.Destination != "buyer" {
		return nil, httpx.Validation("field map is only for buyer routes")
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
	if rt.Destination != "buyer" {
		return nil, httpx.Validation("field map is only for buyer routes")
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
