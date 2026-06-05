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
	SwitchedFrom    string // origin account public_id when in a switch session
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

// WebhookAuth is resolved from a valid per-webhook secret.
type WebhookAuth struct {
	WebhookID int64
	AccountID int64
}

const webhookAuthKey ctxKey = 3

func WithWebhookAuth(ctx context.Context, a *WebhookAuth) context.Context {
	return context.WithValue(ctx, webhookAuthKey, a)
}

func WebhookAuthFromContext(ctx context.Context) *WebhookAuth {
	a, _ := ctx.Value(webhookAuthKey).(*WebhookAuth)
	return a
}

// ImpersonationChange is a before/after field diff recorded for collaboration audit.
type ImpersonationChange struct {
	Field string
	From  string
	To    string
}

const impersonationChangesKey ctxKey = 2

// SetImpersonationChanges stores field diffs on the request context for post-handler audit logging.
func SetImpersonationChanges(ctx context.Context, changes []ImpersonationChange) context.Context {
	if len(changes) == 0 {
		return ctx
	}
	return context.WithValue(ctx, impersonationChangesKey, changes)
}

// ImpersonationChangesFromContext returns field diffs set during request handling.
func ImpersonationChangesFromContext(ctx context.Context) []ImpersonationChange {
	changes, _ := ctx.Value(impersonationChangesKey).([]ImpersonationChange)
	return changes
}
