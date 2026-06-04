// Package intake handles the public lead ingest flow and the publisher's
// pre-routing intake queue.
package intake

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/echayko/leadrula/backend/internal/accounts"
	"github.com/echayko/leadrula/backend/internal/billing"
	"github.com/echayko/leadrula/backend/internal/contracts"
	"github.com/echayko/leadrula/backend/internal/database"
	"github.com/echayko/leadrula/backend/internal/leads"
	"github.com/echayko/leadrula/backend/internal/notifications"
	"github.com/echayko/leadrula/backend/internal/routing"
	"github.com/echayko/leadrula/backend/pkg/httpx"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Service struct {
	pool         *pgxpool.Pool
	leads        *leads.Repository
	notif        *notifications.Service
	accounts     *accounts.Repository
	integrations leads.IntegrationEnqueuer
}

func NewService(pool *pgxpool.Pool, leadRepo *leads.Repository, notif *notifications.Service, acc *accounts.Repository, integrations leads.IntegrationEnqueuer) *Service {
	return &Service{pool: pool, leads: leadRepo, notif: notif, accounts: acc, integrations: integrations}
}

func (s *Service) routeDeps() leads.RouteApplyDeps {
	return leads.RouteApplyDeps{Repo: s.leads, Accounts: s.accounts, Notif: s.notif, Integrations: s.integrations}
}

// IngestResult is returned to the API caller.
type IngestResult struct {
	LeadID string `json:"lead_id"`
	Status string `json:"status"`
}

var builtinKeys = []string{"first_name", "last_name", "phone", "email", "address", "city", "state", "zip"}

func flattenPayload(raw map[string]any) map[string]any {
	sources := map[string]any{}
	for k, v := range raw {
		if k == "custom" {
			continue
		}
		sources[k] = v
	}
	if custom, ok := raw["custom"].(map[string]any); ok {
		for k, v := range custom {
			sources[k] = v
		}
	}
	return sources
}

func resolveIngestSource(raw map[string]any) string {
	if s, ok := raw["source"].(string); ok && s != "" {
		return s
	}
	if s, ok := raw["campaign_name"].(string); ok {
		return s
	}
	return ""
}

// Ingest lands legacy POST /api/v1/leads payloads in the intake queue.
func (s *Service) Ingest(ctx context.Context, publisherID int64, raw map[string]any) (*IngestResult, error) {
	source := resolveIngestSource(raw)
	rawJSON, _ := json.Marshal(raw)

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	leadID, publicID, err := s.leads.InsertLead(ctx, tx, publisherID, publisherID, source, rawJSON)
	if err != nil {
		return nil, err
	}
	sources := flattenPayload(raw)
	for _, k := range builtinKeys {
		if v, ok := sources[k]; ok {
			if str := toText(v); str != "" {
				if err := s.leads.SetBuiltinField(ctx, tx, leadID, k, str); err != nil {
					return nil, err
				}
			}
		}
	}
	if _, err := tx.Exec(ctx,
		`INSERT INTO lead_intake_queue(lead_id, raw_payload, source) VALUES ($1,$2,$3)`,
		leadID, rawJSON, nullStr(source)); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return &IngestResult{LeadID: publicID, Status: "review"}, nil
}

// IngestFromSource handles POST /api/v1/sources/{slug}.
func (s *Service) IngestFromSource(ctx context.Context, publisherID int64, slug string, raw map[string]any) (*IngestResult, error) {
	rawJSON, _ := json.Marshal(raw)
	sources := flattenPayload(raw)

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	src, err := routing.MatchSourceBySlug(ctx, tx, publisherID, slug)
	if err != nil {
		return nil, err
	}
	if src == nil {
		return nil, httpx.NotFound("source not found")
	}

	leadID, publicID, err := s.leads.InsertLead(ctx, tx, publisherID, publisherID, slug, rawJSON)
	if err != nil {
		return nil, err
	}
	for _, k := range builtinKeys {
		if v, ok := sources[k]; ok {
			if str := toText(v); str != "" {
				if err := s.leads.SetBuiltinField(ctx, tx, leadID, k, str); err != nil {
					return nil, err
				}
			}
		}
	}
	maps, err := routing.SourceFieldMap(ctx, tx, src.ID)
	if err != nil {
		return nil, err
	}
	for _, m := range maps {
		v, ok := sources[m.SourceKey]
		if !ok {
			continue
		}
		if m.TargetType == "builtin" && m.BuiltinField != nil {
			if err := s.leads.SetBuiltinField(ctx, tx, leadID, *m.BuiltinField, toText(v)); err != nil {
				return nil, err
			}
		} else if m.TargetType == "custom" && m.CustomFieldID != nil {
			valJSON, _ := json.Marshal(v)
			if err := s.leads.UpsertCustomValue(ctx, tx, leadID, *m.CustomFieldID, valJSON); err != nil {
				return nil, err
			}
		}
	}

	rt, err := routing.RouteForSource(ctx, tx, src.ID)
	if err != nil {
		return nil, err
	}
	if rt == nil {
		if _, err := tx.Exec(ctx,
			`INSERT INTO lead_intake_queue(lead_id, raw_payload, source) VALUES ($1,$2,$3)`,
			leadID, rawJSON, slug); err != nil {
			return nil, err
		}
		if err := tx.Commit(ctx); err != nil {
			return nil, err
		}
		return &IngestResult{LeadID: publicID, Status: "review"}, nil
	}

	if err := leads.ApplyRoute(ctx, tx, s.routeDeps(), rt, publisherID, leadID); err != nil {
		return nil, err
	}
	status := "review"
	if rt.Destination == "buyer" && rt.Delivery == "leads_pipeline" {
		status = "distributed"
	} else if rt.Destination == "publisher" && rt.Delivery == "leads_pipeline" {
		status = "distributed"
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	leads.TryEnqueueIntegrations(ctx, s.leads.Pool(), s.leads, s.integrations, rt.ID, leadID)
	return &IngestResult{LeadID: publicID, Status: status}, nil
}

// SetActionByPublicID sets a lead's action_at via the public API.
func (s *Service) SetActionByPublicID(ctx context.Context, publicID string, actionAt *time.Time) error {
	ct, err := s.pool.Exec(ctx, `UPDATE leads SET action_at=$2 WHERE public_id=$1`, publicID, actionAt)
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return httpx.NotFound("lead not found")
	}
	return nil
}

// ── Intake queue (publisher admin) ────────────────────────────────

type QueueItem struct {
	ID           int64           `json:"id"`
	LeadID       int64           `json:"lead_id"`
	FirstName    string          `json:"first_name"`
	LastName     string          `json:"last_name"`
	Phone        *string         `json:"phone"`
	Source       *string         `json:"source"`
	RawPayload   json.RawMessage `json:"raw_payload"`
	Status       string          `json:"status"`
	UnmappedKeys []string        `json:"unmapped_keys"`
	CreatedAt    time.Time       `json:"created_at"`
}

var skipPayloadKeys = map[string]bool{
	"source":        true,
	"campaign_name": true,
	"custom":        true,
}

func computeUnmappedKeys(raw map[string]any, maps []routing.SourceFieldMapEntry) []string {
	flat := flattenPayload(raw)
	mapped := map[string]bool{}
	for _, m := range maps {
		mapped[m.SourceKey] = true
	}
	var out []string
	for k := range flat {
		if skipPayloadKeys[k] || mapped[k] {
			continue
		}
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func (s *Service) sourceFieldMaps(ctx context.Context, publisherID int64, slug string) ([]routing.SourceFieldMapEntry, error) {
	if slug == "" {
		return nil, nil
	}
	src, err := routing.MatchSourceBySlug(ctx, s.pool, publisherID, slug)
	if err != nil || src == nil {
		return nil, err
	}
	return routing.SourceFieldMap(ctx, s.pool, src.ID)
}

func (s *Service) enrichQueueItem(ctx context.Context, publisherID int64, it *QueueItem) error {
	var raw map[string]any
	if len(it.RawPayload) > 0 {
		if err := json.Unmarshal(it.RawPayload, &raw); err != nil {
			return err
		}
	}
	slug := ""
	if it.Source != nil {
		slug = *it.Source
	}
	maps, err := s.sourceFieldMaps(ctx, publisherID, slug)
	if err != nil {
		return err
	}
	it.UnmappedKeys = computeUnmappedKeys(raw, maps)
	if it.UnmappedKeys == nil {
		it.UnmappedKeys = []string{}
	}
	return nil
}

type QueueListResponse struct {
	Items []QueueItem `json:"items"`
	Total int64       `json:"total"`
	Page  int         `json:"page"`
	Limit int         `json:"limit"`
}

type ListQueueParams struct {
	Status string
	Page   int
	Limit  int
	Search string
}

func (s *Service) ListQueue(ctx context.Context, publisherID int64, p ListQueueParams) (*QueueListResponse, error) {
	status := p.Status
	if status == "" {
		status = "pending_review"
	}

	where := "1=1"
	args := []any{}
	argN := func(v any) string {
		args = append(args, v)
		return fmt.Sprintf("$%d", len(args))
	}

	if status != "all" {
		where += " AND q.status = " + argN(status) + "::intake_status"
	}
	if p.Search != "" {
		like := "%" + p.Search + "%"
		n := argN(like)
		where += fmt.Sprintf(" AND (l.first_name ILIKE %s OR l.last_name ILIKE %s OR l.phone ILIKE %s OR q.source ILIKE %s)", n, n, n, n)
	}

	from := ` FROM lead_intake_queue q JOIN leads l ON l.id = q.lead_id WHERE ` + where

	var total int64
	if err := s.pool.QueryRow(ctx, `SELECT COUNT(*)`+from, args...).Scan(&total); err != nil {
		return nil, err
	}

	page := p.Page
	limit := p.Limit
	paginate := page > 0 && limit > 0
	if paginate {
		if page < 1 {
			page = 1
		}
	} else {
		page = 1
		if total > 0 {
			limit = int(total)
		}
	}

	selectQ := `SELECT q.id, q.lead_id, l.first_name, l.last_name, l.phone, q.source, q.raw_payload, q.status, q.created_at` + from + ` ORDER BY q.created_at DESC`
	if paginate {
		offset := (page - 1) * limit
		selectQ += fmt.Sprintf(" LIMIT %s OFFSET %s", argN(limit), argN(offset))
	}

	rows, err := s.pool.Query(ctx, selectQ, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []QueueItem
	for rows.Next() {
		var it QueueItem
		if err := rows.Scan(&it.ID, &it.LeadID, &it.FirstName, &it.LastName, &it.Phone, &it.Source, &it.RawPayload, &it.Status, &it.CreatedAt); err != nil {
			return nil, err
		}
		if err := s.enrichQueueItem(ctx, publisherID, &it); err != nil {
			return nil, err
		}
		items = append(items, it)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if items == nil {
		items = []QueueItem{}
	}

	return &QueueListResponse{
		Items: items,
		Total: total,
		Page:  page,
		Limit: limit,
	}, nil
}

const buyerRoutingFrom = `
FROM (
  SELECT DISTINCT ON (t.lead_id) t.lead_id, t.created_at AS routed_at
  FROM transactions t
  WHERE t.buyer_id = $1
    AND t.contract_id IS NOT NULL
    AND t.type = 'debit'
    AND t.lead_id IS NOT NULL
    AND (
      t.description LIKE 'lead routed:%'
      OR t.description = 'lead routed from intake queue'
      OR t.description = 'lead re-distributed'
    )
  ORDER BY t.lead_id, t.created_at DESC
) r
JOIN leads l ON l.id = r.lead_id
LEFT JOIN lead_intake_queue q ON q.lead_id = l.id
WHERE 1=1`

func buyerLogStatus(buyerID int64, leadStatus string, ownerID int64) string {
	if leadStatus == "returned" || ownerID != buyerID {
		return "returned"
	}
	switch leadStatus {
	case "review":
		return "pending_review"
	case "distributed":
		return "routed"
	default:
		return leadStatus
	}
}

func (s *Service) ListRoutingLogForBuyer(ctx context.Context, buyerID int64, p ListQueueParams) (*QueueListResponse, error) {
	status := p.Status
	if status == "" {
		status = "all"
	}

	args := []any{buyerID}
	argN := func(v any) string {
		args = append(args, v)
		return fmt.Sprintf("$%d", len(args))
	}

	where := ""
	switch status {
	case "pending_review":
		where += fmt.Sprintf(" AND l.status = 'review' AND l.owner_account_id = %s", argN(buyerID))
	case "routed":
		where += fmt.Sprintf(" AND l.status = 'distributed' AND l.owner_account_id = %s", argN(buyerID))
	case "rejected":
		where += " AND 1=0"
	}
	if p.Search != "" {
		like := "%" + p.Search + "%"
		n := argN(like)
		where += fmt.Sprintf(" AND (l.first_name ILIKE %s OR l.last_name ILIKE %s OR l.phone ILIKE %s OR COALESCE(q.source, l.source) ILIKE %s)", n, n, n, n)
	}

	from := buyerRoutingFrom + where

	var total int64
	if err := s.pool.QueryRow(ctx, `SELECT COUNT(*)`+from, args...).Scan(&total); err != nil {
		return nil, err
	}

	page := p.Page
	limit := p.Limit
	paginate := page > 0 && limit > 0
	if paginate {
		if page < 1 {
			page = 1
		}
	} else {
		page = 1
		if total > 0 {
			limit = int(total)
		}
	}

	selectQ := `SELECT COALESCE(q.id, l.id), l.id, l.first_name, l.last_name, l.phone,
		COALESCE(q.source, l.source), COALESCE(q.raw_payload, l.raw_payload),
		l.status, l.owner_account_id, l.publisher_id, r.routed_at` + from + ` ORDER BY r.routed_at DESC`
	if paginate {
		offset := (page - 1) * limit
		selectQ += fmt.Sprintf(" LIMIT %s OFFSET %s", argN(limit), argN(offset))
	}

	rows, err := s.pool.Query(ctx, selectQ, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []QueueItem
	for rows.Next() {
		var it QueueItem
		var leadStatus string
		var ownerID, publisherID int64
		if err := rows.Scan(&it.ID, &it.LeadID, &it.FirstName, &it.LastName, &it.Phone, &it.Source, &it.RawPayload, &leadStatus, &ownerID, &publisherID, &it.CreatedAt); err != nil {
			return nil, err
		}
		it.Status = buyerLogStatus(buyerID, leadStatus, ownerID)
		if err := s.enrichQueueItem(ctx, publisherID, &it); err != nil {
			return nil, err
		}
		items = append(items, it)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if items == nil {
		items = []QueueItem{}
	}

	return &QueueListResponse{
		Items: items,
		Total: total,
		Page:  page,
		Limit: limit,
	}, nil
}

func (s *Service) scanQueueItem(ctx context.Context, q database.Querier, publisherID, queueID int64) (*QueueItem, error) {
	var it QueueItem
	err := q.QueryRow(ctx,
		`SELECT q.id, q.lead_id, l.first_name, l.last_name, l.phone, q.source, q.raw_payload, q.status, q.created_at
		 FROM lead_intake_queue q JOIN leads l ON l.id = q.lead_id WHERE q.id=$1`, queueID).
		Scan(&it.ID, &it.LeadID, &it.FirstName, &it.LastName, &it.Phone, &it.Source, &it.RawPayload, &it.Status, &it.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, httpx.NotFound("queue item not found")
	}
	if err != nil {
		return nil, err
	}
	if err := s.enrichQueueItem(ctx, publisherID, &it); err != nil {
		return nil, err
	}
	return &it, nil
}

// RouteFromQueue manually routes a queued lead to a buyer (DB spec §4.1 manual path).
func (s *Service) RouteFromQueue(ctx context.Context, queueID, pipelineID, stageID, buyerID, reviewerID int64) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	var leadID int64
	var status string
	if err := tx.QueryRow(ctx, `SELECT lead_id, status FROM lead_intake_queue WHERE id=$1 FOR UPDATE`, queueID).Scan(&leadID, &status); err != nil {
		return httpx.NotFound("queue item not found")
	}
	if status != "pending_review" {
		return httpx.BusinessRule("queue item already handled")
	}

	var publisherID int64
	var sourceSlug *string
	if err := tx.QueryRow(ctx, `SELECT publisher_id, source FROM leads WHERE id=$1`, leadID).Scan(&publisherID, &sourceSlug); err != nil {
		return err
	}
	if sourceSlug != nil && *sourceSlug != "" {
		src, err := routing.MatchSourceBySlug(ctx, tx, publisherID, *sourceSlug)
		if err != nil {
			return err
		}
		if src != nil {
			rt, err := routing.BuyerRouteForSourceAndBuyer(ctx, tx, publisherID, src.ID, buyerID)
			if err != nil {
				return err
			}
			if rt != nil {
				lead, err := s.leads.GetByID(ctx, tx, leadID)
				if err != nil {
					return err
				}
				if err := leads.LoadCustomValues(ctx, tx, lead); err != nil {
					return err
				}
				maps, err := routing.RouteFieldMap(ctx, tx, rt.ID)
				if err != nil {
					return err
				}
				if err := leads.ApplyRouteFieldMap(ctx, tx, s.leads, lead, maps); err != nil {
					return err
				}
			}
		}
	}

	target, err := contracts.FindByBuyer(ctx, tx, buyerID)
	if err != nil {
		return err
	}
	if target == nil {
		return httpx.BusinessRule("selected buyer has no active contract")
	}
	if pipelineID == 0 {
		pipelineID = target.BuyerPipelineID
	}
	if stageID == 0 {
		if err := tx.QueryRow(ctx,
			`SELECT id FROM pipeline_stages WHERE pipeline_id=$1 ORDER BY position, id LIMIT 1`,
			pipelineID).Scan(&stageID); err != nil {
			return httpx.BusinessRule("target pipeline has no stages")
		}
	}
	contractID := target.ID
	if err := s.leads.PlaceInPipeline(ctx, tx, leadID, buyerID, pipelineID, stageID, &contractID); err != nil {
		return err
	}
	if err := billing.Debit(ctx, tx, buyerID, target.RatePerLead, leadID, target.ID, "lead routed from intake queue"); err != nil {
		return err
	}
	if err := s.leads.SetStatus(ctx, tx, leadID, "distributed"); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx,
		`UPDATE lead_intake_queue SET status='routed', reviewed_by=$2, reviewed_at=now() WHERE id=$1`,
		queueID, reviewerID); err != nil {
		return err
	}
	adminIDs, err := s.accounts.AdminUserIDs(ctx, tx, buyerID)
	if err != nil {
		return err
	}
	if err := s.notif.Enqueue(ctx, tx, adminIDs, "new_lead", map[string]any{"lead_id": leadID}); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// MapField saves a source payload mapping (when source exists) and applies the value to the lead.
func (s *Service) MapField(ctx context.Context, publisherID, queueID int64, sourceKey, targetType string, builtinField *string, customFieldID *int64) (*QueueItem, error) {
	if sourceKey == "" {
		return nil, httpx.Validation("source_key is required")
	}
	if targetType != "builtin" && targetType != "custom" {
		return nil, httpx.Validation("target_type must be builtin or custom")
	}
	if targetType == "builtin" && (builtinField == nil || *builtinField == "") {
		return nil, httpx.Validation("builtin_field is required for builtin target")
	}
	if targetType == "custom" && (customFieldID == nil || *customFieldID == 0) {
		return nil, httpx.Validation("custom_field_id is required for custom target")
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	var leadID int64
	var sourceSlug *string
	var rawPayload []byte
	if err := tx.QueryRow(ctx,
		`SELECT q.lead_id, q.source, q.raw_payload FROM lead_intake_queue q WHERE q.id=$1`, queueID).
		Scan(&leadID, &sourceSlug, &rawPayload); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, httpx.NotFound("queue item not found")
		}
		return nil, err
	}

	var raw map[string]any
	if err := json.Unmarshal(rawPayload, &raw); err != nil {
		return nil, err
	}
	flat := flattenPayload(raw)
	v, ok := flat[sourceKey]
	if !ok {
		return nil, httpx.Validation("source_key not found in payload")
	}

	slug := ""
	if sourceSlug != nil {
		slug = *sourceSlug
	}
	if slug != "" {
		src, err := routing.MatchSourceBySlug(ctx, tx, publisherID, slug)
		if err != nil {
			return nil, err
		}
		if src != nil {
			var existing int64
			err := tx.QueryRow(ctx,
				`SELECT id FROM routing_source_field_map WHERE source_id=$1 AND source_key=$2`,
				src.ID, sourceKey).Scan(&existing)
			if errors.Is(err, pgx.ErrNoRows) {
				_, err = tx.Exec(ctx,
					`INSERT INTO routing_source_field_map(source_id, source_key, target_type, builtin_field, custom_field_id)
					 VALUES ($1,$2,$3,$4,$5)`,
					src.ID, sourceKey, targetType, builtinField, customFieldID)
				if err != nil {
					return nil, err
				}
			} else if err != nil {
				return nil, err
			}
		}
	}

	if targetType == "builtin" {
		if err := s.leads.SetBuiltinField(ctx, tx, leadID, *builtinField, toText(v)); err != nil {
			return nil, err
		}
	} else {
		valJSON, _ := json.Marshal(v)
		if err := s.leads.UpsertCustomValue(ctx, tx, leadID, *customFieldID, valJSON); err != nil {
			return nil, err
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return s.scanQueueItem(ctx, s.pool, publisherID, queueID)
}

func (s *Service) Reject(ctx context.Context, queueID, reviewerID int64) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var leadID int64
	if err := tx.QueryRow(ctx, `SELECT lead_id FROM lead_intake_queue WHERE id=$1 FOR UPDATE`, queueID).Scan(&leadID); err != nil {
		return httpx.NotFound("queue item not found")
	}
	if _, err := tx.Exec(ctx, `UPDATE lead_intake_queue SET status='rejected', reviewed_by=$2, reviewed_at=now() WHERE id=$1`, queueID, reviewerID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `UPDATE leads SET status='closed' WHERE id=$1`, leadID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func toText(v any) string {
	switch t := v.(type) {
	case nil:
		return ""
	case string:
		return t
	case float64:
		return fmt.Sprintf("%v", t)
	case bool:
		return fmt.Sprintf("%v", t)
	default:
		b, _ := json.Marshal(v)
		return string(b)
	}
}

func nullStr(s string) interface{} {
	if s == "" {
		return nil
	}
	return s
}
