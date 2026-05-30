// Package intake handles the public lead ingest flow and the publisher's
// pre-routing intake queue.
package intake

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/echayko/leadrula/backend/internal/accounts"
	"github.com/echayko/leadrula/backend/internal/billing"
	"github.com/echayko/leadrula/backend/internal/contracts"
	"github.com/echayko/leadrula/backend/internal/leads"
	"github.com/echayko/leadrula/backend/internal/notifications"
	"github.com/echayko/leadrula/backend/internal/routing"
	"github.com/echayko/leadrula/backend/pkg/httpx"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Service struct {
	pool     *pgxpool.Pool
	leads    *leads.Repository
	notif    *notifications.Service
	accounts *accounts.Repository
}

func NewService(pool *pgxpool.Pool, leadRepo *leads.Repository, notif *notifications.Service, acc *accounts.Repository) *Service {
	return &Service{pool: pool, leads: leadRepo, notif: notif, accounts: acc}
}

func (s *Service) routeDeps() leads.RouteApplyDeps {
	return leads.RouteApplyDeps{Repo: s.leads, Accounts: s.accounts, Notif: s.notif}
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
	CreatedAt    time.Time       `json:"created_at"`
}

func (s *Service) ListQueue(ctx context.Context, status string) ([]QueueItem, error) {
	if status == "" {
		status = "pending_review"
	}
	rows, err := s.pool.Query(ctx,
		`SELECT q.id, q.lead_id, l.first_name, l.last_name, l.phone, q.source, q.raw_payload, q.status, q.created_at
		 FROM lead_intake_queue q JOIN leads l ON l.id = q.lead_id
		 WHERE q.status = $1::intake_status ORDER BY q.created_at DESC`, status)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []QueueItem
	for rows.Next() {
		var it QueueItem
		if err := rows.Scan(&it.ID, &it.LeadID, &it.FirstName, &it.LastName, &it.Phone, &it.Source, &it.RawPayload, &it.Status, &it.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, it)
	}
	return out, rows.Err()
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
