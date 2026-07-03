package collaboration

import (
	"context"
	"errors"

	"github.com/echayko/leadrula/backend/internal/accounts"
	"github.com/echayko/leadrula/backend/internal/auth"
	"github.com/echayko/leadrula/backend/internal/permissions"
)

// ResolvePrincipal loads the real user and applies switch or impersonation claims when present.
func (s *Service) ResolvePrincipal(ctx context.Context, claims *auth.Claims) (*auth.Principal, error) {
	real, err := s.loadUserPrincipal(ctx, claims.Subject)
	if err != nil {
		return nil, err
	}

	if claims.SwitchedFrom != "" {
		return s.resolveSwitch(ctx, real, claims)
	}
	if claims.Impersonating {
		return s.resolveImpersonation(ctx, real, claims)
	}
	return real, nil
}

func (s *Service) resolveSwitch(ctx context.Context, real *auth.Principal, claims *auth.Claims) (*auth.Principal, error) {
	var originID int64
	var originType string
	err := s.repo.pool.QueryRow(ctx,
		`SELECT id, type FROM accounts WHERE public_id = $1`, claims.SwitchedFrom).
		Scan(&originID, &originType)
	if err != nil {
		return nil, ErrNotFound
	}

	targetID, targetType, err := s.repo.GetAccountByPublicID(ctx, claims.AccountID)
	if err != nil {
		return nil, ErrNotFound
	}
	var targetOpStatus string
	if err := s.repo.pool.QueryRow(ctx,
		`SELECT operational_status FROM accounts WHERE id = $1`, targetID).Scan(&targetOpStatus); err != nil {
		return nil, ErrNotFound
	}
	if targetOpStatus == "suspended" {
		return nil, ErrNotFound
	}

	switch originType {
	case "platform":
		if targetType != "publisher" && targetType != "buyer" {
			return nil, ErrNotFound
		}
	case "publisher":
		if targetType != "buyer" {
			return nil, ErrNotFound
		}
		var ok bool
		_ = s.repo.pool.QueryRow(ctx,
			`SELECT EXISTS(
				SELECT 1 FROM partnerships
				WHERE publisher_id = $1 AND buyer_id = $2 AND status = 'active'
			)`, originID, targetID).Scan(&ok)
		if !ok {
			return nil, ErrNotFound
		}
	case "buyer":
		if targetType != "buyer" {
			return nil, ErrNotFound
		}
		var ok bool
		_ = s.repo.pool.QueryRow(ctx,
			`SELECT EXISTS(
				SELECT 1 FROM users u
				JOIN users u2 ON u2.email = u.email
				WHERE u.account_id = $1 AND u2.account_id = $2
				  AND u.role = 'admin' AND u2.role = 'admin'
				  AND u.is_active AND u2.is_active
			)`, originID, targetID).Scan(&ok)
		if !ok {
			return nil, ErrNotFound
		}
	default:
		return nil, ErrNotFound
	}

	principal := &auth.Principal{
		UserID:          real.UserID,
		UserPublicID:    real.UserPublicID,
		AccountID:       targetID,
		AccountPublicID: claims.AccountID,
		AccountType:     targetType,
		Role:            "admin",
		SwitchedFrom:    claims.SwitchedFrom,
		FullAccess:      true,
		Perms:           permissions.FullAccess(targetType),
	}
	if originType == "publisher" && targetType == "buyer" {
		principal.SwitchedFromPublisherID = originID
	}
	return principal, nil
}

func (s *Service) resolveImpersonation(ctx context.Context, real *auth.Principal, claims *auth.Claims) (*auth.Principal, error) {
	if claims.ImpersonatorAcct == "" {
		return nil, ErrNotFound
	}
	buyerAccountID, buyerType, err := s.repo.GetAccountByPublicID(ctx, claims.AccountID)
	if err != nil || buyerType != "buyer" {
		return nil, ErrNotFound
	}
	pubID, pubType, err := s.repo.GetAccountByPublicID(ctx, claims.ImpersonatorAcct)
	if err != nil || pubType != "publisher" {
		return nil, ErrNotFound
	}
	if !canImpersonateFromPublisher(real, pubID) {
		return nil, ErrNotFound
	}
	if err := s.repo.ValidateActive(ctx, pubID, buyerAccountID, claims.CollabVersion); err != nil {
		return nil, err
	}
	impersonator := auth.Principal{
		UserID:          real.UserID,
		UserPublicID:    real.UserPublicID,
		AccountID:       pubID,
		AccountPublicID: claims.ImpersonatorAcct,
		AccountType:     "publisher",
		Role:            "admin",
		FullAccess:      true,
		Perms:           permissions.FullAccess("publisher"),
	}
	return &auth.Principal{
		UserID:          real.UserID,
		UserPublicID:    real.UserPublicID,
		AccountID:       buyerAccountID,
		AccountPublicID: claims.AccountID,
		AccountType:     "buyer",
		Role:            "admin",
		Impersonator:    &impersonator,
		FullAccess:      true,
		Perms:           permissions.FullAccess("buyer"),
	}, nil
}

func canImpersonateFromPublisher(real *auth.Principal, pubID int64) bool {
	if real == nil || !real.IsAdmin() {
		return false
	}
	if real.AccountType == "platform" {
		return true
	}
	return real.AccountType == "publisher" && real.AccountID == pubID
}

func (s *Service) loadUserPrincipal(ctx context.Context, userPublicID string) (*auth.Principal, error) {
	p, err := accounts.NewRepository(s.repo.pool).LoadPrincipal(ctx, userPublicID)
	if err != nil {
		if errors.Is(err, accounts.ErrNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return p, nil
}
