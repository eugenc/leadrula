package leads

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/echayko/leadrula/backend/internal/auth"
	"github.com/echayko/leadrula/backend/pkg/httpx"
)

func (s *Service) ListViews(ctx context.Context, p *auth.Principal, placement string) ([]SavedView, error) {
	if err := s.migrateLegacyViews(ctx, p); err != nil {
		return nil, err
	}
	dbViews, err := s.repo.ListSavedViews(ctx, p.AccountID, p.UserID)
	if err != nil {
		return nil, err
	}
	out := builtinViewsForPlacement(placement)
	for _, v := range dbViews {
		if placement != "" && v.Placement != "both" && v.Placement != placement {
			continue
		}
		out = append(out, v)
	}
	return out, nil
}

func builtinViewsForPlacement(placement string) []SavedView {
	order := []string{"all", "action_today", "overdue"}
	out := make([]SavedView, 0, len(order))
	for _, key := range order {
		v, ok := BuiltinViews[key]
		if !ok {
			continue
		}
		if placement != "" && v.Placement != "both" && v.Placement != placement {
			continue
		}
		out = append(out, v)
	}
	return out
}

func (s *Service) ResolveViewFilters(ctx context.Context, p *auth.Principal, viewID string) ([]FilterCondition, *SavedView, error) {
	if viewID == "" {
		return nil, nil, nil
	}
	if v, ok := BuiltinViews[viewID]; ok {
		cp := v
		return cp.Filters, &cp, nil
	}
	v, err := s.repo.GetSavedViewByPublicID(ctx, p.AccountID, viewID)
	if err != nil {
		return nil, nil, err
	}
	if !canAccessView(p, v) {
		return nil, nil, httpx.NotFound("view not found")
	}
	return v.Filters, v, nil
}

func canAccessView(p *auth.Principal, v *SavedView) bool {
	if v.AccountID != p.AccountID {
		return false
	}
	if v.OwnerUserID == nil {
		return true
	}
	return *v.OwnerUserID == p.UserID
}

func canEditView(p *auth.Principal, v *SavedView) bool {
	if v.IsBuiltin {
		return false
	}
	if v.AccountID != p.AccountID {
		return false
	}
	if v.OwnerUserID == nil {
		return p.Role == "admin"
	}
	return *v.OwnerUserID == p.UserID
}

type CreateViewInput struct {
	Name      string             `json:"name"`
	Placement string             `json:"placement"`
	Shared    bool               `json:"shared"`
	Filters   []FilterCondition  `json:"filters"`
	Columns   []string           `json:"columns"`
	Sort      string             `json:"sort"`
	SortDir   string             `json:"sort_dir"`
}

func (s *Service) CreateView(ctx context.Context, p *auth.Principal, in CreateViewInput) (*SavedView, error) {
	name := strings.TrimSpace(in.Name)
	if name == "" {
		return nil, httpx.Validation("name is required")
	}
	placement := in.Placement
	if placement == "" {
		placement = "list"
	}
	if placement != "list" && placement != "board" && placement != "both" {
		return nil, httpx.Validation("invalid placement")
	}
	if in.Shared && p.Role != "admin" {
		return nil, httpx.Forbidden("only admins can create shared views")
	}
	var ownerID *int64
	if !in.Shared {
		ownerID = &p.UserID
	}
	if in.SortDir != "" && in.SortDir != "asc" && in.SortDir != "desc" {
		return nil, httpx.Validation("sort_dir must be asc or desc")
	}
	return s.repo.CreateSavedView(ctx, CreateSavedViewParams{
		AccountID:   p.AccountID,
		OwnerUserID: ownerID,
		Name:        name,
		Placement:   placement,
		Filters:     in.Filters,
		Columns:     in.Columns,
		Sort:        in.Sort,
		SortDir:     in.SortDir,
		CreatedBy:   p.UserID,
	})
}

type UpdateViewInput struct {
	Name      *string           `json:"name"`
	Placement *string           `json:"placement"`
	Filters   []FilterCondition `json:"filters"`
	Columns   []string          `json:"columns"`
	Sort      *string           `json:"sort"`
	SortDir   *string           `json:"sort_dir"`
}

func (s *Service) UpdateView(ctx context.Context, p *auth.Principal, publicID string, in UpdateViewInput) (*SavedView, error) {
	if _, ok := BuiltinViews[publicID]; ok {
		return nil, httpx.Forbidden("built-in views cannot be edited")
	}
	v, err := s.repo.GetSavedViewByPublicID(ctx, p.AccountID, publicID)
	if err != nil {
		return nil, err
	}
	if !canEditView(p, v) {
		return nil, httpx.Forbidden("cannot edit this view")
	}
	params := UpdateSavedViewParams{SetFilters: in.Filters != nil, SetCols: in.Columns != nil}
	if in.Name != nil {
		n := strings.TrimSpace(*in.Name)
		if n == "" {
			return nil, httpx.Validation("name is required")
		}
		params.Name = &n
	}
	if in.Placement != nil {
		pl := *in.Placement
		if pl != "list" && pl != "board" && pl != "both" {
			return nil, httpx.Validation("invalid placement")
		}
		params.Placement = &pl
	}
	if in.Filters != nil {
		params.Filters = in.Filters
	}
	if in.Columns != nil {
		params.Columns = in.Columns
	}
	if in.SortDir != nil && *in.SortDir != "asc" && *in.SortDir != "desc" {
		return nil, httpx.Validation("sort_dir must be asc or desc")
	}
	params.Sort = in.Sort
	params.SortDir = in.SortDir
	return s.repo.UpdateSavedView(ctx, v.ID, params)
}

func (s *Service) DeleteView(ctx context.Context, p *auth.Principal, publicID string) error {
	if _, ok := BuiltinViews[publicID]; ok {
		return httpx.Forbidden("built-in views cannot be deleted")
	}
	v, err := s.repo.GetSavedViewByPublicID(ctx, p.AccountID, publicID)
	if err != nil {
		return err
	}
	if !canEditView(p, v) {
		return httpx.Forbidden("cannot delete this view")
	}
	return s.repo.DeleteSavedView(ctx, v.ID)
}

func (s *Service) migrateLegacyViews(ctx context.Context, p *auth.Principal) error {
	n, err := s.repo.CountUserSavedViews(ctx, p.UserID)
	if err != nil || n > 0 {
		return err
	}
	u, err := s.accounts.GetUser(ctx, p.UserID)
	if err != nil {
		return err
	}
	if len(u.Prefs) == 0 {
		return nil
	}
	var prefs map[string]any
	if err := json.Unmarshal(u.Prefs, &prefs); err != nil {
		return nil
	}
	raw, ok := prefs["lead_views"].(map[string]any)
	if !ok {
		return nil
	}
	viewsRaw, ok := raw["views"].([]any)
	if !ok {
		return nil
	}
	for _, item := range viewsRaw {
		vm, ok := item.(map[string]any)
		if !ok {
			continue
		}
		if builtin, _ := vm["builtin"].(bool); builtin {
			continue
		}
		params := parseLegacyViewMap(vm)
		params.AccountID = p.AccountID
		params.OwnerUserID = &p.UserID
		params.CreatedBy = p.UserID
		if params.Name == "" {
			continue
		}
		if params.Placement == "" {
			params.Placement = "list"
		}
		if _, err := s.repo.CreateSavedView(ctx, params); err != nil {
			return err
		}
	}
	delete(prefs, "lead_views")
	merged, err := json.Marshal(prefs)
	if err != nil {
		return err
	}
	return s.accounts.UpdatePrefs(ctx, p.UserID, merged)
}

func (s *Service) AccountTimezone(ctx context.Context, accountID int64) string {
	acct, err := s.accounts.GetAccount(ctx, accountID)
	if err != nil || acct.Timezone == "" {
		return "UTC"
	}
	return acct.Timezone
}

func ParseFiltersJSON(raw string) ([]FilterCondition, error) {
	if raw == "" {
		return nil, nil
	}
	var conditions []FilterCondition
	if err := json.Unmarshal([]byte(raw), &conditions); err != nil {
		return nil, httpx.Validation("invalid filters JSON")
	}
	return conditions, nil
}
