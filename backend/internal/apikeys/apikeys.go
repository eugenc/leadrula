// Package apikeys issues and verifies API keys for the public intake API.
package apikeys

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/echayko/leadrula/backend/internal/auth"
	"github.com/echayko/leadrula/backend/pkg/httpx"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type APIKey struct {
	ID         int64           `json:"id"`
	Name       string          `json:"name"`
	KeyPrefix  string          `json:"key_prefix"`
	Scopes     json.RawMessage `json:"scopes"`
	LastUsedAt *time.Time      `json:"last_used_at"`
	RevokedAt  *time.Time      `json:"revoked_at"`
	CreatedAt  time.Time       `json:"created_at"`
}

type Service struct {
	pool *pgxpool.Pool
}

func NewService(pool *pgxpool.Pool) *Service { return &Service{pool: pool} }

// Create generates a new key, stores its hash, and returns the plaintext once.
func (s *Service) Create(ctx context.Context, accountID int64, name string, scopes []string) (*APIKey, string, error) {
	secret := randString(32)
	prefix := randString(8)
	full := prefix + "." + secret

	hash, err := auth.HashPassword(full)
	if err != nil {
		return nil, "", err
	}
	if scopes == nil {
		scopes = []string{"leads:write"}
	}
	scopesJSON, _ := json.Marshal(scopes)

	k := &APIKey{}
	err = s.pool.QueryRow(ctx,
		`INSERT INTO api_keys(account_id, name, key_prefix, key_hash, scopes)
		 VALUES ($1,$2,$3,$4,$5)
		 RETURNING id, name, key_prefix, scopes, last_used_at, revoked_at, created_at`,
		accountID, name, prefix, hash, scopesJSON).Scan(
		&k.ID, &k.Name, &k.KeyPrefix, &k.Scopes, &k.LastUsedAt, &k.RevokedAt, &k.CreatedAt)
	if err != nil {
		return nil, "", err
	}
	return k, full, nil
}

func (s *Service) List(ctx context.Context, accountID int64) ([]APIKey, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT id, name, key_prefix, scopes, last_used_at, revoked_at, created_at
		 FROM api_keys WHERE account_id = $1 ORDER BY created_at DESC`, accountID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []APIKey
	for rows.Next() {
		var k APIKey
		if err := rows.Scan(&k.ID, &k.Name, &k.KeyPrefix, &k.Scopes, &k.LastUsedAt, &k.RevokedAt, &k.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, k)
	}
	return out, rows.Err()
}

func (s *Service) UpdateName(ctx context.Context, accountID, keyID int64, name string) (*APIKey, error) {
	k := &APIKey{}
	err := s.pool.QueryRow(ctx,
		`UPDATE api_keys SET name = $3
		 WHERE id = $1 AND account_id = $2
		 RETURNING id, name, key_prefix, scopes, last_used_at, revoked_at, created_at`,
		keyID, accountID, name).Scan(
		&k.ID, &k.Name, &k.KeyPrefix, &k.Scopes, &k.LastUsedAt, &k.RevokedAt, &k.CreatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, httpx.NotFound("api key not found")
		}
		return nil, err
	}
	return k, nil
}

func (s *Service) Rotate(ctx context.Context, accountID, keyID int64) (*APIKey, string, error) {
	secret := randString(32)
	prefix := randString(8)
	full := prefix + "." + secret

	hash, err := auth.HashPassword(full)
	if err != nil {
		return nil, "", err
	}

	k := &APIKey{}
	err = s.pool.QueryRow(ctx,
		`UPDATE api_keys SET key_prefix = $3, key_hash = $4
		 WHERE id = $1 AND account_id = $2 AND revoked_at IS NULL
		 RETURNING id, name, key_prefix, scopes, last_used_at, revoked_at, created_at`,
		keyID, accountID, prefix, hash).Scan(
		&k.ID, &k.Name, &k.KeyPrefix, &k.Scopes, &k.LastUsedAt, &k.RevokedAt, &k.CreatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, "", httpx.NotFound("api key not found")
		}
		return nil, "", err
	}
	return k, full, nil
}

func (s *Service) Revoke(ctx context.Context, accountID, keyID int64) error {
	ct, err := s.pool.Exec(ctx,
		`UPDATE api_keys SET revoked_at = now() WHERE id = $1 AND account_id = $2 AND revoked_at IS NULL`,
		keyID, accountID)
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return httpx.NotFound("api key not found")
	}
	return nil
}

// Verify resolves a plaintext key to its account, updating last_used_at.
func (s *Service) Verify(ctx context.Context, full string) (*auth.APIKeyAccount, error) {
	prefix, _, ok := splitKey(full)
	if !ok {
		return nil, errors.New("malformed key")
	}
	rows, err := s.pool.Query(ctx,
		`SELECT k.id, k.key_hash, k.account_id, a.type
		 FROM api_keys k JOIN accounts a ON a.id = k.account_id
		 WHERE k.key_prefix = $1 AND k.revoked_at IS NULL`, prefix)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var id, accountID int64
		var hash, atype string
		if err := rows.Scan(&id, &hash, &accountID, &atype); err != nil {
			return nil, err
		}
		match, _ := auth.VerifyPassword(full, hash)
		if match {
			_, _ = s.pool.Exec(ctx, `UPDATE api_keys SET last_used_at = now() WHERE id = $1`, id)
			return &auth.APIKeyAccount{AccountID: accountID, AccountType: atype}, nil
		}
	}
	return nil, errors.New("invalid key")
}

// RequireAPIKey is middleware that authenticates the public intake API.
func (s *Service) RequireAPIKey(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := auth.Bearer(r)
		if token == "" {
			httpx.Err(w, http.StatusUnauthorized, httpx.CodeUnauthorized, "missing API key")
			return
		}
		acct, err := s.Verify(r.Context(), token)
		if err != nil {
			httpx.Err(w, http.StatusUnauthorized, httpx.CodeUnauthorized, "invalid API key")
			return
		}
		next.ServeHTTP(w, r.WithContext(auth.WithAPIKeyAccount(r.Context(), acct)))
	})
}

// HTTP handlers for key management.

type Handler struct{ svc *Service }

func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

func (h *Handler) RegisterRoutes(r chi.Router) {
	r.Get("/api-keys", h.list)
	r.With(auth.RequireRole("admin")).Post("/api-keys", h.create)
	r.With(auth.RequireRole("admin")).Patch("/api-keys/{id}", h.update)
	r.With(auth.RequireRole("admin")).Post("/api-keys/{id}/rotate", h.rotate)
	r.With(auth.RequireRole("admin")).Delete("/api-keys/{id}", h.revoke)
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	keys, err := h.svc.List(r.Context(), p.AccountID)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, keys)
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	var body struct {
		Name   string   `json:"name"`
		Scopes []string `json:"scopes"`
	}
	if !httpx.DecodeJSON(w, r, &body) {
		return
	}
	name := strings.TrimSpace(body.Name)
	if name == "" {
		httpx.Err(w, http.StatusBadRequest, httpx.CodeValidation, "name is required")
		return
	}
	k, full, err := h.svc.Create(r.Context(), p.AccountID, name, body.Scopes)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, map[string]any{"key": k, "secret": full})
}

func (h *Handler) update(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	id, err := parseKeyID(w, r)
	if err != nil {
		return
	}
	var body struct {
		Name string `json:"name"`
	}
	if !httpx.DecodeJSON(w, r, &body) {
		return
	}
	name := strings.TrimSpace(body.Name)
	if name == "" {
		httpx.Err(w, http.StatusBadRequest, httpx.CodeValidation, "name is required")
		return
	}
	k, err := h.svc.UpdateName(r.Context(), p.AccountID, id, name)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, k)
}

func (h *Handler) rotate(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	id, err := parseKeyID(w, r)
	if err != nil {
		return
	}
	k, full, err := h.svc.Rotate(r.Context(), p.AccountID, id)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"key": k, "secret": full})
}

func (h *Handler) revoke(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	id, err := parseKeyID(w, r)
	if err != nil {
		return
	}
	if err := h.svc.Revoke(r.Context(), p.AccountID, id); err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"ok": true})
}

func parseKeyID(w http.ResponseWriter, r *http.Request) (int64, error) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		httpx.Err(w, http.StatusBadRequest, httpx.CodeValidation, "invalid id")
		return 0, err
	}
	return id, nil
}

func randString(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)[:n]
}

func splitKey(full string) (prefix, secret string, ok bool) {
	for i := 0; i < len(full); i++ {
		if full[i] == '.' {
			return full[:i], full[i+1:], true
		}
	}
	return "", "", false
}
