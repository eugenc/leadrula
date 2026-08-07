package integrations

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"github.com/echayko/leadrula/backend/internal/database"
	"github.com/echayko/leadrula/backend/internal/integrations/providers"
	"github.com/echayko/leadrula/backend/internal/leads"
	"github.com/echayko/leadrula/backend/internal/routing"
	"github.com/echayko/leadrula/backend/pkg/httpx"
)

type VoiceUniIngestResult struct {
	LeadID   string `json:"lead_id"`
	PublicID string `json:"public_id"`
	Status   string `json:"status"`
	Created  bool   `json:"created"`
}

type VoiceUniIngestParams struct {
	ConnectionPublicID string
	ExternalID         string
	Raw                map[string]any
}

func voiceuniFieldMaps(config map[string]any) []routing.SourceFieldMapEntry {
	entries := providers.VoiceUniOutboundFieldMapFromConfig(config)
	var out []routing.SourceFieldMapEntry
	for _, e := range entries {
		if e.DestKey == "" || e.SourceType == "static" {
			continue
		}
		entry := routing.SourceFieldMapEntry{SourceKey: strings.TrimSpace(e.DestKey)}
		switch e.SourceType {
		case "builtin":
			if e.BuiltinField == nil {
				continue
			}
			bf := strings.TrimSpace(*e.BuiltinField)
			entry.TargetType = "builtin"
			entry.BuiltinField = &bf
		case "custom":
			if e.CustomFieldID == nil || *e.CustomFieldID <= 0 {
				continue
			}
			id := *e.CustomFieldID
			entry.TargetType = "custom"
			entry.CustomFieldID = &id
		default:
			continue
		}
		out = append(out, entry)
	}
	return out
}

func voiceuniFlattenPayload(raw map[string]any) map[string]any {
	out := map[string]any{}
	for k, v := range raw {
		if k == "custom" || k == "connection_id" {
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

func voiceuniHasIdentity(flat map[string]any) bool {
	for _, k := range []string{"first_name", "last_name", "phone", "email"} {
		if v, ok := flat[k]; ok && strings.TrimSpace(voiceuniToText(v)) != "" {
			return true
		}
	}
	return false
}

func voiceuniToText(v any) string {
	switch x := v.(type) {
	case string:
		return strings.TrimSpace(x)
	default:
		b, _ := json.Marshal(v)
		return strings.Trim(strings.TrimSpace(string(b)), `"`)
	}
}

func (s *Service) IngestVoiceUni(ctx context.Context, publisherID int64, p VoiceUniIngestParams) (*VoiceUniIngestResult, error) {
	externalID := strings.TrimSpace(p.ExternalID)
	if externalID == "" {
		if v, ok := p.Raw["external_id"]; ok {
			externalID = strings.TrimSpace(voiceuniToText(v))
		}
	}
	if externalID == "" {
		return nil, httpx.Validation("external_id is required")
	}

	conn, err := s.resolveVoiceUniConnection(ctx, publisherID, p.ConnectionPublicID)
	if err != nil {
		return nil, err
	}
	cfg := configMap(conn.Config)
	sourceSlug := providers.VoiceUniSourceSlug(cfg)
	if sourceSlug == "" {
		return nil, httpx.Validation("voiceuni connection is missing source_slug")
	}

	flat := voiceuniFlattenPayload(p.Raw)
	flat["external_id"] = externalID
	if !voiceuniHasIdentity(flat) {
		return nil, httpx.Validation("at least one of first_name, last_name, phone, or email is required")
	}
	if src, ok := flat["source"]; !ok || strings.TrimSpace(voiceuniToText(src)) == "" {
		flat["source"] = "voiceuni"
	}

	maps := voiceuniFieldMaps(cfg)
	rawJSON, _ := json.Marshal(p.Raw)
	authorName := conn.Name
	leadRepo := leads.NewRepository(s.pool)

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	created := false
	var leadID int64
	var publicID string

	existing, err := leadRepo.GetByExternalID(ctx, tx, publisherID, externalID)
	if err != nil {
		var appErr *httpx.AppError
		if !errors.As(err, &appErr) || appErr.Code != httpx.CodeNotFound {
			return nil, err
		}
		leadID, publicID, err = leadRepo.InsertLead(ctx, tx, publisherID, publisherID, sourceSlug, rawJSON)
		if err != nil {
			return nil, err
		}
		created = true
	} else {
		leadID = existing.ID
		publicID = existing.PublicID
	}

	if err := applyVoiceUniMappings(ctx, tx, leadRepo, publisherID, leadID, authorName, flat, maps); err != nil {
		return nil, err
	}
	if err := leadRepo.SetBuiltinField(ctx, tx, leadID, "external_id", externalID); err != nil {
		return nil, err
	}

	status := "updated"
	if created {
		status = "review"
	}

	src, err := routing.MatchSourceBySlug(ctx, tx, publisherID, sourceSlug)
	if err != nil {
		return nil, err
	}
	if src == nil {
		return nil, httpx.Validation("voiceuni source not found")
	}

	if created {
		rt, err := routing.RouteForSource(ctx, tx, src.ID, leadID, flat)
		if err != nil {
			return nil, err
		}
		if rt == nil {
			if _, err := tx.Exec(ctx,
				`INSERT INTO lead_intake_queue(lead_id, raw_payload, source) VALUES ($1,$2,$3)`,
				leadID, rawJSON, sourceSlug); err != nil {
				return nil, err
			}
		} else {
			deps := s.voiceuniRouteDeps(leadRepo)
			emails, err := leads.ApplyRoute(ctx, tx, deps, rt, leadID, leads.RouteExecutionMeta{
				TriggerType:  "integration_ingest",
				TriggerLabel: "voiceuni",
			})
			if err != nil {
				return nil, err
			}
			if rt.Destination == "contract" && rt.Delivery == "leads_pipeline" {
				status = "distributed"
			}
			if err := tx.Commit(ctx); err != nil {
				return nil, err
			}
			if s.leadSvc != nil {
				s.leadSvc.TryApplyConnectionOriginRoute(ctx, publisherID, conn.ID, leadID, flat)
			}
			_ = emails
			return s.finishVoiceUniIngest(ctx, conn.ID, publicID, status, created)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	if s.leadSvc != nil && s.leadSvc.TryApplyConnectionOriginRoute(ctx, publisherID, conn.ID, leadID, flat) {
		lead, err := leadRepo.GetByID(ctx, s.pool, leadID)
		if err == nil && lead != nil {
			switch lead.Status {
			case "distributed", "closed":
				status = "distributed"
			case "review":
				if created {
					status = "review"
				}
			}
		}
	}

	return s.finishVoiceUniIngest(ctx, conn.ID, publicID, status, created)
}

func (s *Service) voiceuniRouteDeps(leadRepo *leads.Repository) leads.RouteApplyDeps {
	return leads.RouteApplyDeps{Repo: leadRepo, Integrations: s}
}

func (s *Service) finishVoiceUniIngest(ctx context.Context, connectionID int64, publicID, status string, created bool) (*VoiceUniIngestResult, error) {
	_, _ = s.pool.Exec(ctx, `UPDATE integration_connections SET last_used_at=now() WHERE id=$1`, connectionID)
	return &VoiceUniIngestResult{
		LeadID:   publicID,
		PublicID: publicID,
		Status:   status,
		Created:  created,
	}, nil
}

func applyVoiceUniMappings(ctx context.Context, tx database.Querier, repo *leads.Repository, accountID, leadID int64, authorName string, flat map[string]any, maps []routing.SourceFieldMapEntry) error {
	for _, k := range []string{"first_name", "last_name", "phone", "email", "address", "city", "state", "zip", "country", "source", "external_id"} {
		if v, ok := flat[k]; ok {
			if str := voiceuniToText(v); str != "" {
				if err := repo.SetBuiltinField(ctx, tx, leadID, k, str); err != nil {
					return err
				}
			}
		}
	}
	for _, m := range maps {
		v, ok := flat[m.SourceKey]
		if !ok {
			continue
		}
		if m.TargetType == "builtin" && m.BuiltinField != nil {
			if *m.BuiltinField == "note" {
				if err := repo.AddInboundNoteFromValue(ctx, tx, leadID, authorName, v); err != nil {
					return err
				}
				continue
			}
			if err := leads.ApplyMappedField(ctx, tx, repo, accountID, leadID, *m.BuiltinField, v); err != nil {
				return err
			}
		} else if m.TargetType == "custom" && m.CustomFieldID != nil {
			valJSON, _ := json.Marshal(v)
			if err := repo.UpsertCustomValue(ctx, tx, leadID, *m.CustomFieldID, valJSON); err != nil {
				return err
			}
		}
	}
	return nil
}
