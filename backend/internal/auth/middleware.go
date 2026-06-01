package auth

import (
	"context"
	"net/http"
	"strings"

	"github.com/echayko/leadrula/backend/pkg/httpx"
)

// ClaimsLoader resolves JWT claims into a Principal (handles impersonation).
type ClaimsLoader func(ctx context.Context, claims *Claims) (*Principal, error)

// RequireAuth parses the access token and loads the principal into context.
func RequireAuth(tm *TokenManager, load ClaimsLoader) func(http.Handler) http.Handler {
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
			p, err := load(r.Context(), claims)
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

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

// LogImpersonationActions records mutating buyer requests made while impersonating.
func LogImpersonationActions(logFn func(ctx context.Context, p *Principal, method, path string, changes []ImpersonationChange)) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			p := FromContext(r.Context())
			mutating := r.Method != http.MethodGet && r.Method != http.MethodHead
			rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
			next.ServeHTTP(rec, r)
			if p != nil && p.Impersonator != nil && mutating && rec.status < 400 {
				logFn(r.Context(), p, r.Method, r.URL.Path, ImpersonationChangesFromContext(r.Context()))
			}
		})
	}
}

// ApplyImpersonationChanges attaches field diffs to the request for post-handler audit logging.
func ApplyImpersonationChanges(r *http.Request, changes []ImpersonationChange) {
	if len(changes) == 0 {
		return
	}
	*r = *r.WithContext(SetImpersonationChanges(r.Context(), changes))
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
