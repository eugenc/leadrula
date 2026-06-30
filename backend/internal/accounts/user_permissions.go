package accounts

import (
	"context"
	"encoding/json"

	"github.com/echayko/leadrula/backend/internal/permissions"
	"github.com/echayko/leadrula/backend/pkg/httpx"
)

func permissionsMap(raw []byte) map[string]any {
	if len(raw) == 0 {
		return map[string]any{}
	}
	var m map[string]any
	if json.Unmarshal(raw, &m) != nil {
		return map[string]any{}
	}
	if m == nil {
		return map[string]any{}
	}
	return m
}

func userListFields(u User, accountType string) (map[string]any, map[string]any) {
	return permissionsMap(u.Permissions), permissions.ToMap(permissions.Resolve(u.Role, accountType, u.Permissions))
}

func inviteListFields(inv Invite, accountType string) (map[string]any, map[string]any) {
	return permissionsMap(inv.Permissions), permissions.ToMap(permissions.Resolve(inv.Role, accountType, inv.Permissions))
}

func parsePermissionsInput(raw json.RawMessage, accountType string) ([]byte, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	var o permissions.Overrides
	if err := json.Unmarshal(raw, &o); err != nil {
		return nil, httpx.Validation("invalid permissions")
	}
	if err := permissions.ValidateOverrides(accountType, o); err != nil {
		return nil, httpx.Validation(err.Error())
	}
	return permissions.MarshalOverrides(o)
}

func (s *Service) countFullAccessAdmins(ctx context.Context, accountID int64, skipUserID int64) (int, error) {
	acct, err := s.repo.GetAccount(ctx, accountID)
	if err != nil {
		return 0, err
	}
	users, err := s.repo.ListUsers(ctx, accountID)
	if err != nil {
		return 0, err
	}
	count := 0
	for _, u := range users {
		if !u.IsActive || u.ID == skipUserID {
			continue
		}
		if permissions.Resolve(u.Role, acct.Type, u.Permissions).IsFullAdmin() {
			count++
		}
	}
	invites, err := s.repo.ListPendingInvites(ctx, accountID)
	if err != nil {
		return 0, err
	}
	for _, inv := range invites {
		if permissions.Resolve(inv.Role, acct.Type, inv.Permissions).IsFullAdmin() {
			count++
		}
	}
	return count, nil
}

func (s *Service) ensureFullAdminRemains(ctx context.Context, accountID int64, targetUserID int64, role string, permRaw []byte) error {
	acct, err := s.repo.GetAccount(ctx, accountID)
	if err != nil {
		return err
	}
	count := 0
	users, err := s.repo.ListUsers(ctx, accountID)
	if err != nil {
		return err
	}
	for _, u := range users {
		if !u.IsActive {
			continue
		}
		r := u.Role
		p := u.Permissions
		if u.ID == targetUserID {
			r = role
			p = permRaw
		}
		if permissions.Resolve(r, acct.Type, p).IsFullAdmin() {
			count++
		}
	}
	invites, err := s.repo.ListPendingInvites(ctx, accountID)
	if err != nil {
		return err
	}
	for _, inv := range invites {
		if permissions.Resolve(inv.Role, acct.Type, inv.Permissions).IsFullAdmin() {
			count++
		}
	}
	if count == 0 {
		return httpx.BusinessRule("cannot remove the last admin")
	}
	return nil
}

func (s *Service) ensureFullAdminRemainsWithInvite(ctx context.Context, accountID, inviteID int64, role string, permRaw []byte) error {
	acct, err := s.repo.GetAccount(ctx, accountID)
	if err != nil {
		return err
	}
	count := 0
	users, err := s.repo.ListUsers(ctx, accountID)
	if err != nil {
		return err
	}
	for _, u := range users {
		if !u.IsActive {
			continue
		}
		if permissions.Resolve(u.Role, acct.Type, u.Permissions).IsFullAdmin() {
			count++
		}
	}
	invites, err := s.repo.ListPendingInvites(ctx, accountID)
	if err != nil {
		return err
	}
	for _, inv := range invites {
		r := inv.Role
		p := inv.Permissions
		if inv.ID == inviteID {
			r = role
			p = permRaw
		}
		if permissions.Resolve(r, acct.Type, p).IsFullAdmin() {
			count++
		}
	}
	if count == 0 {
		return httpx.BusinessRule("cannot remove the last admin")
	}
	return nil
}
