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
	return s.ingestPayload(ctx, wa, slug, raw, rawJSON, false)
}

// ReplayDelivery re-processes a stored unprocessed delivery.
func (s *Service) ReplayDelivery(ctx context.Context, accountID, webhookID, deliveryID int64) (*IngestResult, error) {
	var status string
	var leadID *int64
	var payload json.RawMessage
	err := s.pool.QueryRow(ctx,
		`SELECT status, lead_id, request_payload FROM webhook_deliveries
		 WHERE id=$1 AND webhook_id=$2`, deliveryID, webhookID).Scan(&status, &leadID, &payload)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, httpx.NotFound("delivery not found")
		}
		return nil, err
	}
	ok, err := s.OwnedBy(ctx, accountID, webhookID)
	if err != nil || !ok {
		return nil, httpx.NotFound("webhook not found")
	}
	if status != "skipped" || leadID != nil {
		return nil, httpx.Validation("only unprocessed deliveries can be replayed")
	}
	var raw map[string]any
	if err := json.Unmarshal(payload, &raw); err != nil {
		return nil, httpx.Validation("invalid stored payload")
	}
	var slug string
	if err := s.pool.QueryRow(ctx, `SELECT slug FROM webhooks WHERE id=$1`, webhookID).Scan(&slug); err != nil {
		return nil, err
	}
	res, err := s.ingestPayload(ctx, &WebhookAuth{WebhookID: webhookID, AccountID: accountID}, slug, raw, payload, true)
	if err != nil {
		msg := err.Error()
		if appErr, ok := err.(*httpx.AppError); ok {
			msg = appErr.Message
		}
		_, _ = s.pool.Exec(ctx,
			`UPDATE webhook_deliveries SET status='error', error_message=$2 WHERE id=$1`,
			deliveryID, msg)
		return nil, err
	}
	var eventID *int64
	var newLeadID *int64
	if len(res.Results) > 0 {
		if res.Results[0].ActionID != 0 {
			id := res.Results[0].ActionID
			eventID = &id
		}
		if res.Results[0].LeadInternalID != 0 {
			id := res.Results[0].LeadInternalID
			newLeadID = &id
		}
	}
	_, _ = s.pool.Exec(ctx,
		`UPDATE webhook_deliveries SET status='success', event_id=$2, lead_id=$3, error_message=NULL WHERE id=$1`,
		deliveryID, eventID, newLeadID)
	return res, nil
}

func (s *Service) ingestPayload(ctx context.Context, wa *WebhookAuth, slug string, raw map[string]any, rawJSON []byte, forceProcess bool) (*IngestResult, error) {
	flat := flattenPayload(raw)

	var webhook Webhook
	err := s.pool.QueryRow(ctx,
		`SELECT id, account_id, name, slug, COALESCE(secret_prefix, ''), is_active, created_at, integration_connection_id
		 FROM webhooks WHERE id=$1 AND slug=$2 AND is_active`,
		wa.WebhookID, slug).Scan(
		&webhook.ID, &webhook.AccountID, &webhook.Name, &webhook.Slug, &webhook.SecretPrefix,
		&webhook.IsActive, &webhook.CreatedAt, &webhook.IntegrationConnectionID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, httpx.NotFound("webhook not found")
		}
		return nil, err
	}

	actions, err := s.listMatchingActions(ctx, webhook.ID, flat)
	if err != nil {
		return nil, err
	}

	var ready []*WebhookEvent
	for _, a := range actions {
		if actionReady(ctx, s, a, flat) {
			ready = append(ready, a)
		}
	}

	if !forceProcess && len(ready) == 0 {
		deliveryID, _ := s.logDelivery(ctx, webhook.ID, nil, nil, "skipped", rawJSON, "awaiting configuration")
		return &IngestResult{Status: "captured", DeliveryID: deliveryID}, nil
	}

	if forceProcess && len(ready) == 0 {
		return nil, httpx.Validation("no matching configured actions")
	}

	var results []ActionResult
	var firstLeadID *int64
	var firstEventID *int64
	for _, action := range ready {
		result, leadID, execErr := s.executeEvent(ctx, webhook.AccountID, action, flat, rawJSON)
		if execErr != nil {
			if !forceProcess {
				msg := execErr.Error()
				if appErr, ok := execErr.(*httpx.AppError); ok {
					msg = appErr.Message
				}
				s.logDelivery(ctx, webhook.ID, &action.ID, leadID, "error", rawJSON, msg)
			}
			return nil, execErr
		}
		ar := ActionResult{
			LeadID:         result.LeadID,
			Action:         result.Action,
			Status:         result.Status,
			ActionID:       action.ID,
			LeadInternalID: 0,
		}
		if leadID != nil {
			ar.LeadInternalID = *leadID
			if firstLeadID == nil {
				firstLeadID = leadID
				firstEventID = &action.ID
			}
		}
		results = append(results, ar)
	}

	if !forceProcess {
		s.logDelivery(ctx, webhook.ID, firstEventID, firstLeadID, "success", rawJSON, "")
	}
	for _, ar := range results {
		if ar.LeadInternalID != 0 {
			applyInboundOriginRoutes(ctx, s.leadSvc, webhook, ar.LeadInternalID, flat)
		}
	}
	return &IngestResult{Status: "processed", Results: results}, nil
}

type inboundOriginApplier interface {
	TryApplyConnectionOriginRoute(ctx context.Context, accountID, connectionID, leadID int64, payloadFlat map[string]any) bool
	TryApplyWebhookOriginRoute(ctx context.Context, accountID, webhookID, leadID int64, payloadFlat map[string]any) bool
}

func applyInboundOriginRoutes(ctx context.Context, svc inboundOriginApplier, webhook Webhook, leadID int64, payloadFlat map[string]any) {
	if svc == nil || leadID == 0 {
		return
	}
	applied := false
	if webhook.IntegrationConnectionID != nil && *webhook.IntegrationConnectionID != 0 {
		applied = svc.TryApplyConnectionOriginRoute(ctx, webhook.AccountID, *webhook.IntegrationConnectionID, leadID, payloadFlat)
	}
	if !applied {
		svc.TryApplyWebhookOriginRoute(ctx, webhook.AccountID, webhook.ID, leadID, payloadFlat)
	}
}

func (s *Service) listMatchingActions(ctx context.Context, webhookID int64, flat map[string]any) ([]*WebhookEvent, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT id, webhook_id, action, duplicate_mode, lookup_by, lookup_source_key,
		        target_stage_id, target_pipeline_id, position, condition_logic, conditions, created_at
		 FROM webhook_events WHERE webhook_id=$1 ORDER BY position, id`, webhookID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var matched []*WebhookEvent
	for rows.Next() {
		e, err := scanWebhookEvent(rows)
		if err != nil {
			return nil, err
		}
		conds, err := parsePayloadConditions(e.Conditions)
		if err != nil {
			continue
		}
		if evalPayloadConditions(conds, e.ConditionLogic, flat) {
			matched = append(matched, e)
		}
	}
	return matched, rows.Err()
}

func actionReady(ctx context.Context, s *Service, action *WebhookEvent, flat map[string]any) bool {
	switch action.Action {
	case "create", "update":
		maps, err := s.ListFieldMap(ctx, action.ID)
		return err == nil && len(maps) > 0
	case "delete", "move_stage":
		if action.LookupBy == nil {
			return false
		}
		maps, _ := s.ListFieldMap(ctx, action.ID)
		return lookupValue(action, flat, maps) != ""
	default:
		return false
	}
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
				if err := s.applyMappedFields(ctx, s.leads.Pool(), accountID, existing.ID, flat, maps, builtins, customs); err != nil {
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
	if err := s.applyMappedFields(ctx, tx, accountID, leadID, flat, maps, builtins, customs); err != nil {
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
	if err := s.applyMappedFields(ctx, s.leads.Pool(), accountID, lead.ID, flat, maps, builtins, customs); err != nil {
		return nil, &lead.ID, err
	}
	return &IngestResult{LeadID: lead.PublicID, Action: "update", Status: "updated"}, &lead.ID, nil
}

func (s *Service) execDelete(ctx context.Context, accountID int64, event *WebhookEvent, flat map[string]any, maps []FieldMapEntry) (*IngestResult, *int64, error) {
	lead, err := s.resolveLead(ctx, accountID, event, flat, maps)
	if err != nil {
		return nil, nil, err
	}
	_ = leads.LoadCustomValues(ctx, s.leads.Pool(), lead)
	if err := s.leads.SoftDelete(ctx, s.leads.Pool(), accountID, lead.ID); err != nil {
		return nil, &lead.ID, err
	}
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
	lookupVal := lookupValue(event, flat, maps)
	if lookupVal == "" {
		return nil, httpx.Validation("lookup value missing from payload")
	}
	switch *event.LookupBy {
	case "external_id":
		return s.leads.GetByExternalID(ctx, s.leads.Pool(), accountID, lookupVal)
	case "public_id":
		return s.leads.GetByPublicID(ctx, s.leads.Pool(), accountID, lookupVal)
	case "phone":
		return s.leads.GetByPhone(ctx, s.leads.Pool(), accountID, lookupVal)
	case "email":
		return s.leads.GetByEmail(ctx, s.leads.Pool(), accountID, lookupVal)
	default:
		return nil, httpx.Validation("invalid lookup_by")
	}
}

func lookupValue(event *WebhookEvent, flat map[string]any, maps []FieldMapEntry) string {
	if event.LookupBy == nil {
		return ""
	}
	lookupBy := *event.LookupBy
	if event.LookupSourceKey != nil && *event.LookupSourceKey != "" {
		if v, ok := flat[*event.LookupSourceKey]; ok {
			return toText(v)
		}
	}
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

func (s *Service) applyMappedFields(ctx context.Context, q database.Querier, accountID, leadID int64, flat map[string]any, maps []FieldMapEntry, builtins map[string]string, customs map[int64]json.RawMessage) error {
	for field, val := range builtins {
		if val == "" {
			continue
		}
		if field == "action_at" || field == "disqualification_reason_id" || field == "tags" || leads.IsMoneyBuiltin(field) {
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
	for _, m := range maps {
		if m.TargetType != "builtin" || m.BuiltinField == nil {
			continue
		}
		v, ok := flat[m.SourceKey]
		if !ok {
			continue
		}
		switch *m.BuiltinField {
		case "tags":
			if err := leads.ApplyMappedTags(ctx, q, s.leads, accountID, leadID, v); err != nil {
				return err
			}
		default:
			if leads.IsMoneyBuiltin(*m.BuiltinField) {
				if err := leads.ApplyMappedBuiltin(ctx, q, s.leads, leadID, *m.BuiltinField, v); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func (s *Service) logDelivery(ctx context.Context, webhookID int64, eventID, leadID *int64, status string, payload []byte, errMsg string) (int64, error) {
	var msg *string
	if errMsg != "" {
		msg = &errMsg
	}
	var id int64
	err := s.pool.QueryRow(ctx,
		`INSERT INTO webhook_deliveries(webhook_id, event_id, lead_id, status, request_payload, error_message)
		 VALUES ($1,$2,$3,$4,$5,$6) RETURNING id`,
		webhookID, eventID, leadID, status, payload, msg).Scan(&id)
	return id, err
}
