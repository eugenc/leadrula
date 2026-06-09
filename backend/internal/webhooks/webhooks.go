package webhooks

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"time"

	"github.com/echayko/leadrula/backend/internal/auth"
	"github.com/echayko/leadrula/backend/internal/database"
	"github.com/echayko/leadrula/backend/internal/leads"
	"github.com/echayko/leadrula/backend/pkg/httpx"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Webhook struct {
	ID                      int64           `json:"id"`
	AccountID               int64           `json:"-"`
	Name                    string          `json:"name"`
	Slug                    string          `json:"slug"`
	SecretPrefix            string          `json:"secret_prefix,omitempty"`
	IsActive                bool            `json:"is_active"`
	InboundEnabled          bool            `json:"inbound_enabled"`
	InboundSecretRequired   bool            `json:"inbound_secret_required"`
	OutboundEnabled         bool            `json:"outbound_enabled"`
	OutboundSignEnabled     bool            `json:"outbound_sign_enabled"`
	OutboundURL             *string         `json:"outbound_url,omitempty"`
	OutboundFormat          string          `json:"outbound_format"`
	OutboundMethod          string          `json:"outbound_method"`
	OutboundPayloadTemplate string          `json:"outbound_payload_template"`
	OutboundFieldMap        json.RawMessage `json:"outbound_field_map"`
	OutboundResponseMap     json.RawMessage `json:"outbound_response_map"`
	OutboundConnectionID    *int64          `json:"-"`
	CreatedAt               time.Time       `json:"created_at"`
}

// OutboundFieldMapEntry maps an external param name to a lead/event value source.
type OutboundFieldMapEntry struct {
	DestKey       string  `json:"dest_key"`
	SourceType    string  `json:"source_type"`
	BuiltinField  *string `json:"builtin_field,omitempty"`
	CustomFieldID *int64  `json:"custom_field_id,omitempty"`
	StaticValue   *string `json:"static_value,omitempty"`
	MetaField     *string `json:"meta_field,omitempty"`
}

type WebhookEvent struct {
	ID                int64           `json:"id"`
	WebhookID         int64           `json:"webhook_id"`
	Action            string          `json:"action"`
	DuplicateMode     *string         `json:"duplicate_mode,omitempty"`
	LookupBy          *string         `json:"lookup_by,omitempty"`
	LookupSourceKey   *string         `json:"lookup_source_key,omitempty"`
	TargetStageID     *int64          `json:"target_stage_id,omitempty"`
	TargetPipelineID  *int64          `json:"target_pipeline_id,omitempty"`
	Position          int             `json:"position"`
	ConditionLogic    string          `json:"condition_logic"`
	Conditions        json.RawMessage `json:"conditions"`
	CreatedAt         time.Time       `json:"created_at"`
}

type FieldMapEntry struct {
	ID            int64     `json:"id"`
	EventID       int64     `json:"event_id"`
	SourceKey     string    `json:"source_key"`
	TargetType    string    `json:"target_type"`
	BuiltinField  *string   `json:"builtin_field"`
	CustomFieldID *int64    `json:"custom_field_id"`
	CreatedAt     time.Time `json:"created_at"`
}

type SamplePayload struct {
	Payload    json.RawMessage `json:"payload"`
	ReceivedAt *time.Time      `json:"received_at,omitempty"`
}

type Delivery struct {
	ID              int64           `json:"id"`
	WebhookID       int64           `json:"webhook_id"`
	EventID         *int64          `json:"event_id,omitempty"`
	LeadID          *int64          `json:"lead_id,omitempty"`
	LeadPublicID    *string         `json:"lead_public_id,omitempty"`
	Status          string          `json:"status"`
	ErrorMessage    *string         `json:"error_message,omitempty"`
	RequestPayload  json.RawMessage `json:"request_payload,omitempty"`
	CreatedAt       time.Time       `json:"created_at"`
}

type DeliveryListResult struct {
	Items []Delivery `json:"items"`
	Total int        `json:"total"`
	Page  int        `json:"page"`
	Limit int        `json:"limit"`
}

type AccountDelivery struct {
	Delivery
	WebhookName string `json:"webhook_name"`
	WebhookSlug string `json:"webhook_slug"`
}

type AccountDeliveryListResult struct {
	Items []AccountDelivery `json:"items"`
	Total int               `json:"total"`
	Page  int               `json:"page"`
	Limit int               `json:"limit"`
}

type ListAccountDeliveriesParams struct {
	Status    string
	WebhookID int64
	Page      int
	Limit     int
}

type ActionResult struct {
	LeadID         string `json:"lead_id,omitempty"`
	Action         string `json:"action,omitempty"`
	Status         string `json:"status,omitempty"`
	ActionID       int64  `json:"action_id,omitempty"`
	LeadInternalID int64  `json:"-"`
}

type IngestResult struct {
	LeadID     string         `json:"lead_id,omitempty"`
	Action     string         `json:"action,omitempty"`
	Status     string         `json:"status"`
	DeliveryID int64          `json:"delivery_id,omitempty"`
	Results    []ActionResult `json:"results,omitempty"`
}

// OutboundEnqueuer enqueues a rendered webhook payload onto the integration delivery queue.
type OutboundEnqueuer interface {
	EnqueueWebhookDelivery(ctx context.Context, connectionID, triggerID, leadID int64, payload []byte) error
}

type Service struct {
	pool     *pgxpool.Pool
	leads    *leads.Repository
	leadSvc  *leads.Service
	encKey   []byte
	outbound OutboundEnqueuer
}

func NewService(pool *pgxpool.Pool, leadRepo *leads.Repository, leadSvc *leads.Service, encKey []byte, outbound OutboundEnqueuer) *Service {
	return &Service{pool: pool, leads: leadRepo, leadSvc: leadSvc, encKey: encKey, outbound: outbound}
}

const webhookCols = `id, account_id, name, slug, secret_prefix, is_active,
    inbound_enabled, inbound_secret_required, outbound_enabled, outbound_sign_enabled,
    outbound_url, outbound_format, outbound_method,
    outbound_payload_template, outbound_field_map, outbound_response_map,
    outbound_connection_id, created_at`

func scanWebhook(row interface{ Scan(...any) error }) (Webhook, error) {
	var w Webhook
	return w, row.Scan(&w.ID, &w.AccountID, &w.Name, &w.Slug, &w.SecretPrefix, &w.IsActive,
		&w.InboundEnabled, &w.InboundSecretRequired, &w.OutboundEnabled, &w.OutboundSignEnabled,
		&w.OutboundURL, &w.OutboundFormat, &w.OutboundMethod,
		&w.OutboundPayloadTemplate, &w.OutboundFieldMap, &w.OutboundResponseMap,
		&w.OutboundConnectionID, &w.CreatedAt)
}

func (s *Service) List(ctx context.Context, accountID int64) ([]Webhook, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT `+webhookCols+` FROM webhooks WHERE account_id=$1 ORDER BY name`, accountID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Webhook
	for rows.Next() {
		w, err := scanWebhook(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, w)
	}
	return out, rows.Err()
}

type CreateWebhookInput struct {
	Name                  string
	Slug                  string
	InboundEnabled        *bool
	InboundSecretRequired *bool
	OutboundEnabled       *bool
	OutboundSignEnabled   *bool
	OutboundURL           *string
}

func (s *Service) Create(ctx context.Context, accountID int64, in CreateWebhookInput) (*Webhook, *string, error) {
	inbound := true
	if in.InboundEnabled != nil {
		inbound = *in.InboundEnabled
	}
	inboundSecretRequired := true
	if in.InboundSecretRequired != nil {
		inboundSecretRequired = *in.InboundSecretRequired
	}
	outbound := false
	if in.OutboundEnabled != nil {
		outbound = *in.OutboundEnabled
	}
	outboundSignEnabled := true
	if in.OutboundSignEnabled != nil {
		outboundSignEnabled = *in.OutboundSignEnabled
	}

	var hash, prefix *string
	var full *string
	if inboundSecretRequired {
		_, generated, h, p, err := generateSecret()
		if err != nil {
			return nil, nil, err
		}
		hash = &h
		prefix = &p
		full = &generated
	}

	w := &Webhook{}
	row := s.pool.QueryRow(ctx,
		`INSERT INTO webhooks(account_id, name, slug, secret_hash, secret_prefix,
		 inbound_enabled, inbound_secret_required, outbound_enabled, outbound_sign_enabled, outbound_url)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
		 RETURNING `+webhookCols,
		accountID, in.Name, in.Slug, hash, prefix,
		inbound, inboundSecretRequired, outbound, outboundSignEnabled, in.OutboundURL)
	var err error
	if *w, err = scanWebhook(row); err != nil {
		if database.IsUniqueViolation(err) {
			return nil, nil, httpx.Conflict("a webhook with this slug already exists")
		}
		return nil, nil, err
	}
	if outbound && in.OutboundURL != nil && *in.OutboundURL != "" {
		_ = s.syncOutboundConnection(ctx, w)
	}
	return w, full, nil
}

type UpdateWebhookInput struct {
	Name                    *string
	Slug                    *string
	IsActive                *bool
	InboundEnabled          *bool
	InboundSecretRequired   *bool
	OutboundEnabled         *bool
	OutboundSignEnabled     *bool
	OutboundURL             *string
	OutboundFormat          *string
	OutboundMethod          *string
	OutboundPayloadTemplate *string
	OutboundFieldMap        json.RawMessage
	OutboundResponseMap     json.RawMessage
}

func (s *Service) Update(ctx context.Context, accountID, id int64, in UpdateWebhookInput) (*Webhook, error) {
	if in.OutboundFormat != nil && *in.OutboundFormat == "url" && len(in.OutboundFieldMap) > 0 {
		var entries []OutboundFieldMapEntry
		if json.Unmarshal(in.OutboundFieldMap, &entries) == nil && len(entries) == 0 {
			return nil, httpx.Validation("outbound_field_map required for url format")
		}
	}
	current, err := s.getWebhook(ctx, accountID, id)
	if err != nil {
		return nil, err
	}
	w := &Webhook{}
	row := s.pool.QueryRow(ctx,
		`UPDATE webhooks SET
		   name                      = COALESCE($3, name),
		   slug                      = COALESCE($4, slug),
		   is_active                 = COALESCE($5, is_active),
		   inbound_enabled           = COALESCE($6, inbound_enabled),
		   inbound_secret_required   = COALESCE($7, inbound_secret_required),
		   outbound_enabled          = COALESCE($8, outbound_enabled),
		   outbound_sign_enabled     = COALESCE($9, outbound_sign_enabled),
		   outbound_url              = COALESCE($10, outbound_url),
		   outbound_format           = COALESCE($11, outbound_format),
		   outbound_method           = COALESCE($12, outbound_method),
		   outbound_payload_template = COALESCE($13, outbound_payload_template),
		   outbound_field_map        = COALESCE($14, outbound_field_map),
		   outbound_response_map     = COALESCE($15, outbound_response_map)
		 WHERE id=$1 AND account_id=$2
		 RETURNING `+webhookCols,
		id, accountID, in.Name, in.Slug, in.IsActive,
		in.InboundEnabled, in.InboundSecretRequired, in.OutboundEnabled, in.OutboundSignEnabled,
		in.OutboundURL, in.OutboundFormat, in.OutboundMethod,
		in.OutboundPayloadTemplate, nullableJSON(in.OutboundFieldMap), in.OutboundResponseMap)
	if *w, err = scanWebhook(row); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, httpx.NotFound("webhook not found")
		}
		if database.IsUniqueViolation(err) {
			return nil, httpx.Conflict("a webhook with this slug already exists")
		}
		return nil, err
	}
	if err := s.applyInboundSecretTransition(ctx, accountID, id, current.InboundSecretRequired, w.InboundSecretRequired); err != nil {
		return nil, err
	}
	if refreshed, err := s.getWebhook(ctx, accountID, id); err == nil {
		*w = *refreshed
	}
	if in.OutboundEnabled != nil || in.OutboundURL != nil || in.OutboundFormat != nil ||
		in.OutboundMethod != nil || in.OutboundSignEnabled != nil {
		if err := s.syncOutboundConnection(ctx, w); err != nil {
			return nil, err
		}
	}
	return w, nil
}

func (s *Service) getWebhook(ctx context.Context, accountID, id int64) (*Webhook, error) {
	w, err := scanWebhook(s.pool.QueryRow(ctx,
		`SELECT `+webhookCols+` FROM webhooks WHERE id=$1 AND account_id=$2`, id, accountID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, httpx.NotFound("webhook not found")
		}
		return nil, err
	}
	return &w, nil
}

func (s *Service) applyInboundSecretTransition(ctx context.Context, accountID, id int64, wasRequired, nowRequired bool) error {
	if wasRequired == nowRequired {
		return nil
	}
	if nowRequired {
		_, full, hash, prefix, err := generateSecret()
		if err != nil {
			return err
		}
		ct, err := s.pool.Exec(ctx,
			`UPDATE webhooks SET secret_hash=$3, secret_prefix=$4 WHERE id=$1 AND account_id=$2`,
			id, accountID, hash, prefix)
		if err != nil {
			return err
		}
		if ct.RowsAffected() == 0 {
			return httpx.NotFound("webhook not found")
		}
		_ = full
		return nil
	}
	ct, err := s.pool.Exec(ctx,
		`UPDATE webhooks SET secret_hash=NULL, secret_prefix=NULL WHERE id=$1 AND account_id=$2`,
		id, accountID)
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return httpx.NotFound("webhook not found")
	}
	return nil
}

func (s *Service) Delete(ctx context.Context, accountID, id int64) error {
	ct, err := s.pool.Exec(ctx, `DELETE FROM webhooks WHERE id=$1 AND account_id=$2`, id, accountID)
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return httpx.NotFound("webhook not found")
	}
	return nil
}

func (s *Service) RotateSecret(ctx context.Context, accountID, id int64) (string, error) {
	w, err := s.getWebhook(ctx, accountID, id)
	if err != nil {
		return "", err
	}
	if !w.InboundSecretRequired {
		return "", httpx.Validation("inbound secret is disabled for this webhook")
	}
	_, full, hash, prefix, err := generateSecret()
	if err != nil {
		return "", err
	}
	ct, err := s.pool.Exec(ctx,
		`UPDATE webhooks SET secret_hash=$3, secret_prefix=$4 WHERE id=$1 AND account_id=$2`,
		id, accountID, hash, prefix)
	if err != nil {
		return "", err
	}
	if ct.RowsAffected() == 0 {
		return "", httpx.NotFound("webhook not found")
	}
	return full, nil
}

func (s *Service) OwnedBy(ctx context.Context, accountID, id int64) (bool, error) {
	var ok bool
	err := s.pool.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM webhooks WHERE id=$1 AND account_id=$2)`, id, accountID).Scan(&ok)
	return ok, err
}

const webhookEventCols = `id, webhook_id, action, duplicate_mode, lookup_by, lookup_source_key,
	target_stage_id, target_pipeline_id, position, condition_logic, conditions, created_at`

func scanWebhookEvent(row interface{ Scan(...any) error }) (*WebhookEvent, error) {
	e := &WebhookEvent{}
	err := row.Scan(&e.ID, &e.WebhookID, &e.Action, &e.DuplicateMode, &e.LookupBy, &e.LookupSourceKey,
		&e.TargetStageID, &e.TargetPipelineID, &e.Position, &e.ConditionLogic, &e.Conditions, &e.CreatedAt)
	return e, err
}

func (s *Service) ListEvents(ctx context.Context, webhookID int64) ([]WebhookEvent, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT `+webhookEventCols+` FROM webhook_events WHERE webhook_id=$1 ORDER BY position, id`, webhookID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []WebhookEvent
	for rows.Next() {
		e, err := scanWebhookEvent(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *e)
	}
	return out, rows.Err()
}

type CreateEventParams struct {
	Action           string          `json:"action"`
	DuplicateMode    *string         `json:"duplicate_mode"`
	LookupBy         *string         `json:"lookup_by"`
	LookupSourceKey  *string         `json:"lookup_source_key"`
	TargetStageID    *int64          `json:"target_stage_id"`
	TargetPipelineID *int64          `json:"target_pipeline_id"`
	ConditionLogic   *string         `json:"condition_logic"`
	Conditions       json.RawMessage `json:"conditions"`
}

func (s *Service) CreateEvent(ctx context.Context, webhookID int64, p CreateEventParams) (*WebhookEvent, error) {
	if err := validateEvent(p); err != nil {
		return nil, err
	}
	logic := "and"
	if p.ConditionLogic != nil && *p.ConditionLogic != "" {
		logic = *p.ConditionLogic
	}
	conds := p.Conditions
	if len(conds) == 0 {
		conds = json.RawMessage("[]")
	}
	return scanWebhookEvent(s.pool.QueryRow(ctx,
		`INSERT INTO webhook_events(webhook_id, action, duplicate_mode, lookup_by, lookup_source_key,
		 target_stage_id, target_pipeline_id, position, condition_logic, conditions)
		 VALUES ($1,$2,$3,$4,$5,$6,$7, COALESCE((SELECT MAX(position)+1 FROM webhook_events WHERE webhook_id=$1), 0), $8, $9)
		 RETURNING `+webhookEventCols,
		webhookID, p.Action, p.DuplicateMode, p.LookupBy, p.LookupSourceKey,
		p.TargetStageID, p.TargetPipelineID, logic, conds))
}

type UpdateEventParams struct {
	Action           *string         `json:"action"`
	DuplicateMode    *string         `json:"duplicate_mode"`
	LookupBy         *string         `json:"lookup_by"`
	LookupSourceKey  *string         `json:"lookup_source_key"`
	TargetStageID    *int64          `json:"target_stage_id"`
	TargetPipelineID *int64          `json:"target_pipeline_id"`
	ConditionLogic   *string         `json:"condition_logic"`
	Conditions       json.RawMessage `json:"conditions"`
}

func (s *Service) UpdateEvent(ctx context.Context, webhookID, eventID int64, p UpdateEventParams) (*WebhookEvent, error) {
	e, err := scanWebhookEvent(s.pool.QueryRow(ctx,
		`UPDATE webhook_events SET
		   action = COALESCE($3, action),
		   duplicate_mode = COALESCE($4, duplicate_mode),
		   lookup_by = COALESCE($5, lookup_by),
		   lookup_source_key = COALESCE($6, lookup_source_key),
		   target_stage_id = COALESCE($7, target_stage_id),
		   target_pipeline_id = COALESCE($8, target_pipeline_id),
		   condition_logic = COALESCE($9, condition_logic),
		   conditions = CASE WHEN $10::jsonb IS NULL THEN conditions ELSE $10::jsonb END
		 WHERE id=$1 AND webhook_id=$2
		 RETURNING `+webhookEventCols,
		eventID, webhookID, p.Action, p.DuplicateMode, p.LookupBy, p.LookupSourceKey,
		p.TargetStageID, p.TargetPipelineID, p.ConditionLogic, nullableJSON(p.Conditions)))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, httpx.NotFound("event not found")
		}
		return nil, err
	}
	return e, nil
}

func nullableJSON(raw json.RawMessage) any {
	if len(raw) == 0 {
		return nil
	}
	return raw
}

func (s *Service) DeleteEvent(ctx context.Context, webhookID, eventID int64) error {
	ct, err := s.pool.Exec(ctx, `DELETE FROM webhook_events WHERE id=$1 AND webhook_id=$2`, eventID, webhookID)
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return httpx.NotFound("event not found")
	}
	return nil
}

func (s *Service) ListFieldMap(ctx context.Context, eventID int64) ([]FieldMapEntry, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT id, event_id, source_key, target_type, builtin_field, custom_field_id, created_at
		 FROM webhook_event_field_map WHERE event_id=$1`, eventID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []FieldMapEntry
	for rows.Next() {
		var e FieldMapEntry
		if err := rows.Scan(&e.ID, &e.EventID, &e.SourceKey, &e.TargetType, &e.BuiltinField, &e.CustomFieldID, &e.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

func (s *Service) AddFieldMap(ctx context.Context, eventID int64, sourceKey, targetType string, builtinField *string, customFieldID *int64) (*FieldMapEntry, error) {
	e := &FieldMapEntry{}
	err := s.pool.QueryRow(ctx,
		`INSERT INTO webhook_event_field_map(event_id, source_key, target_type, builtin_field, custom_field_id)
		 VALUES ($1,$2,$3,$4,$5)
		 RETURNING id, event_id, source_key, target_type, builtin_field, custom_field_id, created_at`,
		eventID, sourceKey, targetType, builtinField, customFieldID).Scan(
		&e.ID, &e.EventID, &e.SourceKey, &e.TargetType, &e.BuiltinField, &e.CustomFieldID, &e.CreatedAt)
	if err != nil {
		return nil, err
	}
	return e, nil
}

func (s *Service) DeleteFieldMap(ctx context.Context, mapID int64) error {
	ct, err := s.pool.Exec(ctx, `DELETE FROM webhook_event_field_map WHERE id=$1`, mapID)
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return httpx.NotFound("field map not found")
	}
	return nil
}

func (s *Service) LatestSamplePayload(ctx context.Context, webhookID int64) (*SamplePayload, error) {
	var payload json.RawMessage
	var receivedAt time.Time
	err := s.pool.QueryRow(ctx,
		`SELECT request_payload, created_at FROM webhook_deliveries
		 WHERE webhook_id=$1 AND status='skipped' AND lead_id IS NULL
		 ORDER BY created_at DESC LIMIT 1`, webhookID).Scan(&payload, &receivedAt)
	if err != nil {
		if !errors.Is(err, pgx.ErrNoRows) {
			return nil, err
		}
		err = s.pool.QueryRow(ctx,
			`SELECT request_payload, created_at FROM webhook_deliveries
			 WHERE webhook_id=$1 ORDER BY created_at DESC LIMIT 1`, webhookID).Scan(&payload, &receivedAt)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return &SamplePayload{Payload: json.RawMessage("null")}, nil
			}
			return nil, err
		}
	}
	return &SamplePayload{Payload: payload, ReceivedAt: &receivedAt}, nil
}

func (s *Service) GetDelivery(ctx context.Context, accountID, webhookID, deliveryID int64) (*Delivery, error) {
	ok, err := s.OwnedBy(ctx, accountID, webhookID)
	if err != nil || !ok {
		return nil, httpx.NotFound("webhook not found")
	}
	d := &Delivery{}
	err = s.pool.QueryRow(ctx,
		`SELECT d.id, d.webhook_id, d.event_id, d.lead_id, l.public_id::text, d.status, d.error_message, d.request_payload, d.created_at
		 FROM webhook_deliveries d
		 LEFT JOIN leads l ON l.id = d.lead_id
		 WHERE d.id=$1 AND d.webhook_id=$2`,
		deliveryID, webhookID).Scan(
		&d.ID, &d.WebhookID, &d.EventID, &d.LeadID, &d.LeadPublicID, &d.Status, &d.ErrorMessage, &d.RequestPayload, &d.CreatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, httpx.NotFound("delivery not found")
		}
		return nil, err
	}
	return d, nil
}

func (s *Service) ListDeliveries(ctx context.Context, webhookID int64, page, limit int) (*DeliveryListResult, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	if page < 1 {
		page = 1
	}
	offset := (page - 1) * limit
	var total int
	if err := s.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM webhook_deliveries WHERE webhook_id=$1`, webhookID).Scan(&total); err != nil {
		return nil, err
	}
	rows, err := s.pool.Query(ctx,
		`SELECT d.id, d.webhook_id, d.event_id, d.lead_id, l.public_id::text, d.status, d.error_message, d.created_at
		 FROM webhook_deliveries d
		 LEFT JOIN leads l ON l.id = d.lead_id
		 WHERE d.webhook_id=$1 ORDER BY d.created_at DESC LIMIT $2 OFFSET $3`,
		webhookID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []Delivery
	for rows.Next() {
		var d Delivery
		if err := rows.Scan(&d.ID, &d.WebhookID, &d.EventID, &d.LeadID, &d.LeadPublicID, &d.Status, &d.ErrorMessage, &d.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, d)
	}
	if items == nil {
		items = []Delivery{}
	}
	return &DeliveryListResult{Items: items, Total: total, Page: page, Limit: limit}, rows.Err()
}

func (s *Service) ListAccountDeliveries(ctx context.Context, accountID int64, p ListAccountDeliveriesParams) (*AccountDeliveryListResult, error) {
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

	var total int
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
		`SELECT d.id, d.webhook_id, w.name, w.slug, d.event_id, d.lead_id, l.public_id::text,
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

	var items []AccountDelivery
	for rows.Next() {
		var d AccountDelivery
		if err := rows.Scan(
			&d.ID, &d.WebhookID, &d.WebhookName, &d.WebhookSlug,
			&d.EventID, &d.LeadID, &d.LeadPublicID, &d.Status, &d.ErrorMessage, &d.CreatedAt,
		); err != nil {
			return nil, err
		}
		items = append(items, d)
	}
	if items == nil {
		items = []AccountDelivery{}
	}
	return &AccountDeliveryListResult{Items: items, Total: total, Page: page, Limit: limit}, rows.Err()
}

type resolvedWebhook struct {
	ID                    int64
	AccountID             int64
	SecretHash            *string
	SecretPrefix          string
	InboundSecretRequired bool
}

// ResolveBySlug loads an active inbound webhook by slug.
func (s *Service) ResolveBySlug(ctx context.Context, slug string) (*resolvedWebhook, error) {
	var w resolvedWebhook
	err := s.pool.QueryRow(ctx,
		`SELECT id, account_id, secret_hash, COALESCE(secret_prefix, ''), inbound_secret_required
		 FROM webhooks
		 WHERE slug=$1 AND is_active AND inbound_enabled`, slug).Scan(
		&w.ID, &w.AccountID, &w.SecretHash, &w.SecretPrefix, &w.InboundSecretRequired)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errors.New("webhook not found")
		}
		return nil, err
	}
	return &w, nil
}

func verifySecretForWebhook(w *resolvedWebhook, full string) bool {
	if w.SecretHash == nil || *w.SecretHash == "" {
		return false
	}
	prefix, _, ok := splitSecret(full)
	if !ok || prefix != w.SecretPrefix {
		return false
	}
	match, _ := auth.VerifyPassword(full, *w.SecretHash)
	return match
}

// VerifySecret resolves a webhook by slug + secret.
func (s *Service) VerifySecret(ctx context.Context, slug, full string) (*WebhookAuth, error) {
	w, err := s.ResolveBySlug(ctx, slug)
	if err != nil {
		return nil, err
	}
	if !w.InboundSecretRequired {
		return nil, errors.New("secret not required")
	}
	if !verifySecretForWebhook(w, full) {
		return nil, errors.New("invalid secret")
	}
	return &WebhookAuth{WebhookID: w.ID, AccountID: w.AccountID}, nil
}

type WebhookAuth struct {
	WebhookID int64
	AccountID int64
}

func validateEvent(p CreateEventParams) error {
	switch p.Action {
	case "create":
		if p.DuplicateMode == nil {
			return httpx.Validation("duplicate_mode required for create action")
		}
		switch *p.DuplicateMode {
		case "update", "duplicate", "reject":
		default:
			return httpx.Validation("duplicate_mode must be update, duplicate, or reject")
		}
	case "update", "delete", "move_stage":
		if p.LookupBy == nil {
			return httpx.Validation("lookup_by required for this action")
		}
		switch *p.LookupBy {
		case "external_id", "public_id", "phone", "email":
		default:
			return httpx.Validation("lookup_by must be external_id, public_id, phone, or email")
		}
	default:
		return httpx.Validation("action must be create, update, delete, or move_stage")
	}
	if p.Action == "move_stage" && (p.TargetStageID == nil || *p.TargetStageID == 0) {
		return httpx.Validation("target_stage_id required for move_stage action")
	}
	return nil
}

func generateSecret() (secret, full, hash, prefix string, err error) {
	secret = randString(32)
	prefix = randString(8)
	full = prefix + "." + secret
	hash, err = auth.HashPassword(full)
	return
}

func randString(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)[:n]
}

func splitSecret(full string) (prefix, secret string, ok bool) {
	for i := 0; i < len(full); i++ {
		if full[i] == '.' {
			return full[:i], full[i+1:], true
		}
	}
	return "", "", false
}
