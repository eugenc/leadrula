package accounts

import (
	"context"

	"github.com/echayko/leadrula/backend/internal/auth"
	"github.com/echayko/leadrula/backend/pkg/httpx"
	"github.com/jackc/pgx/v5"
)

type switchOrigin struct {
	AccountID       int64
	AccountPublicID string
	AccountType     string
	Role            string
}

func (s *Service) switchOrigin(ctx context.Context, actor *auth.Principal) (*switchOrigin, error) {
	if actor.SwitchedFrom != "" {
		var o switchOrigin
		err := s.repo.pool.QueryRow(ctx,
			`SELECT a.id, a.public_id, a.type FROM accounts a WHERE a.public_id = $1`,
			actor.SwitchedFrom).Scan(&o.AccountID, &o.AccountPublicID, &o.AccountType)
		if err != nil {
			if err == pgx.ErrNoRows {
				return nil, httpx.NotFound("origin account not found")
			}
			return nil, err
		}
		o.Role = "admin"
		return &o, nil
	}
	return &switchOrigin{
		AccountID:       actor.AccountID,
		AccountPublicID: actor.AccountPublicID,
		AccountType:     actor.AccountType,
		Role:            actor.Role,
	}, nil
}

func (s *Service) canSwitchTo(ctx context.Context, origin *switchOrigin, targetID int64, targetType string) error {
	switch origin.AccountType {
	case "platform":
		if !origin.RoleIsAdmin() {
			return httpx.Forbidden("only admins can switch accounts")
		}
		if targetType != "publisher" && targetType != "buyer" {
			return httpx.Forbidden("platform can only switch to publisher or buyer accounts")
		}
	case "publisher":
		if origin.Role != "admin" {
			return httpx.Forbidden("only admins can switch accounts")
		}
		if targetType != "buyer" {
			return httpx.Forbidden("publishers can only switch to buyer accounts")
		}
		var ok bool
		_ = s.repo.pool.QueryRow(ctx,
			`SELECT EXISTS(
				SELECT 1 FROM partnerships
				WHERE publisher_id = $1 AND buyer_id = $2 AND status = 'active'
			)`, origin.AccountID, targetID).Scan(&ok)
		if !ok {
			return httpx.Forbidden("no active partnership with this buyer")
		}
	case "buyer":
		if origin.Role != "admin" {
			return httpx.Forbidden("only admins can switch accounts")
		}
		if targetType != "buyer" {
			return httpx.Forbidden("buyers can only switch to other buyer accounts")
		}
		if targetID == origin.AccountID {
			return httpx.Forbidden("already on this account")
		}
		var ok bool
		_ = s.repo.pool.QueryRow(ctx,
			`SELECT EXISTS(
				SELECT 1 FROM users u
				JOIN users u2 ON u2.email = u.email
				WHERE u.account_id = $1 AND u2.account_id = $2
				  AND u.role = 'admin' AND u2.role = 'admin'
				  AND u.is_active AND u2.is_active
			)`, origin.AccountID, targetID).Scan(&ok)
		if !ok {
			return httpx.Forbidden("no access to this buyer account")
		}
	default:
		return httpx.Forbidden("account switching not available for this account type")
	}
	return nil
}

func (o *switchOrigin) RoleIsAdmin() bool { return o.Role == "admin" }

// SwitchAccount issues a scoped access token for the target account.
func (s *Service) SwitchAccount(ctx context.Context, actor *auth.Principal, targetAccountPublicID string) (*LoginResult, error) {
	origin, err := s.switchOrigin(ctx, actor)
	if err != nil {
		return nil, err
	}

	targetID, targetType, targetName, targetOpStatus, err := s.repo.GetAccountByPublicID(ctx, targetAccountPublicID)
	if err != nil {
		if err == ErrNotFound {
			return nil, httpx.NotFound("account not found")
		}
		return nil, err
	}
	if targetOpStatus == AccountStatusSuspended {
		return nil, httpx.Forbidden("account is suspended")
	}
	if err := s.canSwitchTo(ctx, origin, targetID, targetType); err != nil {
		return nil, err
	}

	_ = s.repo.LogAccountSwitch(ctx, actor.UserID, origin.AccountID, targetID)

	access, err := s.tokens.IssueAccess(auth.Identity{
		UserPublicID:    actor.UserPublicID,
		AccountPublicID: targetAccountPublicID,
		AccountType:     targetType,
		Role:            "admin",
		SwitchedFrom:    origin.AccountPublicID,
	})
	if err != nil {
		return nil, err
	}

	return &LoginResult{
		Access: access,
		User: map[string]any{
			"id":            actor.UserPublicID,
			"account_id":    targetAccountPublicID,
			"account_type":  targetType,
			"account_name":  targetName,
			"switched_from": origin.AccountPublicID,
		},
	}, nil
}

// SwitchBack issues a token back to the user's origin account.
func (s *Service) SwitchBack(ctx context.Context, actor *auth.Principal) (*LoginResult, error) {
	if actor.SwitchedFrom == "" {
		return nil, httpx.BusinessRule("not in a switched session")
	}
	_, originType, originName, _, err := s.repo.GetAccountByPublicID(ctx, actor.SwitchedFrom)
	if err != nil {
		if err == ErrNotFound {
			return nil, httpx.NotFound("origin account not found")
		}
		return nil, err
	}

	real, err := s.repo.LoadPrincipal(ctx, actor.UserPublicID)
	if err != nil {
		return nil, err
	}

	access, err := s.tokens.IssueAccess(auth.Identity{
		UserPublicID:    actor.UserPublicID,
		AccountPublicID: actor.SwitchedFrom,
		AccountType:     originType,
		Role:            real.Role,
		SwitchedFrom:    "",
	})
	if err != nil {
		return nil, err
	}

	return &LoginResult{
		Access: access,
		User: map[string]any{
			"id":           actor.UserPublicID,
			"account_id":   actor.SwitchedFrom,
			"account_type": originType,
			"account_name": originName,
		},
	}, nil
}

// ListSwitchable returns accounts the user can switch into from their origin account.
func (s *Service) ListSwitchable(ctx context.Context, actor *auth.Principal) ([]map[string]any, error) {
	origin, err := s.switchOrigin(ctx, actor)
	if err != nil {
		return nil, err
	}

	var rows pgx.Rows
	switch origin.AccountType {
	case "platform":
		if !origin.RoleIsAdmin() {
			return []map[string]any{}, nil
		}
		rows, err = s.repo.pool.Query(ctx,
			`SELECT public_id, handler_id, type, name FROM accounts
			 WHERE type IN ('publisher', 'buyer') AND operational_status = 'active' AND deleted_at IS NULL
			 ORDER BY type, name`)
	case "publisher":
		if origin.Role != "admin" {
			return []map[string]any{}, nil
		}
		rows, err = s.repo.pool.Query(ctx,
			`SELECT a.public_id, a.handler_id, a.type, a.name
			 FROM accounts a
			 JOIN partnerships p ON p.buyer_id = a.id
			 WHERE p.publisher_id = $1 AND p.status = 'active'
			   AND a.operational_status = 'active' AND a.deleted_at IS NULL
			 ORDER BY a.name`, origin.AccountID)
	case "buyer":
		if origin.Role != "admin" {
			return []map[string]any{}, nil
		}
		rows, err = s.repo.pool.Query(ctx,
			`SELECT a.public_id, a.handler_id, a.type, a.name
			 FROM accounts a
			 JOIN users u ON u.account_id = a.id
			 WHERE a.type = 'buyer'
			   AND u.email = (SELECT email FROM users WHERE id = $2)
			   AND u.role = 'admin' AND u.is_active
			   AND a.id <> (SELECT account_id FROM users WHERE id = $2)
			   AND a.operational_status = 'active' AND a.deleted_at IS NULL
			 ORDER BY a.name`, origin.AccountID, actor.UserID)
	default:
		return []map[string]any{}, nil
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []map[string]any
	for rows.Next() {
		var pubID, handlerID, acctType, name string
		if err := rows.Scan(&pubID, &handlerID, &acctType, &name); err != nil {
			return nil, err
		}
		out = append(out, map[string]any{
			"id": pubID, "handler_id": handlerID, "type": acctType, "name": name,
		})
	}
	if out == nil {
		out = []map[string]any{}
	}
	return out, rows.Err()
}
