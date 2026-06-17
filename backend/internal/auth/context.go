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
	Impersonator             *Principal
	SwitchedFrom             string // origin account public_id when in a switch session
	SwitchedFromPublisherID  int64  // set when SwitchedFrom is a publisher account
}

func (p *Principal) IsAdmin() bool    { return p.Role == "admin" }
func (p *Principal) IsFollower() bool { return p.Role == "follower" }

// CollaborationPublisherID returns the impersonating publisher account id when the
// principal is a publisher acting on a buyer account via collaboration.
func (p *Principal) CollaborationPublisherID() (int64, bool) {
	if p == nil || p.Impersonator == nil || p.Impersonator.AccountType != "publisher" {
		return 0, false
	}
	return p.Impersonator.AccountID, true
}

// OversightPublisherID returns the publisher account id when a publisher admin is
// acting on a buyer account via collaboration impersonation or account switch.
func (p *Principal) OversightPublisherID() (int64, bool) {
	if pubID, ok := p.CollaborationPublisherID(); ok {
		return pubID, true
	}
	if p != nil && p.SwitchedFromPublisherID > 0 {
		return p.SwitchedFromPublisherID, true
	}
	return 0, false
}

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
	Scopes      []string
}

// CanReadLeads returns true when the key may call lead read endpoints.
func (a *APIKeyAccount) CanReadLeads() bool {
	if a == nil {
		return false
	}
	if len(a.Scopes) == 0 {
		return true
	}
	for _, s := range a.Scopes {
		if s == "leads:read" || s == "leads:write" {
			return true
		}
	}
	return false
}

// CanWriteLeads returns true when the key may call lead write endpoints.
func (a *APIKeyAccount) CanWriteLeads() bool {
	if a == nil {
		return false
	}
	if len(a.Scopes) == 0 {
		return true
	}
	for _, s := range a.Scopes {
		if s == "leads:write" {
			return true
		}
	}
	return false
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

// SourceAuth is resolved from a source slug (and optional API key).
type SourceAuth struct {
	SourceID    int64
	PublisherID int64
}

const sourceAuthKey ctxKey = 4

func WithSourceAuth(ctx context.Context, a *SourceAuth) context.Context {
	return context.WithValue(ctx, sourceAuthKey, a)
}

func SourceAuthFromContext(ctx context.Context) *SourceAuth {
	a, _ := ctx.Value(sourceAuthKey).(*SourceAuth)
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
