package webhooks

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/echayko/leadrula/backend/internal/database"
	"github.com/echayko/leadrula/backend/internal/leads"
	"github.com/echayko/leadrula/backend/pkg/httpx"
	"github.com/jackc/pgx/v5"
)

var builtinKeys = []string{
	"first_name", "last_name", "phone", "email", "address", "city", "state", "zip",
	"source", "external_id",
}

func flattenPayload(raw map[string]any) map[string]any {
	out := map[string]any{}
	for k, v := range raw {
		if k == "custom" {
			continue
		}
		out[k] = v
	}
	if custom, ok := raw["custom"].(map[string]any); ok {
		for k, v := range custom {
			out[k] = v
		}
	}
	return out
}

func toText(v any) string {
	switch x := v.(type) {
	case string:
		return strings.TrimSpace(x)
	case float64:
		if x == float64(int64(x)) {
			return strconv.FormatInt(int64(x), 10)
		}
		return strconv.FormatFloat(x, 'f', -1, 64)
	case bool:
		if x {
			return "true"
		}
		return "false"
	default:
		b, _ := json.Marshal(v)
		return strings.Trim(string(b), `"`)
	}
}

// Ingest handles POST /api/v1/webhooks/{slug}.
func (s *Service) Ingest(ctx context.Context, wa *WebhookAuth, slug string, raw map[string]any) (*IngestResult, error) {
	rawJSON, _ := json.Marshal(raw)
	flat := flattenPayload(raw)

	var webhook Webhook
	err := s.pool.QueryRow(ctx,
		`SELECT id, account_id, name, slug, secret_prefix, is_active, created_at
		 FROM webhooks WHERE id=$1 AND slug=$2 AND is_active`,
		wa.WebhookID, slug).Scan(
		&webhook.ID, &webhook.AccountID, &webhook.Name, &webhook.Slug, &webhook.SecretPrefix,
		&webhook.IsActive, &webhook.CreatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, httpx.NotFound("webhook not found")
		}
		return nil, err
	}

	eventKey := toText(flat["event"])
	if eventKey == "" {
		s.logDelivery(ctx, webhook.ID, nil, nil, "skipped", rawJSON, "missing event field")
		return nil, httpx.Validation(`payload field "event" is required`)
	}

	event, err := s.matchEvent(ctx, webhook.ID, eventKey)
	if err != nil {
		s.logDelivery(ctx, webhook.ID, nil, nil, "skipped", rawJSON, err.Error())
		return nil, err
	}

	result, leadID, execErr := s.executeEvent(ctx, webhook.AccountID, event, flat, rawJSON)
	if execErr != nil {
		msg := execErr.Error()
		if appErr, ok := execErr.(*httpx.AppError); ok {
			msg = appErr.Message
		}
		s.logDelivery(ctx, webhook.ID, &event.ID, leadID, "error", rawJSON, msg)
		return nil, execErr
	}
	s.logDelivery(ctx, webhook.ID, &event.ID, leadID, "success", rawJSON, "")
	return result, nil
}

func (s *Service) matchEvent(ctx context.Context, webhookID int64, eventKey string) (*WebhookEvent, error) {
	e := &WebhookEvent{}
	err := s.pool.QueryRow(ctx,
		`SELECT id, webhook_id, event_key, action, duplicate_mode, lookup_by,
		        target_stage_id, target_pipeline_id, position, created_at
		 FROM webhook_events WHERE webhook_id=$1 AND event_key=$2
		 ORDER BY position, id LIMIT 1`,
		webhookID, eventKey).Scan(
		&e.ID, &e.WebhookID, &e.EventKey, &e.Action, &e.DuplicateMode, &e.LookupBy,
		&e.TargetStageID, &e.TargetPipelineID, &e.Position, &e.CreatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, httpx.Validation("no matching event configured")
		}
		return nil, err
	}
	return e, nil
}

func (s *Service) executeEvent(ctx context.Context, accountID int64, event *WebhookEvent, flat map[string]any, rawJSON []byte) (*IngestResult, *int64, error) {
	maps, err := s.ListFieldMap(ctx, event.ID)
	if err != nil {
		return nil, nil, err
	}

	switch event.Action {
	case "create":
		return s.execCreate(ctx, accountID, event, flat, rawJSON, maps)
	case "update":
		return s.execUpdate(ctx, accountID, event, flat, maps)
	case "delete":
		return s.execDelete(ctx, accountID, event, flat, maps)
	case "move_stage":
		return s.execMoveStage(ctx, accountID, event, flat, maps)
	default:
		return nil, nil, httpx.Validation("unsupported action")
	}
}

func (s *Service) execCreate(ctx context.Context, accountID int64, event *WebhookEvent, flat map[string]any, rawJSON []byte, maps []FieldMapEntry) (*IngestResult, *int64, error) {
	builtins, customs, externalID := applyFieldMaps(flat, maps)

	if externalID != "" {
		existing, err := s.leads.GetByExternalID(ctx, s.leads.Pool(), accountID, externalID)
		if err == nil && existing != nil {
			switch *event.DuplicateMode {
			case "reject":
				return nil, &existing.ID, httpx.Conflict("lead with this external_id already exists")
			case "update":
				if err := s.applyMappedFields(ctx, s.leads.Pool(), accountID, existing.ID, builtins, customs); err != nil {
					return nil, &existing.ID, err
				}
				return &IngestResult{LeadID: existing.PublicID, Action: "update", Status: "updated"}, &existing.ID, nil
			case "duplicate":
				// fall through to create
			}
		} else if err != nil {
			var appErr *httpx.AppError
			if !errors.As(err, &appErr) || appErr.Code != httpx.CodeNotFound {
				return nil, nil, err
			}
		}
	}

	tx, err := s.leads.Pool().Begin(ctx)
	if err != nil {
		return nil, nil, err
	}
	defer tx.Rollback(ctx)

	source := ""
	if v, ok := builtins["source"]; ok {
		source = v
	}
	leadID, publicID, err := s.leads.InsertLead(ctx, tx, accountID, accountID, source, rawJSON)
	if err != nil {
		return nil, nil, err
	}

	for _, k := range builtinKeys {
		if v, ok := flat[k]; ok {
			if str := toText(v); str != "" {
				if err := s.leads.SetBuiltinField(ctx, tx, leadID, k, str); err != nil {
					return nil, nil, err
				}
			}
		}
	}
	if err := s.applyMappedFields(ctx, tx, accountID, leadID, builtins, customs); err != nil {
		return nil, &leadID, err
	}

	if event.TargetPipelineID != nil && *event.TargetPipelineID != 0 {
		var stageID int64
		err := tx.QueryRow(ctx,
			`SELECT id FROM pipeline_stages WHERE pipeline_id=$1 ORDER BY position, id LIMIT 1`,
			*event.TargetPipelineID).Scan(&stageID)
		if err != nil {
			return nil, &leadID, httpx.Validation("target pipeline has no stages")
		}
		stage, err := s.leads.GetStage(ctx, tx, stageID)
		if err != nil {
			return nil, &leadID, err
		}
		if stage.AccountID != accountID {
			return nil, &leadID, httpx.Validation("target pipeline does not belong to this account")
		}
		if err := s.leads.PlaceInPipeline(ctx, tx, leadID, accountID, *event.TargetPipelineID, stageID, nil); err != nil {
			return nil, &leadID, err
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, &leadID, err
	}
	return &IngestResult{LeadID: publicID, Action: "create", Status: "created"}, &leadID, nil
}

func (s *Service) execUpdate(ctx context.Context, accountID int64, event *WebhookEvent, flat map[string]any, maps []FieldMapEntry) (*IngestResult, *int64, error) {
	lead, err := s.resolveLead(ctx, accountID, event, flat, maps)
	if err != nil {
		return nil, nil, err
	}
	builtins, customs, _ := applyFieldMaps(flat, maps)
	if err := s.applyMappedFields(ctx, s.leads.Pool(), accountID, lead.ID, builtins, customs); err != nil {
		return nil, &lead.ID, err
	}
	return &IngestResult{LeadID: lead.PublicID, Action: "update", Status: "updated"}, &lead.ID, nil
}

func (s *Service) execDelete(ctx context.Context, accountID int64, event *WebhookEvent, flat map[string]any, maps []FieldMapEntry) (*IngestResult, *int64, error) {
	lead, err := s.resolveLead(ctx, accountID, event, flat, maps)
	if err != nil {
		return nil, nil, err
	}
	// Load custom values before soft-delete so the fire call has a complete snapshot.
	_ = leads.LoadCustomValues(ctx, s.leads.Pool(), lead)
	if err := s.leads.SoftDelete(ctx, s.leads.Pool(), accountID, lead.ID); err != nil {
		return nil, &lead.ID, err
	}
	// Fire outbound webhook for lead deletion.
	s.FireOutbound(ctx, accountID, EventLeadDelete, lead, leads.PipelineContext{})
	return &IngestResult{LeadID: lead.PublicID, Action: "delete", Status: "deleted"}, &lead.ID, nil
}

func (s *Service) execMoveStage(ctx context.Context, accountID int64, event *WebhookEvent, flat map[string]any, maps []FieldMapEntry) (*IngestResult, *int64, error) {
	lead, err := s.resolveLead(ctx, accountID, event, flat, maps)
	if err != nil {
		return nil, nil, err
	}
	if event.TargetStageID == nil {
		return nil, &lead.ID, httpx.Validation("target_stage_id not configured")
	}

	var actionAt *time.Time
	var disqReasonID *int64
	builtins, _, _ := applyFieldMaps(flat, maps)
	if v, ok := builtins["action_at"]; ok && v != "" {
		t, err := time.Parse(time.RFC3339, v)
		if err != nil {
			t, err = time.Parse("2006-01-02", v)
			if err != nil {
				return nil, &lead.ID, httpx.Validation("invalid action_at value")
			}
		}
		actionAt = &t
	}
	if v, ok := builtins["disqualification_reason_id"]; ok && v != "" {
		id, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			return nil, &lead.ID, httpx.Validation("invalid disqualification_reason_id")
		}
		disqReasonID = &id
	}

	updated, err := s.leadSvc.ChangeStageByWebhook(ctx, accountID, lead.ID, *event.TargetStageID, actionAt, disqReasonID)
	if err != nil {
		return nil, &lead.ID, err
	}
	return &IngestResult{LeadID: updated.PublicID, Action: "move_stage", Status: "moved"}, &lead.ID, nil
}

func (s *Service) resolveLead(ctx context.Context, accountID int64, event *WebhookEvent, flat map[string]any, maps []FieldMapEntry) (*leads.Lead, error) {
	if event.LookupBy == nil {
		return nil, httpx.Validation("lookup_by not configured")
	}
	lookupVal := lookupValue(*event.LookupBy, flat, maps)
	if lookupVal == "" {
		return nil, httpx.Validation("lookup value missing from payload")
	}
	switch *event.LookupBy {
	case "external_id":
		return s.leads.GetByExternalID(ctx, s.leads.Pool(), accountID, lookupVal)
	case "public_id":
		return s.leads.GetByPublicID(ctx, s.leads.Pool(), accountID, lookupVal)
	default:
		return nil, httpx.Validation("invalid lookup_by")
	}
}

func lookupValue(lookupBy string, flat map[string]any, maps []FieldMapEntry) string {
	for _, m := range maps {
		if m.TargetType == "builtin" && m.BuiltinField != nil && *m.BuiltinField == lookupBy {
			if v, ok := flat[m.SourceKey]; ok {
				return toText(v)
			}
		}
	}
	if v, ok := flat[lookupBy]; ok {
		return toText(v)
	}
	return ""
}

func applyFieldMaps(flat map[string]any, maps []FieldMapEntry) (builtins map[string]string, customs map[int64]json.RawMessage, externalID string) {
	builtins = map[string]string{}
	customs = map[int64]json.RawMessage{}
	for _, m := range maps {
		v, ok := flat[m.SourceKey]
		if !ok {
			continue
		}
		if m.TargetType == "builtin" && m.BuiltinField != nil {
			builtins[*m.BuiltinField] = toText(v)
			if *m.BuiltinField == "external_id" {
				externalID = toText(v)
			}
		} else if m.TargetType == "custom" && m.CustomFieldID != nil {
			valJSON, _ := json.Marshal(v)
			customs[*m.CustomFieldID] = valJSON
		}
	}
	return builtins, customs, externalID
}

func (s *Service) applyMappedFields(ctx context.Context, q database.Querier, accountID, leadID int64, builtins map[string]string, customs map[int64]json.RawMessage) error {
	for field, val := range builtins {
		if val == "" {
			continue
		}
		if field == "action_at" || field == "disqualification_reason_id" {
			continue
		}
		if err := s.leads.SetBuiltinField(ctx, q, leadID, field, val); err != nil {
			return err
		}
	}
	for fid, val := range customs {
		if err := s.leads.UpsertCustomValue(ctx, q, leadID, fid, val); err != nil {
			return err
		}
	}
	if tags, ok := builtins["tags"]; ok && tags != "" {
		var tagList []string
		for _, part := range strings.Split(tags, ",") {
			if t := strings.TrimSpace(part); t != "" {
				tagList = append(tagList, t)
			}
		}
		if len(tagList) > 0 {
			if err := s.leads.SetTags(ctx, accountID, leadID, tagList); err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *Service) logDelivery(ctx context.Context, webhookID int64, eventID, leadID *int64, status string, payload []byte, errMsg string) {
	var msg *string
	if errMsg != "" {
		msg = &errMsg
	}
	_, _ = s.pool.Exec(ctx,
		`INSERT INTO webhook_deliveries(webhook_id, event_id, lead_id, status, request_payload, error_message)
		 VALUES ($1,$2,$3,$4,$5,$6)`,
		webhookID, eventID, leadID, status, payload, msg)
}
