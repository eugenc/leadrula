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

type GHLWebhookIDs struct {
	Inbound     int64  `json:"inbound"`
	InboundSlug string `json:"inbound_webhook_slug"`
}

type GHLInboundField struct {
	SourceKey    string
	BuiltinField string
}

var defaultGHLInboundFields = []GHLInboundField{
	{SourceKey: "firstName", BuiltinField: "first_name"},
	{SourceKey: "first_name", BuiltinField: "first_name"},
	{SourceKey: "lastName", BuiltinField: "last_name"},
	{SourceKey: "last_name", BuiltinField: "last_name"},
	{SourceKey: "phone", BuiltinField: "phone"},
	{SourceKey: "email", BuiltinField: "email"},
	{SourceKey: "address1", BuiltinField: "address"},
	{SourceKey: "address", BuiltinField: "address"},
	{SourceKey: "city", BuiltinField: "city"},
	{SourceKey: "state", BuiltinField: "state"},
	{SourceKey: "postalCode", BuiltinField: "zip"},
	{SourceKey: "zip", BuiltinField: "zip"},
	{SourceKey: "id", BuiltinField: "external_id"},
	{SourceKey: "contactId", BuiltinField: "external_id"},
	{SourceKey: "contact_id", BuiltinField: "external_id"},
	{SourceKey: "source", BuiltinField: "source"},
}

func ghlInboundContactID(flat map[string]any) string {
	for _, key := range []string{"contact_id", "contactId", "id"} {
		v, ok := flat[key]
		if !ok || v == nil {
			continue
		}
		if s := strings.TrimSpace(toText(v)); s != "" && s != "null" {
			return s
		}
	}
	return ""
}

// ProvisionGHLWebhooks creates an inbound webhook for a GHL connection.
func (s *Service) ProvisionGHLWebhooks(
	ctx context.Context,
	accountID int64,
	connectionID int64,
	connectionPublicID string,
	connectionName string,
) (*GHLWebhookIDs, error) {
	var accountName string
	if err := s.pool.QueryRow(ctx, `SELECT name FROM accounts WHERE id=$1`, accountID).Scan(&accountName); err != nil {
		return nil, err
	}

	baseSlug, err := ghlBaseSlug(accountName, connectionPublicID)
	if err != nil {
		return nil, err
	}
	inboundSlug, err := s.uniqueGHLSlug(ctx, baseSlug)
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

	inbound, _, err := s.Create(ctx, accountID, CreateWebhookInput{
		Name:                    fmt.Sprintf("GHL Inbound — %s", connectionName),
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
	seen := map[string]bool{}
	for _, f := range defaultGHLInboundFields {
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

	return &GHLWebhookIDs{
		Inbound:     inbound.ID,
		InboundSlug: inboundSlug,
	}, nil
}

// SyncGHLInboundEvent ensures inbound field maps for GHL contact id keys.
func (s *Service) SyncGHLInboundEvent(ctx context.Context, inboundWebhookID int64) error {
	if inboundWebhookID <= 0 {
		return nil
	}
	return s.ensureGHLInboundContactIDFieldMap(ctx, inboundWebhookID)
}

// SyncAllGHLInboundWebhooks repairs inbound field maps for every GHL connection.
func (s *Service) SyncAllGHLInboundWebhooks(ctx context.Context) error {
	rows, err := s.pool.Query(ctx,
		`SELECT w.id FROM webhooks w
		 JOIN integration_connections ic ON ic.id = w.integration_connection_id
		 JOIN integration_providers ip ON ip.id = ic.provider_id
		 WHERE ip.slug = 'ghl' AND w.inbound_enabled`)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var webhookID int64
		if err := rows.Scan(&webhookID); err != nil {
			return err
		}
		if err := s.SyncGHLInboundEvent(ctx, webhookID); err != nil {
			return err
		}
	}
	return rows.Err()
}

func (s *Service) ensureGHLInboundContactIDFieldMap(ctx context.Context, inboundWebhookID int64) error {
	var eventID int64
	err := s.pool.QueryRow(ctx,
		`SELECT id FROM webhook_events WHERE webhook_id=$1 AND action='create' ORDER BY id LIMIT 1`,
		inboundWebhookID).Scan(&eventID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	var hasContactID bool
	if err := s.pool.QueryRow(ctx,
		`SELECT EXISTS(
		   SELECT 1 FROM webhook_event_field_map
		   WHERE event_id=$1 AND source_key='contact_id' AND target_type='builtin' AND builtin_field='external_id'
		 )`, eventID).Scan(&hasContactID); err != nil {
		return err
	}
	if hasContactID {
		return nil
	}
	bf := "external_id"
	_, err = s.AddFieldMap(ctx, eventID, "contact_id", "builtin", &bf, nil)
	return err
}

func (s *Service) DeleteGHLWebhooks(ctx context.Context, accountID int64, ids GHLWebhookIDs) {
	if ids.Inbound > 0 {
		_ = s.Delete(ctx, accountID, ids.Inbound)
	}
}

func ParseGHLWebhookIDs(config any) GHLWebhookIDs {
	var out GHLWebhookIDs
	m, ok := config.(map[string]any)
	if !ok {
		if b, err := json.Marshal(config); err == nil {
			var cfg map[string]any
			if json.Unmarshal(b, &cfg) == nil {
				m = cfg
			}
		}
	}
	if m == nil {
		return out
	}
	if slug, ok := m["inbound_webhook_slug"].(string); ok {
		out.InboundSlug = slug
	}
	if id := int64FromAny(m["inbound_webhook_id"]); id > 0 {
		out.Inbound = id
		return out
	}
	wh, ok := m["webhook_ids"].(map[string]any)
	if !ok {
		return out
	}
	out.Inbound = int64FromAny(wh["inbound"])
	return out
}

func MergeGHLConfig(config map[string]any, ids *GHLWebhookIDs) map[string]any {
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

func ghlBaseSlug(accountName, connectionPublicID string) (string, error) {
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
	return slug + "-ghl-" + short, nil
}

func (s *Service) uniqueGHLSlug(ctx context.Context, base string) (string, error) {
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
