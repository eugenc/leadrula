package notifications

import (
	"context"
	"encoding/json"

	"github.com/echayko/leadrula/backend/internal/auth"
	"github.com/echayko/leadrula/backend/internal/permissions"
	"github.com/echayko/leadrula/backend/pkg/httpx"
)

type SettingsResponse struct {
	Account  PrefsMap `json:"account,omitempty"`
	Personal PrefsMap `json:"personal"`
}

type SettingsPatch struct {
	Account  PrefsMap `json:"account,omitempty"`
	Personal PrefsMap `json:"personal,omitempty"`
}

func (s *Service) GetSettings(ctx context.Context, p *auth.Principal) (*SettingsResponse, error) {
	acct, err := s.accounts.GetAccount(ctx, p.AccountID)
	if err != nil {
		return nil, err
	}
	u, err := s.accounts.GetUser(ctx, p.UserID)
	if err != nil {
		return nil, err
	}

	resp := &SettingsResponse{
		Personal: fillDefaults(userNotifPrefsFromRaw(u.Prefs), personalEvents()),
	}
	if p.CanAction(permissions.ActionSettingsAdmin) && acct.Type != "platform" {
		raw, err := s.accounts.GetNotificationPrefs(ctx, s.pool, p.AccountID)
		if err != nil {
			return nil, err
		}
		resp.Account = fillDefaults(prefsFromRaw(raw), accountEvents(acct.Type))
	}
	return resp, nil
}

func (s *Service) PatchSettings(ctx context.Context, p *auth.Principal, patch SettingsPatch) (*SettingsResponse, error) {
	acct, err := s.accounts.GetAccount(ctx, p.AccountID)
	if err != nil {
		return nil, err
	}

	if patch.Account != nil {
		if !p.CanAction(permissions.ActionSettingsAdmin) {
			return nil, httpx.Forbidden("admin role required")
		}
		if acct.Type == "platform" {
			return nil, httpx.Validation("platform accounts do not have notification settings")
		}
		raw, err := s.accounts.GetNotificationPrefs(ctx, s.pool, p.AccountID)
		if err != nil {
			return nil, err
		}
		merged := mergePrefs(prefsFromRaw(raw), patch.Account)
		out, err := json.Marshal(merged)
		if err != nil {
			return nil, err
		}
		if err := s.accounts.UpdateNotificationPrefs(ctx, p.AccountID, out); err != nil {
			return nil, err
		}
	}

	if patch.Personal != nil {
		u, err := s.accounts.GetUser(ctx, p.UserID)
		if err != nil {
			return nil, err
		}
		root := map[string]any{}
		if len(u.Prefs) > 0 {
			_ = json.Unmarshal(u.Prefs, &root)
		}
		existing := userNotifPrefsFromRaw(u.Prefs)
		root[userPrefsKey] = mergePrefs(existing, patch.Personal)
		merged, err := json.Marshal(root)
		if err != nil {
			return nil, err
		}
		if err := s.accounts.UpdatePrefs(ctx, p.UserID, merged); err != nil {
			return nil, err
		}
	}

	return s.GetSettings(ctx, p)
}
