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

// IngestResult is returned to the API caller.
type IngestResult struct {
	LeadID string `json:"lead_id"`
	Status string `json:"status"`
}

var builtinKeys = []string{"first_name", "last_name", "phone", "email", "address", "city", "state", "zip"}

// Ingest runs the atomic intake flow (DB spec §4.1).
func (s *Service) Ingest(ctx context.Context, publisherID int64, raw map[string]any) (*IngestResult, error) {
	campaignName, _ := raw["campaign_name"].(string)

	// flatten: top-level keys plus any "custom" object keys are matchable sources
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

	rawJSON, _ := json.Marshal(raw)

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	leadID, publicID, err := s.leads.InsertLead(ctx, tx, publisherID, publisherID, campaignName, rawJSON)
	if err != nil {
		return nil, err
	}

	// populate well-known builtin contact fields from explicit top-level keys
	for _, k := range builtinKeys {
		if v, ok := sources[k]; ok {
			if str := toText(v); str != "" {
				if err := s.leads.SetBuiltinField(ctx, tx, leadID, k, str); err != nil {
					return nil, err
				}
			}
		}
	}

	match, err := routing.MatchCampaign(ctx, tx, publisherID, campaignName)
	if err != nil {
		return nil, err
	}
	if match == nil {
		// no campaign → intake queue, stays with publisher in review
		if _, err := tx.Exec(ctx,
			`INSERT INTO lead_intake_queue(lead_id, raw_payload, campaign_name) VALUES ($1,$2,$3)`,
			leadID, rawJSON, nullStr(campaignName)); err != nil {
			return nil, err
		}
		if err := tx.Commit(ctx); err != nil {
			return nil, err
		}
		return &IngestResult{LeadID: publicID, Status: "review"}, nil
	}

	// apply per-campaign field mapping
	maps, err := routing.FieldMap(ctx, tx, match.CampaignID)
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

	// resolve buyer contract by the campaign's target (buyer) pipeline
	target, err := contracts.FindByBuyerPipeline(ctx, tx, match.TargetPipelineID)
	if err != nil {
		return nil, err
	}
	if target == nil {
		// configured campaign but no contract — fall back to the queue
		if _, err := tx.Exec(ctx,
			`INSERT INTO lead_intake_queue(lead_id, raw_payload, campaign_name) VALUES ($1,$2,$3)`,
			leadID, rawJSON, nullStr(campaignName)); err != nil {
			return nil, err
		}
		if err := tx.Commit(ctx); err != nil {
			return nil, err
		}
		return &IngestResult{LeadID: publicID, Status: "review"}, nil
	}

	if err := s.leads.PlaceInPipeline(ctx, tx, leadID, target.BuyerID, target.BuyerPipelineID, match.TargetStageID, target.ID); err != nil {
		return nil, err
	}
	if err := billing.Debit(ctx, tx, target.BuyerID, target.RatePerLead, leadID, target.ID, "lead distributed: "+campaignName); err != nil {
		return nil, err
	}
	if err := s.leads.SetStatus(ctx, tx, leadID, "distributed"); err != nil {
		return nil, err
	}
	adminIDs, err := s.accounts.AdminUserIDs(ctx, tx, target.BuyerID)
	if err != nil {
		return nil, err
	}
	if err := s.notif.Enqueue(ctx, tx, adminIDs, "new_lead", map[string]any{"lead_id": leadID}); err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return &IngestResult{LeadID: publicID, Status: "distributed"}, nil
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
	CampaignName *string         `json:"campaign_name"`
	RawPayload   json.RawMessage `json:"raw_payload"`
	Status       string          `json:"status"`
	CreatedAt    time.Time       `json:"created_at"`
}

func (s *Service) ListQueue(ctx context.Context, status string) ([]QueueItem, error) {
	if status == "" {
		status = "pending_review"
	}
	rows, err := s.pool.Query(ctx,
		`SELECT q.id, q.lead_id, l.first_name, l.last_name, l.phone, q.campaign_name, q.raw_payload, q.status, q.created_at
		 FROM lead_intake_queue q JOIN leads l ON l.id = q.lead_id
		 WHERE q.status = $1::intake_status ORDER BY q.created_at DESC`, status)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []QueueItem
	for rows.Next() {
		var it QueueItem
		if err := rows.Scan(&it.ID, &it.LeadID, &it.FirstName, &it.LastName, &it.Phone, &it.CampaignName, &it.RawPayload, &it.Status, &it.CreatedAt); err != nil {
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
	// Default to the buyer's contract pipeline + its first stage when the
	// caller didn't specify an explicit destination.
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
	if err := s.leads.PlaceInPipeline(ctx, tx, leadID, buyerID, pipelineID, stageID, target.ID); err != nil {
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
