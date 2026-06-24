package calls

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"strings"
	"time"

	"github.com/echayko/leadrula/backend/internal/database"
	"github.com/echayko/leadrula/backend/internal/routing"
	"github.com/echayko/leadrula/backend/pkg/httpx"
)

const preloadTTL = 30 * time.Minute

type PreloadResult struct {
	PreloadToken string    `json:"preload_token"`
	ExpiresAt    time.Time `json:"expires_at"`
}

// CreatePreload stores caller payload (no lead) keyed by source + optional caller
// phone, returning a token usable on the tracking URL. Auth is the publisher API key.
func (s *Service) CreatePreload(ctx context.Context, publisherID int64, sourceSlug, callerPhone string, payload map[string]any) (*PreloadResult, error) {
	sourceSlug = strings.TrimSpace(sourceSlug)
	if sourceSlug == "" {
		return nil, httpx.Validation("source is required")
	}
	src, err := routing.MatchSourceBySlug(ctx, s.pool, publisherID, sourceSlug)
	if err != nil {
		return nil, err
	}
	if src == nil {
		return nil, httpx.NotFound("source not found")
	}
	if src.Type != "call" {
		return nil, httpx.Validation("source is not a call source")
	}
	token := randomToken()
	var phoneHash string
	if callerPhone != "" {
		phoneHash = hashPhone(callerPhone)
	}
	raw, _ := json.Marshal(payload)
	expires := time.Now().Add(preloadTTL)
	if _, err := s.pool.Exec(ctx,
		`INSERT INTO call_preloads(source_id, caller_phone_hash, preload_token, raw_payload, expires_at)
		 VALUES ($1,$2,$3,$4,$5)`,
		src.ID, nullStr(phoneHash), token, raw, expires); err != nil {
		return nil, err
	}
	return &PreloadResult{PreloadToken: token, ExpiresAt: expires}, nil
}

// matchPreload finds a fresh preload for a source by caller phone first, then by
// token. Returns the stored payload and marks it consumed. nil when no match.
func (s *Service) matchPreload(ctx context.Context, q database.Querier, sourceID int64, callerHash, token string) (map[string]any, error) {
	var id int64
	var raw json.RawMessage
	err := q.QueryRow(ctx,
		`SELECT id, raw_payload FROM call_preloads
		 WHERE source_id=$1 AND consumed_at IS NULL AND expires_at > now()
		   AND ($2 <> '' AND caller_phone_hash = $2)
		 ORDER BY created_at DESC LIMIT 1`,
		sourceID, callerHash).Scan(&id, &raw)
	if err != nil && token != "" {
		err = q.QueryRow(ctx,
			`SELECT id, raw_payload FROM call_preloads
			 WHERE source_id=$1 AND preload_token=$2 AND consumed_at IS NULL AND expires_at > now()
			 LIMIT 1`,
			sourceID, token).Scan(&id, &raw)
	}
	if err != nil {
		return nil, nil
	}
	_, _ = q.Exec(ctx, `UPDATE call_preloads SET consumed_at=now() WHERE id=$1`, id)
	out := map[string]any{}
	_ = json.Unmarshal(raw, &out)
	return out, nil
}

func randomToken() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
