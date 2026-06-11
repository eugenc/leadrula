package webhooks

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/echayko/leadrula/backend/pkg/httpx"
)

const sunbaseDefaultEndpoint = "https://server4.sunbasedata.com/sunbase/portal/api/lead_post.jsp"

type SunbaseWebhookIDs struct {
	OutboundPost int64  `json:"outbound_post"`
	OutboundGet  int64  `json:"outbound_get"`
	Inbound      int64  `json:"inbound"`
	InboundSlug  string `json:"inbound_webhook_slug"`
}

type SunbaseInboundField struct {
	SourceKey    string
	BuiltinField string
}

var defaultSunbaseInboundFields = []SunbaseInboundField{
	{SourceKey: "first_name", BuiltinField: "first_name"},
	{SourceKey: "last_name", BuiltinField: "last_name"},
	{SourceKey: "phone", BuiltinField: "phone"},
	{SourceKey: "email", BuiltinField: "email"},
	{SourceKey: "address1", BuiltinField: "address"},
	{SourceKey: "city", BuiltinField: "city"},
	{SourceKey: "state", BuiltinField: "state"},
	{SourceKey: "zip", BuiltinField: "zip"},
	{SourceKey: "uuid", BuiltinField: "external_id"},
	{SourceKey: "id", BuiltinField: "external_id"},
	{SourceKey: "lead_source", BuiltinField: "source"},
}

// ProvisionSunbaseWebhooks creates inbound + outbound POST/GET webhooks for a SunBase connection.
func (s *Service) ProvisionSunbaseWebhooks(
	ctx context.Context,
	accountID int64,
	connectionID int64,
	connectionPublicID string,
	connectionName string,
	schemaName string,
	endpointURL string,
	outboundFieldMap json.RawMessage,
) (*SunbaseWebhookIDs, error) {
	if endpointURL == "" {
		endpointURL = sunbaseDefaultEndpoint
	}
	if len(outboundFieldMap) == 0 || string(outboundFieldMap) == "null" {
		outboundFieldMap = defaultSunbaseOutboundFieldMapJSON(schemaName)
	}

	var accountName string
	if err := s.pool.QueryRow(ctx, `SELECT name FROM accounts WHERE id=$1`, accountID).Scan(&accountName); err != nil {
		return nil, err
	}

	baseSlug, err := sunbaseBaseSlug(accountName, connectionPublicID)
	if err != nil {
		return nil, err
	}

	inboundSlug, err := s.uniqueSunbaseSlug(ctx, baseSlug)
	if err != nil {
		return nil, err
	}
	postSlug, err := s.uniqueSunbaseSlug(ctx, baseSlug+"-post")
	if err != nil {
		return nil, err
	}
	getSlug, err := s.uniqueSunbaseSlug(ctx, baseSlug+"-get")
	if err != nil {
		return nil, err
	}

	falseVal := false
	trueVal := true
	noSign := false
	inboundSecretRequired := false

	inbound, _, err := s.Create(ctx, accountID, CreateWebhookInput{
		Name:                  fmt.Sprintf("SunBase Inbound — %s", connectionName),
		Slug:                  inboundSlug,
		InboundEnabled:        &trueVal,
		InboundSecretRequired: &inboundSecretRequired,
		OutboundEnabled:       &falseVal,
	})
	if err != nil {
		return nil, err
	}

	dupUpdate := "update"
	event, err := s.CreateEvent(ctx, inbound.ID, CreateEventParams{
		Action:        "create",
		DuplicateMode: &dupUpdate,
		Conditions:    json.RawMessage(`[{"field":"action","op":"eq","value":"Create"}]`),
	})
	if err != nil {
		_ = s.Delete(ctx, accountID, inbound.ID)
		return nil, err
	}
	for _, f := range defaultSunbaseInboundFields {
		bf := f.BuiltinField
		if _, err := s.AddFieldMap(ctx, event.ID, f.SourceKey, "builtin", &bf, nil); err != nil {
			_ = s.Delete(ctx, accountID, inbound.ID)
			return nil, err
		}
	}

	outboundURL := endpointURL
	format := "url"
	methodPost := "POST"
	methodGet := "GET"
	emptyTemplate := ""

	postWH, _, err := s.Create(ctx, accountID, CreateWebhookInput{
		Name:                fmt.Sprintf("SunBase Outbound POST — %s", connectionName),
		Slug:                postSlug,
		InboundEnabled:      &falseVal,
		OutboundEnabled:     &trueVal,
		OutboundSignEnabled: &noSign,
		OutboundURL:         &outboundURL,
	})
	if err != nil {
		s.deleteSunbaseWebhooks(ctx, accountID, inbound.ID, 0, 0)
		return nil, err
	}
	responseMap := defaultSunbaseResponseMapJSON()
	if _, err := s.Update(ctx, accountID, postWH.ID, UpdateWebhookInput{
		OutboundFormat:          &format,
		OutboundMethod:          &methodPost,
		OutboundFieldMap:        outboundFieldMap,
		OutboundResponseMap:     responseMap,
		OutboundPayloadTemplate: &emptyTemplate,
	}); err != nil {
		s.deleteSunbaseWebhooks(ctx, accountID, inbound.ID, postWH.ID, 0)
		return nil, err
	}

	getWH, _, err := s.Create(ctx, accountID, CreateWebhookInput{
		Name:                fmt.Sprintf("SunBase Outbound GET — %s", connectionName),
		Slug:                getSlug,
		InboundEnabled:      &falseVal,
		OutboundEnabled:     &trueVal,
		OutboundSignEnabled: &noSign,
		OutboundURL:         &outboundURL,
	})
	if err != nil {
		s.deleteSunbaseWebhooks(ctx, accountID, inbound.ID, postWH.ID, 0)
		return nil, err
	}
	if _, err := s.Update(ctx, accountID, getWH.ID, UpdateWebhookInput{
		OutboundFormat: &format,
		OutboundMethod: &methodGet,
		OutboundFieldMap: outboundFieldMap,
		OutboundPayloadTemplate: &emptyTemplate,
	}); err != nil {
		s.deleteSunbaseWebhooks(ctx, accountID, inbound.ID, postWH.ID, getWH.ID)
		return nil, err
	}

	ids := &SunbaseWebhookIDs{
		OutboundPost: postWH.ID,
		OutboundGet:  getWH.ID,
		Inbound:      inbound.ID,
		InboundSlug:  inboundSlug,
	}
	return ids, nil
}

func (s *Service) SyncSunbaseOutboundWebhooks(ctx context.Context, accountID int64, ids SunbaseWebhookIDs, endpointURL string, outboundFieldMap json.RawMessage) error {
	if endpointURL == "" {
		endpointURL = sunbaseDefaultEndpoint
	}
	format := "url"
	methodPost := "POST"
	methodGet := "GET"
	emptyTemplate := ""

	if ids.OutboundPost > 0 {
		if _, err := s.Update(ctx, accountID, ids.OutboundPost, UpdateWebhookInput{
			OutboundURL:             &endpointURL,
			OutboundFormat:          &format,
			OutboundMethod:          &methodPost,
			OutboundFieldMap:        outboundFieldMap,
			OutboundPayloadTemplate: &emptyTemplate,
		}); err != nil {
			return err
		}
	}
	if ids.OutboundGet > 0 {
		if _, err := s.Update(ctx, accountID, ids.OutboundGet, UpdateWebhookInput{
			OutboundURL:             &endpointURL,
			OutboundFormat:          &format,
			OutboundMethod:          &methodGet,
			OutboundFieldMap:        outboundFieldMap,
			OutboundPayloadTemplate: &emptyTemplate,
		}); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) DeleteSunbaseWebhooks(ctx context.Context, accountID int64, ids SunbaseWebhookIDs) {
	s.deleteSunbaseWebhooks(ctx, accountID, ids.Inbound, ids.OutboundPost, ids.OutboundGet)
}

func (s *Service) deleteSunbaseWebhooks(ctx context.Context, accountID int64, inboundID, postID, getID int64) {
	for _, id := range []int64{inboundID, postID, getID} {
		if id > 0 {
			_ = s.Delete(ctx, accountID, id)
		}
	}
}

func (s *Service) uniqueSunbaseSlug(ctx context.Context, base string) (string, error) {
	for attempt := 0; attempt < 5; attempt++ {
		slug := base
		if attempt > 0 {
			suffix, err := randomSlugSuffix(4)
			if err != nil {
				return "", err
			}
			slug = base + "-" + suffix
		}
		var exists bool
		if err := s.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM webhooks WHERE slug=$1)`, slug).Scan(&exists); err != nil {
			return "", err
		}
		if !exists {
			return slug, nil
		}
	}
	return "", httpx.Conflict("could not generate unique webhook slug")
}

func sunbaseBaseSlug(accountName, connectionPublicID string) (string, error) {
	business := slugifySunbaseName(accountName)
	suffix := strings.ReplaceAll(connectionPublicID, "-", "")
	if len(suffix) > 6 {
		suffix = suffix[:6]
	}
	if suffix == "" {
		var err error
		suffix, err = randomSlugSuffix(6)
		if err != nil {
			return "", err
		}
	}
	return business + "-leadrula-" + strings.ToLower(suffix), nil
}

func slugifySunbaseName(name string) string {
	name = strings.ToLower(strings.TrimSpace(name))
	if name == "" {
		return "account"
	}
	re := regexp.MustCompile(`[^a-z0-9]+`)
	name = re.ReplaceAllString(name, "-")
	name = strings.Trim(name, "-")
	if len(name) > 32 {
		name = strings.Trim(name[:32], "-")
	}
	if name == "" {
		return "account"
	}
	return name
}

func randomSlugSuffix(n int) (string, error) {
	b := make([]byte, (n+1)/2)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b)[:n], nil
}

func defaultSunbaseResponseMapJSON() json.RawMessage {
	b, _ := json.Marshal([]map[string]string{
		{"response_key": "uuid", "target_type": "builtin", "builtin_field": "external_id"},
		{"response_key": "id", "target_type": "builtin", "builtin_field": "external_id"},
	})
	return b
}

func defaultSunbaseOutboundFieldMapJSON(schemaName string) json.RawMessage {
	entries := []OutboundFieldMapEntry{
		{DestKey: "schema_name", SourceType: "static", StaticValue: &schemaName},
		{DestKey: "last_name", SourceType: "builtin", BuiltinField: strPtr("last_name")},
		{DestKey: "first_name", SourceType: "builtin", BuiltinField: strPtr("first_name")},
		{DestKey: "address1", SourceType: "builtin", BuiltinField: strPtr("address")},
		{DestKey: "city", SourceType: "builtin", BuiltinField: strPtr("city")},
		{DestKey: "state", SourceType: "builtin", BuiltinField: strPtr("state")},
		{DestKey: "zip_code", SourceType: "builtin", BuiltinField: strPtr("zip")},
		{DestKey: "email", SourceType: "builtin", BuiltinField: strPtr("email")},
		{DestKey: "phone", SourceType: "builtin", BuiltinField: strPtr("phone")},
		{DestKey: "lead_source", SourceType: "builtin", BuiltinField: strPtr("source")},
		{DestKey: "lead_other", SourceType: "builtin", BuiltinField: strPtr("external_id")},
	}
	b, _ := json.Marshal(entries)
	return b
}

func strPtr(s string) *string { return &s }

func ParseSunbaseWebhookIDs(config any) SunbaseWebhookIDs {
	var out SunbaseWebhookIDs
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
	wh, ok := m["webhook_ids"].(map[string]any)
	if !ok {
		return out
	}
	out.OutboundPost = int64FromAny(wh["outbound_post"])
	out.OutboundGet = int64FromAny(wh["outbound_get"])
	out.Inbound = int64FromAny(wh["inbound"])
	return out
}

func int64FromAny(v any) int64 {
	switch t := v.(type) {
	case float64:
		return int64(t)
	case int64:
		return t
	case int:
		return int64(t)
	default:
		return 0
	}
}

func MergeSunbaseConfig(config map[string]any, ids *SunbaseWebhookIDs, endpointURL string, outboundFieldMap json.RawMessage) map[string]any {
	if config == nil {
		config = map[string]any{}
	}
	config["endpoint_url"] = endpointURL
	if len(outboundFieldMap) > 0 {
		var fm any
		_ = json.Unmarshal(outboundFieldMap, &fm)
		config["outbound_field_map"] = fm
	}
	var responseMap any
	_ = json.Unmarshal(defaultSunbaseResponseMapJSON(), &responseMap)
	config["outbound_response_map"] = responseMap
	if ids != nil {
		config["webhook_ids"] = map[string]any{
			"outbound_post": ids.OutboundPost,
			"outbound_get":  ids.OutboundGet,
			"inbound":       ids.Inbound,
		}
		config["inbound_webhook_slug"] = ids.InboundSlug
	}
	return config
}
