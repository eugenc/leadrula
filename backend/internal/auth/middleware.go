package auth

import (
	"context"
	"net/http"
	"strings"

	"github.com/echayko/leadrula/backend/pkg/httpx"
)

// PrincipalLoader resolves a user public_id to a full Principal with internal
// IDs. The server wires this from the accounts repository to avoid an import
// cycle.
type PrincipalLoader func(ctx context.Context, userPublicID string) (*Principal, error)

// RequireAuth parses the access token and loads the principal into context.
func RequireAuth(tm *TokenManager, load PrincipalLoader) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token := bearer(r)
			if token == "" {
				httpx.Err(w, http.StatusUnauthorized, httpx.CodeUnauthorized, "missing bearer token")
				return
			}
			claims, err := tm.ParseAccess(token)
			if err != nil {
				httpx.Err(w, http.StatusUnauthorized, httpx.CodeUnauthorized, "invalid or expired token")
				return
			}
			p, err := load(r.Context(), claims.Subject)
			if err != nil || p == nil {
				httpx.Err(w, http.StatusUnauthorized, httpx.CodeUnauthorized, "user not found")
				return
			}
			next.ServeHTTP(w, r.WithContext(WithPrincipal(r.Context(), p)))
		})
	}
}

// RequireAccountType rejects principals whose account type does not match.
func RequireAccountType(accountType string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			p := FromContext(r.Context())
			if p == nil || p.AccountType != accountType {
				httpx.Err(w, http.StatusForbidden, httpx.CodeForbidden, "wrong account type")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// RequireRole rejects principals whose role is not in the allowed set.
func RequireRole(roles ...string) func(http.Handler) http.Handler {
	allowed := map[string]bool{}
	for _, role := range roles {
		allowed[role] = true
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			p := FromContext(r.Context())
			if p == nil || !allowed[p.Role] {
				httpx.Err(w, http.StatusForbidden, httpx.CodeForbidden, "insufficient role")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func bearer(r *http.Request) string {
	h := r.Header.Get("Authorization")
	if h == "" {
		return ""
	}
	parts := strings.SplitN(h, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return ""
	}
	return strings.TrimSpace(parts[1])
}

// Bearer extracts the bearer token from a request (exported for API-key mw).
func Bearer(r *http.Request) string { return bearer(r) }
