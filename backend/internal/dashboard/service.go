package dashboard

import (
	"context"
	"strings"

	"github.com/echayko/leadrula/backend/internal/auth"
	"github.com/echayko/leadrula/backend/internal/permissions"
	"github.com/echayko/leadrula/backend/pkg/httpx"
)

var validPeriods = map[string]bool{
	"today": true, "week": true, "month": true, "all": true,
}

type Service struct {
	repo *Repository
}

func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) ListViews(ctx context.Context, p *auth.Principal) ([]View, error) {
	return s.repo.ListSharedViews(ctx, p.AccountID)
}

type CreateInput struct {
	Name     string   `json:"name"`
	Widgets  []string `json:"widgets"`
	Period   string   `json:"period"`
	Goals    Goals    `json:"goals"`
	Position int      `json:"position"`
}

func (s *Service) CreateView(ctx context.Context, p *auth.Principal, in CreateInput) (*View, error) {
	if !p.CanAction(permissions.ActionSettingsAdmin) {
		return nil, httpx.Forbidden("only admins can create dashboard views")
	}
	name := strings.TrimSpace(in.Name)
	if name == "" {
		return nil, httpx.Validation("name is required")
	}
	period := in.Period
	if period == "" {
		period = "all"
	}
	if !validPeriods[period] {
		return nil, httpx.Validation("invalid period")
	}
	widgets := in.Widgets
	if widgets == nil {
		widgets = []string{}
	}
	return s.repo.Create(ctx, CreateParams{
		AccountID: p.AccountID,
		Name:      name,
		Widgets:   widgets,
		Period:    period,
		Goals:     in.Goals,
		Position:  in.Position,
		CreatedBy: p.UserID,
	})
}

type UpdateInput struct {
	Name     *string  `json:"name"`
	Widgets  []string `json:"widgets"`
	Period   *string  `json:"period"`
	Goals    *Goals   `json:"goals"`
	Position *int     `json:"position"`
}

func (s *Service) UpdateView(ctx context.Context, p *auth.Principal, publicID string, in UpdateInput) (*View, error) {
	v, err := s.repo.GetByPublicID(ctx, p.AccountID, publicID)
	if err != nil {
		return nil, err
	}
	if !canEditView(p, v) {
		return nil, httpx.Forbidden("cannot edit this view")
	}
	params := UpdateParams{SetWidgets: in.Widgets != nil, SetGoals: in.Goals != nil}
	if in.Name != nil {
		n := strings.TrimSpace(*in.Name)
		if n == "" {
			return nil, httpx.Validation("name is required")
		}
		params.Name = &n
	}
	if in.Widgets != nil {
		params.Widgets = in.Widgets
	}
	if in.Period != nil {
		if !validPeriods[*in.Period] {
			return nil, httpx.Validation("invalid period")
		}
		params.Period = in.Period
	}
	if in.Goals != nil {
		params.Goals = in.Goals
	}
	if in.Position != nil {
		params.Position = in.Position
	}
	return s.repo.Update(ctx, v.ID, params)
}

func (s *Service) DeleteView(ctx context.Context, p *auth.Principal, publicID string) error {
	v, err := s.repo.GetByPublicID(ctx, p.AccountID, publicID)
	if err != nil {
		return err
	}
	if !canEditView(p, v) {
		return httpx.Forbidden("cannot delete this view")
	}
	return s.repo.Delete(ctx, v.ID)
}

func canEditView(p *auth.Principal, v *View) bool {
	if v.AccountID != p.AccountID {
		return false
	}
	if !p.CanAction(permissions.ActionSettingsAdmin) {
		return false
	}
	return v.OwnerUserID == nil
}
