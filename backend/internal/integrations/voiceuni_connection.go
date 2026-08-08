package integrations

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/echayko/leadrula/backend/internal/integrations/providers"
	"github.com/echayko/leadrula/backend/internal/routing"
	"github.com/echayko/leadrula/backend/pkg/httpx"
	"github.com/jackc/pgx/v5"
)

type VoiceUniConnectionDetail struct {
	Connection     Connection          `json:"connection"`
	IngestEndpoint string              `json:"ingest_endpoint"`
	ExampleCurl    string              `json:"example_curl,omitempty"`
	SourceSlug     string              `json:"source_slug,omitempty"`
	CallSourceSlug string              `json:"call_source_slug,omitempty"`
}

type VoiceUniConnectionResponse struct {
	Connection
	IngestEndpoint string `json:"ingest_endpoint,omitempty"`
	SourceSlug     string `json:"source_slug,omitempty"`
}

func voiceuniBaseSlug(connectionPublicID string) string {
	s := strings.ToLower(strings.TrimSpace(connectionPublicID))
	if len(s) > 8 {
		s = s[:8]
	}
	return "voiceuni-" + s
}

func (s *Service) provisionVoiceUniSources(ctx context.Context, accountID, connectionID int64, connectionPublicID, connectionName string) (sourceSlug, callSourceSlug string, sourceID int64, err error) {
	routeSvc := routing.NewService(s.pool)
	base := voiceuniBaseSlug(connectionPublicID)
	sourceSlug = base
	callSourceSlug = base + "-calls"

	falseVal := false
	src, err := routeSvc.CreateSource(ctx, accountID, fmt.Sprintf("VoiceUni — %s", connectionName), sourceSlug, "webhook", &falseVal, nil, nil)
	if err != nil {
		var appErr *httpx.AppError
		if errors.As(err, &appErr) && appErr.Code == httpx.CodeConflict {
			existing, lookupErr := routing.MatchSourceBySlug(ctx, s.pool, accountID, sourceSlug)
			if lookupErr != nil || existing == nil {
				return "", "", 0, err
			}
			src = existing
		} else {
			return "", "", 0, err
		}
	}
	return sourceSlug, callSourceSlug, src.ID, nil
}

func (s *Service) syncVoiceUniSourceFieldMaps(ctx context.Context, publisherID, sourceID int64, config map[string]any) error {
	entries := providers.VoiceUniOutboundFieldMapFromConfig(config)
	_, err := s.pool.Exec(ctx, `DELETE FROM routing_source_field_map WHERE source_id=$1`, sourceID)
	if err != nil {
		return err
	}
	routeSvc := routing.NewService(s.pool)
	for _, e := range entries {
		if e.DestKey == "" || e.SourceType == "static" {
			continue
		}
		switch e.SourceType {
		case "builtin":
			if e.BuiltinField == nil {
				continue
			}
			bf := strings.TrimSpace(*e.BuiltinField)
			if _, err := routeSvc.AddSourceFieldMap(ctx, publisherID, sourceID, e.DestKey, "builtin", &bf, nil); err != nil {
				return err
			}
		case "custom":
			if e.CustomFieldID == nil || *e.CustomFieldID <= 0 {
				continue
			}
			id := *e.CustomFieldID
			if _, err := routeSvc.AddSourceFieldMap(ctx, publisherID, sourceID, e.DestKey, "custom", nil, &id); err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *Service) FinalizeVoiceUniConnection(ctx context.Context, connectionID int64, config map[string]any) error {
	configJSON, _ := json.Marshal(config)
	_, err := s.pool.Exec(ctx,
		`UPDATE integration_connections SET config=$2, updated_at=now() WHERE id=$1`,
		connectionID, configJSON)
	return err
}

func (s *Service) UpdateVoiceUniConnection(
	ctx context.Context,
	accountID, id int64,
	config map[string]any,
) (*Connection, error) {
	conn, err := s.GetConnection(ctx, accountID, id)
	if err != nil {
		return nil, err
	}
	if conn.ProviderSlug != "voiceuni" {
		return nil, httpx.Validation("not a voiceuni connection")
	}
	if config == nil {
		config = map[string]any{}
	}
	existingCfg := configMap(conn.Config)
	for k, v := range existingCfg {
		if _, ok := config[k]; !ok {
			config[k] = v
		}
	}
	config = providers.MergeVoiceUniConfigDefaults(config)

	sourceID := configInt64(config["source_id"])
	if sourceID > 0 {
		if err := s.syncVoiceUniSourceFieldMaps(ctx, accountID, sourceID, config); err != nil {
			return nil, err
		}
	}

	configJSON, _ := json.Marshal(config)
	var updated Connection
	err = s.pool.QueryRow(ctx,
		`UPDATE integration_connections SET config=$2, updated_at=now() WHERE id=$1
		 RETURNING id, public_id::text, account_id, name, config, status, created_at`,
		id, configJSON).Scan(
		&updated.ID, &updated.PublicID, &updated.AccountID, &updated.Name,
		&updated.Config, &updated.Status, &updated.CreatedAt)
	if err != nil {
		return nil, err
	}
	updated.ProviderSlug = conn.ProviderSlug
	updated.ProviderName = conn.ProviderName
	return &updated, nil
}

func (s *Service) VoiceUniConnectionDetail(ctx context.Context, accountID, id int64, apiBaseURL string) (*VoiceUniConnectionDetail, error) {
	conn, err := s.GetConnection(ctx, accountID, id)
	if err != nil {
		return nil, err
	}
	if conn.ProviderSlug != "voiceuni" {
		return nil, httpx.Validation("not a voiceuni connection")
	}
	cfg := configMap(conn.Config)
	endpoint := strings.TrimRight(apiBaseURL, "/") + "/api/v1/integrations/voiceuni/ingest"
	detail := &VoiceUniConnectionDetail{
		Connection:     *conn,
		IngestEndpoint: endpoint,
		SourceSlug:     providers.VoiceUniSourceSlug(cfg),
		CallSourceSlug: providers.VoiceUniCallSourceSlug(cfg),
		ExampleCurl: fmt.Sprintf(
			`curl -s -X POST %q -H "Authorization: Bearer YOUR_API_KEY" -H "Content-Type: application/json" -d '{"connection_id":"%s","external_id":"vu-lead-uuid","first_name":"Jane","last_name":"Doe","phone":"+15551234567"}'`,
			endpoint, conn.PublicID,
		),
	}
	return detail, nil
}

func configInt64(v any) int64 {
	switch x := v.(type) {
	case float64:
		return int64(x)
	case int64:
		return x
	case int:
		return int64(x)
	case json.Number:
		n, _ := x.Int64()
		return n
	}
	return 0
}

func (s *Service) resolveVoiceUniConnection(ctx context.Context, accountID int64, connectionPublicID string) (*Connection, error) {
	if strings.TrimSpace(connectionPublicID) != "" {
		var conn Connection
		err := s.pool.QueryRow(ctx,
			`SELECT c.id, c.public_id::text, c.account_id, p.slug, p.name, c.name,
			        c.config, c.status, c.last_error, c.last_used_at, c.created_at
			 FROM integration_connections c
			 JOIN integration_providers p ON p.id = c.provider_id
			 WHERE c.account_id = $1 AND c.public_id::text = $2 AND p.slug = 'voiceuni' AND c.status = 'active'`,
			accountID, strings.TrimSpace(connectionPublicID)).Scan(
			&conn.ID, &conn.PublicID, &conn.AccountID,
			&conn.ProviderSlug, &conn.ProviderName, &conn.Name,
			&conn.Config, &conn.Status, &conn.LastError,
			&conn.LastUsedAt, &conn.CreatedAt)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return nil, httpx.NotFound("voiceuni connection not found")
			}
			return nil, err
		}
		return &conn, nil
	}
	rows, err := s.pool.Query(ctx,
		`SELECT c.id, c.public_id::text, c.account_id, p.slug, p.name, c.name,
		        c.config, c.status, c.last_error, c.last_used_at, c.created_at
		 FROM integration_connections c
		 JOIN integration_providers p ON p.id = c.provider_id
		 WHERE c.account_id = $1 AND p.slug = 'voiceuni' AND c.status = 'active'
		 ORDER BY c.id`, accountID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var conns []Connection
	for rows.Next() {
		var conn Connection
		if err := rows.Scan(
			&conn.ID, &conn.PublicID, &conn.AccountID,
			&conn.ProviderSlug, &conn.ProviderName, &conn.Name,
			&conn.Config, &conn.Status, &conn.LastError,
			&conn.LastUsedAt, &conn.CreatedAt); err != nil {
			return nil, err
		}
		conns = append(conns, conn)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(conns) == 0 {
		return nil, httpx.NotFound("no active voiceuni connection")
	}
	if len(conns) > 1 {
		return nil, httpx.Validation("connection_id is required when multiple voiceuni connections exist")
	}
	return &conns[0], nil
}
