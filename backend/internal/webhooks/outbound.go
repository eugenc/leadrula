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
	ID             int64           `json:"id"`
	WebhookID      int64           `json:"webhook_id"`
	TriggerEvent   string          `json:"trigger_event"`
	ConditionLogic string          `json:"condition_logic"`
	Conditions     json.RawMessage `json:"conditions"`
	Position       int             `json:"position"`
	IsActive       bool            `json:"is_active"`
}

// ── outbound trigger CRUD ──────────────────────────────────────────

func (s *Service) ListTriggers(ctx context.Context, webhookID int64) ([]OutboundTrigger, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT id, webhook_id, trigger_event, condition_logic, conditions, position, is_active
		 FROM webhook_outbound_triggers WHERE webhook_id=$1 ORDER BY position, id`, webhookID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []OutboundTrigger
	for rows.Next() {
		var t OutboundTrigger
		if err := rows.Scan(&t.ID, &t.WebhookID, &t.TriggerEvent, &t.ConditionLogic, &t.Conditions,
			&t.Position, &t.IsActive); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

type CreateTriggerInput struct {
	TriggerEvent   string          `json:"trigger_event"`
	ConditionLogic string          `json:"condition_logic"`
	Conditions     json.RawMessage `json:"conditions"`
	Position       int             `json:"position"`
}

func (s *Service) CreateTrigger(ctx context.Context, webhookID int64, in CreateTriggerInput) (*OutboundTrigger, error) {
	if in.ConditionLogic == "" {
		in.ConditionLogic = "and"
	}
	if in.Conditions == nil {
		in.Conditions = json.RawMessage("[]")
	}
	var t OutboundTrigger
	err := s.pool.QueryRow(ctx,
		`INSERT INTO webhook_outbound_triggers(webhook_id, trigger_event, condition_logic, conditions, position)
		 VALUES ($1,$2,$3,$4,$5)
		 RETURNING id, webhook_id, trigger_event, condition_logic, conditions, position, is_active`,
		webhookID, in.TriggerEvent, in.ConditionLogic, in.Conditions, in.Position).Scan(
		&t.ID, &t.WebhookID, &t.TriggerEvent, &t.ConditionLogic, &t.Conditions,
		&t.Position, &t.IsActive)
	if err != nil {
		return nil, err
	}
	return &t, nil
}

type UpdateTriggerInput struct {
	TriggerEvent   *string
	ConditionLogic *string
	Conditions     json.RawMessage
	Position       *int
	IsActive       *bool
}

func (s *Service) UpdateTrigger(ctx context.Context, id int64, in UpdateTriggerInput) (*OutboundTrigger, error) {
	var t OutboundTrigger
	err := s.pool.QueryRow(ctx,
		`UPDATE webhook_outbound_triggers SET
		   trigger_event   = COALESCE($2, trigger_event),
		   condition_logic = COALESCE($3, condition_logic),
		   conditions      = COALESCE($4, conditions),
		   position        = COALESCE($5, position),
		   is_active       = COALESCE($6, is_active)
		 WHERE id=$1
		 RETURNING id, webhook_id, trigger_event, condition_logic, conditions, position, is_active`,
		id, in.TriggerEvent, in.ConditionLogic, in.Conditions, in.Position, in.IsActive).Scan(
		&t.ID, &t.WebhookID, &t.TriggerEvent, &t.ConditionLogic, &t.Conditions,
		&t.Position, &t.IsActive)
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
		return nil
	}
	if len(s.encKey) == 0 {
		log.Printf("webhooks: INTEGRATION_ENC_KEY not configured; skipping outbound connection sync for webhook %d", w.ID)
		return nil
	}

	var providerID int64
	if err := s.pool.QueryRow(ctx,
		`SELECT id FROM integration_providers WHERE slug='webhook'`).Scan(&providerID); err != nil {
		return fmt.Errorf("webhook integration provider not found: %w", err)
	}

	secretHex, err := s.existingOutboundSecret(ctx, w.OutboundConnectionID)
	if err != nil {
		return err
	}
	if secretHex == "" {
		rawSecret := make([]byte, 32)
		if _, err := io.ReadFull(rand.Reader, rawSecret); err != nil {
			return err
		}
		secretHex = fmt.Sprintf("%x", rawSecret)
	}

	format := w.OutboundFormat
	if format == "" {
		format = "json"
	}
	method := w.OutboundMethod
	if method == "" {
		method = "POST"
	}
	creds := map[string]string{
		"url":    *w.OutboundURL,
		"secret": secretHex,
		"format": format,
		"method": method,
	}
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

func (s *Service) existingOutboundSecret(ctx context.Context, connID *int64) (string, error) {
	if connID == nil || len(s.encKey) == 0 {
		return "", nil
	}
	var enc []byte
	if err := s.pool.QueryRow(ctx,
		`SELECT credentials FROM integration_connections WHERE id=$1`, *connID).Scan(&enc); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", nil
		}
		return "", err
	}
	if len(enc) == 0 {
		return "", nil
	}
	plain, err := aesDecrypt(s.encKey, enc)
	if err != nil {
		return "", err
	}
	var creds struct {
		Secret string `json:"secret"`
	}
	if json.Unmarshal(plain, &creds) != nil {
		return "", nil
	}
	return creds.Secret, nil
}

// RotateOutboundSecret regenerates the HMAC signing secret for the outbound connection.
func (s *Service) RotateOutboundSecret(ctx context.Context, accountID, webhookID int64) (string, error) {
	var w Webhook
	if err := s.pool.QueryRow(ctx,
		`SELECT `+webhookCols+` FROM webhooks WHERE id=$1 AND account_id=$2`,
		webhookID, accountID).Scan(
		&w.ID, &w.AccountID, &w.Name, &w.Slug, &w.SecretPrefix, &w.IsActive,
		&w.InboundEnabled, &w.OutboundEnabled, &w.OutboundURL, &w.OutboundFormat, &w.OutboundMethod,
		&w.OutboundPayloadTemplate, &w.OutboundFieldMap, &w.OutboundResponseMap,
		&w.OutboundConnectionID, &w.CreatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", fmt.Errorf("webhook not found")
		}
		return "", err
	}
	if w.OutboundConnectionID == nil || w.OutboundURL == nil {
		return "", fmt.Errorf("outbound not configured")
	}
	rawSecret := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, rawSecret); err != nil {
		return "", err
	}
	secretHex := fmt.Sprintf("%x", rawSecret)
	format := w.OutboundFormat
	if format == "" {
		format = "json"
	}
	method := w.OutboundMethod
	if method == "" {
		method = "POST"
	}
	creds := map[string]string{
		"url":    *w.OutboundURL,
		"secret": secretHex,
		"format": format,
		"method": method,
	}
	credsJSON, _ := json.Marshal(creds)
	encrypted, err := aesEncrypt(s.encKey, credsJSON)
	if err != nil {
		return "", err
	}
	if _, err := s.pool.Exec(ctx,
		`UPDATE integration_connections SET credentials=$2 WHERE id=$1`, *w.OutboundConnectionID, encrypted); err != nil {
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
		`SELECT t.id, t.webhook_id, t.condition_logic, t.conditions,
		        w.outbound_format, w.outbound_payload_template, w.outbound_field_map,
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
		format       string
		template     string
		fieldMap     json.RawMessage
		connectionID int64
	}

	var triggers []triggerRow
	for rows.Next() {
		var tr triggerRow
		if err := rows.Scan(&tr.id, &tr.webhookID, &tr.logic, &tr.conditions,
			&tr.format, &tr.template, &tr.fieldMap, &tr.connectionID); err != nil {
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

		var payload []byte
		var err error
		if tr.format == "url" {
			entries, err := parseOutboundFieldMap(tr.fieldMap)
			if err != nil {
				log.Printf("webhooks: trigger %d field map error: %v", tr.id, err)
				continue
			}
			if len(entries) == 0 {
				log.Printf("webhooks: trigger %d url format with empty field map", tr.id)
				continue
			}
			payload, err = buildURLPayload(event, lead, pctx, entries)
		} else {
			payload, err = renderTemplate(tr.template, tctx)
		}
		if err != nil {
			log.Printf("webhooks: trigger %d payload build error: %v", tr.id, err)
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

func parseOutboundFieldMap(raw json.RawMessage) ([]OutboundFieldMapEntry, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return nil, nil
	}
	var entries []OutboundFieldMapEntry
	if err := json.Unmarshal(raw, &entries); err != nil {
		return nil, err
	}
	return entries, nil
}

func buildURLPayload(event string, l *leads.Lead, pctx PipelineContext, entries []OutboundFieldMapEntry) ([]byte, error) {
	tctx := buildTemplateContext(event, l, pctx)
	out := map[string]string{}
	for _, e := range entries {
		if e.DestKey == "" {
			continue
		}
		v := resolveOutboundFieldValue(e, tctx, l)
		if v != "" {
			out[e.DestKey] = v
		}
	}
	return json.Marshal(out)
}

func resolveOutboundFieldValue(e OutboundFieldMapEntry, tctx map[string]string, l *leads.Lead) string {
	switch e.SourceType {
	case "static":
		if e.StaticValue != nil {
			return *e.StaticValue
		}
	case "builtin":
		if e.BuiltinField == nil {
			return ""
		}
		switch *e.BuiltinField {
		case "first_name":
			if l != nil {
				return l.FirstName
			}
		case "last_name":
			if l != nil {
				return l.LastName
			}
		case "phone", "email", "address", "city", "state", "zip", "source", "external_id", "status":
			if l != nil {
				return optStrVal(lFieldPtr(l, *e.BuiltinField))
			}
		case "public_id":
			if l != nil {
				return l.PublicID
			}
		}
	case "custom":
		if e.CustomFieldID != nil && l != nil {
			key := fmt.Sprintf("%d", *e.CustomFieldID)
			if raw, ok := l.CustomValues[key]; ok {
				var s string
				if json.Unmarshal(raw, &s) == nil {
					return s
				}
				return string(raw)
			}
		}
	case "meta":
		if e.MetaField != nil {
			return tctx[*e.MetaField]
		}
	}
	return ""
}

func lFieldPtr(l *leads.Lead, field string) *string {
	switch field {
	case "phone":
		return l.Phone
	case "email":
		return l.Email
	case "address":
		return l.Address
	case "city":
		return l.City
	case "state":
		return l.State
	case "zip":
		return l.Zip
	case "source":
		return l.Source
	case "external_id":
		return l.ExternalID
	case "status":
		return &l.Status
	}
	return nil
}

func optStrVal(p *string) string {
	if p != nil {
		return *p
	}
	return ""
}

// ── encryption helpers (local copy avoids importing integrations) ──

func aesDecrypt(key, ciphertext []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	if len(ciphertext) < gcm.NonceSize() {
		return nil, fmt.Errorf("ciphertext too short")
	}
	nonce, ct := ciphertext[:gcm.NonceSize()], ciphertext[gcm.NonceSize():]
	return gcm.Open(nil, nonce, ct, nil)
}

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
