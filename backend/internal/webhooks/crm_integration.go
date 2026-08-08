package webhooks

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/echayko/leadrula/backend/pkg/httpx"
	"github.com/jackc/pgx/v5"
)

type CRMWebhookIDs struct {
	Inbound     int64  `json:"inbound"`
	InboundSlug string `json:"inbound_webhook_slug"`
}

type crmInboundField struct {
	SourceKey    string
	BuiltinField string
}

var defaultCRMInboundFields = map[string][]crmInboundField{
	"ghl": {
		{SourceKey: "contact_id", BuiltinField: "external_id"},
		{SourceKey: "contactId", BuiltinField: "external_id"},
		{SourceKey: "pipeline_id", BuiltinField: ""},
		{SourceKey: "pipelineStageId", BuiltinField: ""},
		{SourceKey: "stageId", BuiltinField: ""},
	},
	"pipedrive": {
		{SourceKey: "current.person_id", BuiltinField: "external_id"},
		{SourceKey: "current.pipeline_id", BuiltinField: ""},
		{SourceKey: "current.stage_id", BuiltinField: ""},
	},
	"hubspot": {
		{SourceKey: "objectId", BuiltinField: "external_id"},
		{SourceKey: "propertyValue", BuiltinField: ""},
		{SourceKey: "dealstage", BuiltinField: ""},
		{SourceKey: "pipeline", BuiltinField: ""},
		{SourceKey: "pipeline_id", BuiltinField: ""},
	},
	"zoho_crm": {
		{SourceKey: "data.contact_id", BuiltinField: "external_id"},
		{SourceKey: "data.Stage", BuiltinField: ""},
		{SourceKey: "data.Pipeline", BuiltinField: ""},
		{SourceKey: "data.stage_id", BuiltinField: ""},
		{SourceKey: "data.pipeline_id", BuiltinField: ""},
	},
}

// ProvisionCRMInboundWebhook creates an inbound webhook for a CRM integration connection.
func (s *Service) ProvisionCRMInboundWebhook(
	ctx context.Context,
	accountID, connectionID int64,
	connectionPublicID, connectionName, providerSlug string,
) (*CRMWebhookIDs, error) {
	if providerSlug == "ghl" {
		ghlIDs, err := s.ProvisionGHLWebhooks(ctx, accountID, connectionID, connectionPublicID, connectionName)
		if err != nil {
			return nil, err
		}
		return &CRMWebhookIDs{Inbound: ghlIDs.Inbound, InboundSlug: ghlIDs.InboundSlug}, nil
	}

	var accountName string
	if err := s.pool.QueryRow(ctx, `SELECT name FROM accounts WHERE id=$1`, accountID).Scan(&accountName); err != nil {
		return nil, err
	}
	baseSlug, err := crmBaseSlug(accountName, connectionPublicID, providerSlug)
	if err != nil {
		return nil, err
	}
	inboundSlug, err := s.uniqueCRMSlug(ctx, baseSlug)
	if err != nil {
		return nil, err
	}

	falseVal := false
	trueVal := true
	inboundSecretRequired := false
	var integrationConnID *int64
	if connectionID > 0 {
		integrationConnID = &connectionID
	}

	label := strings.ToUpper(providerSlug)
	inbound, _, err := s.Create(ctx, accountID, CreateWebhookInput{
		Name:                    fmt.Sprintf("%s Inbound — %s", label, connectionName),
		Slug:                    inboundSlug,
		InboundEnabled:          &trueVal,
		InboundSecretRequired:   &inboundSecretRequired,
		OutboundEnabled:         &falseVal,
		IntegrationConnectionID: integrationConnID,
	})
	if err != nil {
		return nil, err
	}

	dupUpdate := "update"
	event, err := s.CreateEvent(ctx, inbound.ID, CreateEventParams{
		Action:        "create",
		DuplicateMode: &dupUpdate,
		Conditions:    json.RawMessage(`[]`),
	})
	if err != nil {
		_ = s.Delete(ctx, accountID, inbound.ID)
		return nil, err
	}

	fields := defaultCRMInboundFields[providerSlug]
	seen := map[string]bool{}
	for _, f := range fields {
		if f.BuiltinField == "" {
			continue
		}
		if seen[f.SourceKey] {
			continue
		}
		seen[f.SourceKey] = true
		bf := f.BuiltinField
		if _, err := s.AddFieldMap(ctx, event.ID, f.SourceKey, "builtin", &bf, nil); err != nil {
			_ = s.Delete(ctx, accountID, inbound.ID)
			return nil, err
		}
	}

	return &CRMWebhookIDs{Inbound: inbound.ID, InboundSlug: inboundSlug}, nil
}

func ParseCRMWebhookIDs(config any) CRMWebhookIDs {
	ids := ParseGHLWebhookIDs(config)
	return CRMWebhookIDs{Inbound: ids.Inbound, InboundSlug: ids.InboundSlug}
}

func MergeCRMConfig(config map[string]any, ids *CRMWebhookIDs) map[string]any {
	if config == nil {
		config = map[string]any{}
	}
	if ids != nil {
		config["webhook_ids"] = map[string]any{
			"inbound": ids.Inbound,
		}
		config["inbound_webhook_id"] = ids.Inbound
		config["inbound_webhook_slug"] = ids.InboundSlug
	}
	return config
}

func crmBaseSlug(accountName, connectionPublicID, providerSlug string) (string, error) {
	slug := strings.ToLower(accountName)
	slug = regexp.MustCompile(`[^a-z0-9]+`).ReplaceAllString(slug, "-")
	slug = strings.Trim(slug, "-")
	if slug == "" {
		slug = "account"
	}
	short := connectionPublicID
	if len(short) > 8 {
		short = short[:8]
	}
	short = strings.ToLower(regexp.MustCompile(`[^a-z0-9]+`).ReplaceAllString(short, ""))
	if short == "" {
		return "", httpx.Validation("invalid connection id for webhook slug")
	}
	provider := strings.ReplaceAll(providerSlug, "_", "-")
	return slug + "-" + provider + "-" + short, nil
}

func (s *Service) uniqueCRMSlug(ctx context.Context, base string) (string, error) {
	for i := range 20 {
		candidate := base
		if i > 0 {
			candidate = fmt.Sprintf("%s-%d", base, i+1)
		}
		var exists bool
		if err := s.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM webhooks WHERE slug=$1)`, candidate).Scan(&exists); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return candidate, nil
			}
			return "", err
		}
		if !exists {
			return candidate, nil
		}
	}
	return "", httpx.Conflict("could not generate unique webhook slug")
}

func (s *Service) DeleteCRMWebhooks(ctx context.Context, accountID int64, ids CRMWebhookIDs) {
	if ids.Inbound > 0 {
		_ = s.Delete(ctx, accountID, ids.Inbound)
	}
}

type crmBindingFieldMap struct {
	customFieldID    int64
	inboundSourceKey string
}

// SyncCRMBindingFieldMaps adds inbound webhook field maps for imported CRM custom field bindings.
func (s *Service) SyncCRMBindingFieldMaps(ctx context.Context, connectionID int64) error {
	if connectionID <= 0 {
		return nil
	}
	var webhookID int64
	var accountID int64
	var providerSlug *string
	err := s.pool.QueryRow(ctx,
		`SELECT w.id, w.account_id, ip.slug FROM webhooks w
		 JOIN integration_connections ic ON ic.id = w.integration_connection_id
		 JOIN integration_providers ip ON ip.id = ic.provider_id
		 WHERE w.integration_connection_id = $1 AND w.inbound_enabled
		 ORDER BY w.id LIMIT 1`, connectionID).Scan(&webhookID, &accountID, &providerSlug)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}

	rows, err := s.pool.Query(ctx,
		`SELECT custom_field_id, inbound_source_key FROM custom_field_crm_bindings
		 WHERE connection_id = $1 AND account_id = $2`, connectionID, accountID)
	if err != nil {
		return err
	}
	defer rows.Close()
	var bindings []crmBindingFieldMap
	for rows.Next() {
		var b crmBindingFieldMap
		if err := rows.Scan(&b.customFieldID, &b.inboundSourceKey); err != nil {
			return err
		}
		bindings = append(bindings, b)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if len(bindings) == 0 {
		return nil
	}

	createEventID, err := s.eventIDByAction(ctx, webhookID, "create")
	if err != nil {
		return err
	}
	if createEventID == 0 {
		return nil
	}
	if err := s.mergeCRMBindingFieldMaps(ctx, createEventID, bindings); err != nil {
		return err
	}

	updateEventID, err := s.eventIDByAction(ctx, webhookID, "update")
	if err != nil {
		return err
	}
	if updateEventID == 0 && providerSlug != nil && *providerSlug != "ghl" {
		dupUpdate := "update"
		event, err := s.CreateEvent(ctx, webhookID, CreateEventParams{
			Action:        "update",
			DuplicateMode: &dupUpdate,
			Conditions:    json.RawMessage(`[]`),
		})
		if err != nil {
			return err
		}
		updateEventID = event.ID
	}
	if updateEventID > 0 {
		return s.mergeCRMBindingFieldMaps(ctx, updateEventID, bindings)
	}
	return nil
}

func (s *Service) eventIDByAction(ctx context.Context, webhookID int64, action string) (int64, error) {
	var eventID int64
	err := s.pool.QueryRow(ctx,
		`SELECT id FROM webhook_events WHERE webhook_id=$1 AND action=$2 ORDER BY id LIMIT 1`,
		webhookID, action).Scan(&eventID)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, nil
	}
	return eventID, err
}

func (s *Service) mergeCRMBindingFieldMaps(ctx context.Context, eventID int64, bindings []crmBindingFieldMap) error {
	existing, err := s.ListFieldMap(ctx, eventID)
	if err != nil {
		return err
	}
	seen := map[string]int64{}
	for _, m := range existing {
		if m.TargetType == "custom" && m.CustomFieldID != nil {
			seen[m.SourceKey] = *m.CustomFieldID
		}
	}
	for _, b := range bindings {
		key := strings.TrimSpace(b.inboundSourceKey)
		if key == "" {
			continue
		}
		if fid, ok := seen[key]; ok && fid == b.customFieldID {
			continue
		}
		id := b.customFieldID
		if _, err := s.AddFieldMap(ctx, eventID, key, "custom", nil, &id); err != nil {
			return err
		}
		seen[key] = b.customFieldID
	}
	return nil
}
