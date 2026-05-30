package collaboration

import (
	"context"

	"github.com/echayko/leadrula/backend/internal/auth"
)

// ResolvePrincipal loads the real user and applies impersonation claims when present.
func (s *Service) ResolvePrincipal(ctx context.Context, claims *auth.Claims) (*auth.Principal, error) {
	real, err := s.loadUserPrincipal(ctx, claims.Subject)
	if err != nil {
		return nil, err
	}
	if !claims.Impersonating {
		return real, nil
	}
	if real.AccountType != "publisher" || !real.IsAdmin() {
		return nil, ErrNotFound
	}
	buyerAccountID, buyerType, err := s.repo.GetAccountByPublicID(ctx, claims.AccountID)
	if err != nil || buyerType != "buyer" {
		return nil, ErrNotFound
	}
	pubID, err := s.repo.PublisherAccountID(ctx)
	if err != nil {
		return nil, err
	}
	if err := s.repo.ValidateActive(ctx, pubID, buyerAccountID, claims.CollabVersion); err != nil {
		return nil, err
	}
	impersonator := *real
	return &auth.Principal{
		UserID:          real.UserID,
		UserPublicID:    real.UserPublicID,
		AccountID:       buyerAccountID,
		AccountPublicID: claims.AccountID,
		AccountType:     "buyer",
		Role:            "admin",
		Impersonator:    &impersonator,
	}, nil
}

func (s *Service) loadUserPrincipal(ctx context.Context, userPublicID string) (*auth.Principal, error) {
	const q = `
		SELECT u.id, u.public_id, u.account_id, a.public_id, a.type, u.role, u.is_active
		FROM users u JOIN accounts a ON a.id = u.account_id
		WHERE u.public_id = $1`
	p := &auth.Principal{}
	var active bool
	err := s.repo.pool.QueryRow(ctx, q, userPublicID).Scan(
		&p.UserID, &p.UserPublicID, &p.AccountID, &p.AccountPublicID,
		&p.AccountType, &p.Role, &active)
	if err != nil {
		return nil, ErrNotFound
	}
	if !active {
		return nil, ErrNotFound
	}
	return p, nil
}
