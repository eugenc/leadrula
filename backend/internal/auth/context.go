package auth

import "context"

// Principal is the authenticated user resolved to internal database IDs.
type Principal struct {
	UserID          int64
	UserPublicID    string
	AccountID       int64
	AccountPublicID string
	AccountType     string // publisher | buyer
	Role            string // admin | user | follower
	Impersonator    *Principal
}

func (p *Principal) IsAdmin() bool    { return p.Role == "admin" }
func (p *Principal) IsFollower() bool { return p.Role == "follower" }

type ctxKey int

const principalKey ctxKey = iota

// WithPrincipal stores the principal in the request context.
func WithPrincipal(ctx context.Context, p *Principal) context.Context {
	return context.WithValue(ctx, principalKey, p)
}

// FromContext retrieves the principal, or nil if unauthenticated.
func FromContext(ctx context.Context) *Principal {
	p, _ := ctx.Value(principalKey).(*Principal)
	return p
}

// APIKeyAccount is the account resolved from a valid API key.
type APIKeyAccount struct {
	AccountID   int64
	AccountType string
}

const apiKeyAccountKey ctxKey = 1

func WithAPIKeyAccount(ctx context.Context, a *APIKeyAccount) context.Context {
	return context.WithValue(ctx, apiKeyAccountKey, a)
}

func APIKeyAccountFromContext(ctx context.Context) *APIKeyAccount {
	a, _ := ctx.Value(apiKeyAccountKey).(*APIKeyAccount)
	return a
}
