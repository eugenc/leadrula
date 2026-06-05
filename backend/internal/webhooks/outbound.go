package webhooks

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"regexp"
	"strings"

	"github.com/echayko/leadrula/backend/internal/database"
	"github.com/echayko/leadrula/backend/internal/leads"
	"github.com/echayko/leadrula/backend/internal/pipelines"
	"github.com/jackc/pgx/v5"
)

// Trigger event strings — mirror the outbound_trigger_event DB enum.
const (
	EventLeadCreate        = "lead.create"
	EventLeadUpdate        = "lead.update"
	EventLeadDelete        = "lead.delete"
	EventPipelineMoveStage = "pipeline.move_stage"
	EventPipelinePlace     = "pipeline.place"
	EventPipelineStageRule = "pipeline.stage_rule_applied"
)

// PipelineContext is an alias so callers use leads.PipelineContext directly.
type PipelineContext = leads.PipelineContext

// OutboundTrigger is one outbound delivery rule attached to a webhook.
type OutboundTrigger struct {
	ID              int64           `json:"id"`
	WebhookID       int64           `json:"webhook_id"`
	TriggerEvent    string          `json:"trigger_event"`
	ConditionLogic  string          `json:"condition_logic"`
	Conditions      json.RawMessage `json:"conditions"`
	PayloadTemplate string          `json:"payload_template"`
	ResponseMap     json.RawMessage `json:"response_map"`
	Position        int             `json:"position"`
	IsActive        bool            `json:"is_active"`
}

// ── outbound trigger CRUD ──────────────────────────────────────────

func (s *Service) ListTriggers(ctx context.Context, webhookID int64) ([]OutboundTrigger, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT id, webhook_id, trigger_event, condition_logic, conditions, payload_template, response_map, position, is_active
		 FROM webhook_outbound_triggers WHERE webhook_id=$1 ORDER BY position, id`, webhookID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []OutboundTrigger
	for rows.Next() {
		var t OutboundTrigger
		if err := rows.Scan(&t.ID, &t.WebhookID, &t.TriggerEvent, &t.ConditionLogic, &t.Conditions,
			&t.PayloadTemplate, &t.ResponseMap, &t.Position, &t.IsActive); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

type CreateTriggerInput struct {
	TriggerEvent    string          `json:"trigger_event"`
	ConditionLogic  string          `json:"condition_logic"`
	Conditions      json.RawMessage `json:"conditions"`
	PayloadTemplate string          `json:"payload_template"`
	ResponseMap     json.RawMessage `json:"response_map"`
	Position        int             `json:"position"`
}

func (s *Service) CreateTrigger(ctx context.Context, webhookID int64, in CreateTriggerInput) (*OutboundTrigger, error) {
	if in.ConditionLogic == "" {
		in.ConditionLogic = "and"
	}
	if in.Conditions == nil {
		in.Conditions = json.RawMessage("[]")
	}
	if in.PayloadTemplate == "" {
		in.PayloadTemplate = "{}"
	}
	if in.ResponseMap == nil {
		in.ResponseMap = json.RawMessage("[]")
	}
	var t OutboundTrigger
	err := s.pool.QueryRow(ctx,
		`INSERT INTO webhook_outbound_triggers(webhook_id, trigger_event, condition_logic, conditions, payload_template, response_map, position)
		 VALUES ($1,$2,$3,$4,$5,$6,$7)
		 RETURNING id, webhook_id, trigger_event, condition_logic, conditions, payload_template, response_map, position, is_active`,
		webhookID, in.TriggerEvent, in.ConditionLogic, in.Conditions, in.PayloadTemplate, in.ResponseMap, in.Position).Scan(
		&t.ID, &t.WebhookID, &t.TriggerEvent, &t.ConditionLogic, &t.Conditions,
		&t.PayloadTemplate, &t.ResponseMap, &t.Position, &t.IsActive)
	if err != nil {
		return nil, err
	}
	return &t, nil
}

type UpdateTriggerInput struct {
	TriggerEvent    *string
	ConditionLogic  *string
	Conditions      json.RawMessage
	PayloadTemplate *string
	ResponseMap     json.RawMessage
	Position        *int
	IsActive        *bool
}

func (s *Service) UpdateTrigger(ctx context.Context, id int64, in UpdateTriggerInput) (*OutboundTrigger, error) {
	var t OutboundTrigger
	err := s.pool.QueryRow(ctx,
		`UPDATE webhook_outbound_triggers SET
		   trigger_event    = COALESCE($2, trigger_event),
		   condition_logic  = COALESCE($3, condition_logic),
		   conditions       = COALESCE($4, conditions),
		   payload_template = COALESCE($5, payload_template),
		   response_map     = COALESCE($6, response_map),
		   position         = COALESCE($7, position),
		   is_active        = COALESCE($8, is_active)
		 WHERE id=$1
		 RETURNING id, webhook_id, trigger_event, condition_logic, conditions, payload_template, response_map, position, is_active`,
		id, in.TriggerEvent, in.ConditionLogic, in.Conditions, in.PayloadTemplate, in.ResponseMap, in.Position, in.IsActive).Scan(
		&t.ID, &t.WebhookID, &t.TriggerEvent, &t.ConditionLogic, &t.Conditions,
		&t.PayloadTemplate, &t.ResponseMap, &t.Position, &t.IsActive)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("trigger not found")
		}
		return nil, err
	}
	return &t, nil
}

func (s *Service) DeleteTrigger(ctx context.Context, id int64) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM webhook_outbound_triggers WHERE id=$1`, id)
	return err
}

// ── outbound connection provisioning ──────────────────────────────

// syncOutboundConnection creates or removes the hidden integration_connections row
// that lets the delivery queue worker post to outbound_url.
func (s *Service) syncOutboundConnection(ctx context.Context, w *Webhook) error {
	if !w.OutboundEnabled || w.OutboundURL == nil || *w.OutboundURL == "" {
		// Disable: nothing to provision; the FK will just be null.
		return nil
	}
	if len(s.encKey) == 0 {
		return fmt.Errorf("encryption key not configured; cannot provision outbound connection")
	}

	// Look up the webhook provider row ID.
	var providerID int64
	if err := s.pool.QueryRow(ctx,
		`SELECT id FROM integration_providers WHERE slug='webhook'`).Scan(&providerID); err != nil {
		return fmt.Errorf("webhook integration provider not found: %w", err)
	}

	// Generate a fresh HMAC secret for this outbound connection.
	rawSecret := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, rawSecret); err != nil {
		return err
	}
	secretHex := fmt.Sprintf("%x", rawSecret)

	creds := map[string]string{"url": *w.OutboundURL, "secret": secretHex}
	credsJSON, _ := json.Marshal(creds)
	encrypted, err := aesEncrypt(s.encKey, credsJSON)
	if err != nil {
		return err
	}

	name := fmt.Sprintf("webhook:%d", w.ID)
	var connID int64
	if err := s.pool.QueryRow(ctx,
		`INSERT INTO integration_connections(account_id, provider_id, name, credentials, config)
		 VALUES ($1,$2,$3,$4,'{}')
		 ON CONFLICT ON CONSTRAINT uq_webhook_connection DO UPDATE
		   SET credentials = EXCLUDED.credentials,
		       name        = EXCLUDED.name
		 RETURNING id`,
		w.AccountID, providerID, name, encrypted).Scan(&connID); err != nil {
		// Fallback: no unique constraint — just insert.
		if err2 := s.pool.QueryRow(ctx,
			`SELECT id FROM integration_connections WHERE account_id=$1 AND name=$2`,
			w.AccountID, name).Scan(&connID); err2 != nil {
			if err3 := s.pool.QueryRow(ctx,
				`INSERT INTO integration_connections(account_id, provider_id, name, credentials, config)
				 VALUES ($1,$2,$3,$4,'{}') RETURNING id`,
				w.AccountID, providerID, name, encrypted).Scan(&connID); err3 != nil {
				return err3
			}
		} else {
			s.pool.Exec(ctx, //nolint
				`UPDATE integration_connections SET credentials=$2 WHERE id=$1`, connID, encrypted)
		}
	}
	if _, err := s.pool.Exec(ctx,
		`UPDATE webhooks SET outbound_connection_id=$2 WHERE id=$1`, w.ID, connID); err != nil {
		return err
	}
	w.OutboundConnectionID = &connID
	return nil
}

// RotateOutboundSecret regenerates the HMAC signing secret for the outbound connection.
func (s *Service) RotateOutboundSecret(ctx context.Context, accountID, webhookID int64) (string, error) {
	var outURL *string
	var connID *int64
	if err := s.pool.QueryRow(ctx,
		`SELECT outbound_url, outbound_connection_id FROM webhooks WHERE id=$1 AND account_id=$2`,
		webhookID, accountID).Scan(&outURL, &connID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", fmt.Errorf("webhook not found")
		}
		return "", err
	}
	if connID == nil || outURL == nil {
		return "", fmt.Errorf("outbound not configured")
	}
	rawSecret := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, rawSecret); err != nil {
		return "", err
	}
	secretHex := fmt.Sprintf("%x", rawSecret)
	creds := map[string]string{"url": *outURL, "secret": secretHex}
	credsJSON, _ := json.Marshal(creds)
	encrypted, err := aesEncrypt(s.encKey, credsJSON)
	if err != nil {
		return "", err
	}
	if _, err := s.pool.Exec(ctx,
		`UPDATE integration_connections SET credentials=$2 WHERE id=$1`, *connID, encrypted); err != nil {
		return "", err
	}
	return secretHex, nil
}

// ── FireOutbound ───────────────────────────────────────────────────

// FireOutbound evaluates all active outbound triggers for the given event,
// renders the payload template, and enqueues for delivery.
// It satisfies the leads.WebhookFirer interface so it can be wired into leads.Service.
func (s *Service) FireOutbound(ctx context.Context, accountID int64, event string, lead *leads.Lead, pctx PipelineContext) {
	if s.outbound == nil {
		return
	}
	rows, err := s.pool.Query(ctx,
		`SELECT t.id, t.webhook_id, t.condition_logic, t.conditions, t.payload_template,
		        w.outbound_connection_id
		 FROM webhook_outbound_triggers t
		 JOIN webhooks w ON w.id=t.webhook_id
		 WHERE w.account_id=$1 AND t.trigger_event=$2 AND t.is_active AND w.outbound_enabled
		   AND w.outbound_url IS NOT NULL AND w.outbound_connection_id IS NOT NULL
		 ORDER BY t.position, t.id`,
		accountID, event)
	if err != nil {
		log.Printf("webhooks: FireOutbound query error: %v", err)
		return
	}
	defer rows.Close()

	type triggerRow struct {
		id           int64
		webhookID    int64
		logic        string
		conditions   json.RawMessage
		template     string
		connectionID int64
	}

	var triggers []triggerRow
	for rows.Next() {
		var tr triggerRow
		if err := rows.Scan(&tr.id, &tr.webhookID, &tr.logic, &tr.conditions, &tr.template, &tr.connectionID); err != nil {
			log.Printf("webhooks: FireOutbound scan error: %v", err)
			return
		}
		triggers = append(triggers, tr)
	}
	if err := rows.Err(); err != nil {
		log.Printf("webhooks: FireOutbound rows error: %v", err)
		return
	}

	// Build shared eval context for condition matching.
	var ec *pipelines.LeadEvalContext
	if lead != nil {
		ec, err = pipelines.BuildLeadEvalContext(ctx, s.pool, accountID, lead.ID, pctx.PrevStageID)
		if err != nil {
			log.Printf("webhooks: FireOutbound buildEvalContext: %v", err)
		}
	}

	tctx := buildTemplateContext(event, lead, pctx)

	for _, tr := range triggers {
		if ec != nil {
			conds, err := pipelines.ParseConditions(tr.conditions)
			if err != nil {
				log.Printf("webhooks: trigger %d invalid conditions: %v", tr.id, err)
				continue
			}
			if !pipelines.EvalConditions(conds, tr.logic, ec) {
				continue
			}
		}

		payload, err := renderTemplate(tr.template, tctx)
		if err != nil {
			log.Printf("webhooks: trigger %d template render error: %v", tr.id, err)
			continue
		}
		if !json.Valid(payload) {
			log.Printf("webhooks: trigger %d rendered payload is not valid JSON", tr.id)
			continue
		}

		var leadID int64
		if lead != nil {
			leadID = lead.ID
		}
		if err := s.outbound.EnqueueWebhookDelivery(ctx, tr.connectionID, tr.id, leadID, payload); err != nil {
			log.Printf("webhooks: trigger %d enqueue error: %v", tr.id, err)
		}
	}
}

// ── template renderer ─────────────────────────────────────────────

var placeholderRe = regexp.MustCompile(`\{\{([^}]+)\}\}`)

// buildTemplateContext assembles a flat string map from lead + pipeline snapshot.
func buildTemplateContext(event string, l *leads.Lead, pctx PipelineContext) map[string]string {
	m := map[string]string{
		"event":         event,
		"trigger.event": event,
	}
	if l == nil {
		return m
	}
	m["lead.public_id"] = l.PublicID
	m["lead.first_name"] = l.FirstName
	m["lead.last_name"] = l.LastName
	m["lead.status"] = l.Status
	optStr := func(key string, p *string) {
		if p != nil {
			m[key] = *p
		}
	}
	optStr("lead.phone", l.Phone)
	optStr("lead.email", l.Email)
	optStr("lead.address", l.Address)
	optStr("lead.city", l.City)
	optStr("lead.state", l.State)
	optStr("lead.zip", l.Zip)
	optStr("lead.source", l.Source)
	optStr("lead.external_id", l.ExternalID)
	m["lead.created_at"] = l.CreatedAt.String()
	m["lead.updated_at"] = l.UpdatedAt.String()
	// Custom values (keyed by field ID).
	for k, raw := range l.CustomValues {
		var s string
		if json.Unmarshal(raw, &s) == nil {
			m["lead.custom."+k] = s
		} else {
			m["lead.custom."+k] = string(raw)
		}
	}
	// Pipeline context.
	if pctx.PipelineID != nil {
		m["pipeline.pipeline_id"] = fmt.Sprintf("%d", *pctx.PipelineID)
	}
	optStr("pipeline.pipeline_name", pctx.PipelineName)
	if pctx.StageID != nil {
		m["pipeline.stage_id"] = fmt.Sprintf("%d", *pctx.StageID)
	}
	optStr("pipeline.stage_name", pctx.StageName)
	if pctx.PrevStageID != nil {
		m["pipeline.previous_stage_id"] = fmt.Sprintf("%d", *pctx.PrevStageID)
	}
	optStr("pipeline.previous_stage_name", pctx.PrevStageName)
	return m
}

// renderTemplate replaces {{key}} placeholders with values from ctx.
func renderTemplate(tmpl string, ctx map[string]string) ([]byte, error) {
	result := placeholderRe.ReplaceAllStringFunc(tmpl, func(match string) string {
		key := strings.TrimSpace(match[2 : len(match)-2])
		if v, ok := ctx[key]; ok {
			// Escape for JSON string context.
			b, _ := json.Marshal(v)
			// json.Marshal wraps in quotes; strip them so the value substitutes in-place.
			s := string(b)
			if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
				return s[1 : len(s)-1]
			}
			return s
		}
		return ""
	})
	return []byte(result), nil
}

// ── encryption helpers (local copy avoids importing integrations) ──

func aesEncrypt(key, plaintext []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}
	return gcm.Seal(nonce, nonce, plaintext, nil), nil
}

// loadLeadForFire loads the lead with custom values for FireOutbound.
// It's called by callers that only have leadID available.
func (s *Service) loadLeadForFire(ctx context.Context, q database.Querier, accountID, leadID int64) (*leads.Lead, error) {
	l, err := s.leads.GetByID(ctx, q, leadID)
	if err != nil {
		return nil, err
	}
	_ = leads.LoadCustomValues(ctx, q, l)
	return l, nil
}
