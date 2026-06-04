package accounts

import (
	"context"
	"strings"
	"time"

	"github.com/echayko/leadrula/backend/internal/database"
	"github.com/echayko/leadrula/backend/pkg/httpx"
)

func (s *Service) CreatePublisher(ctx context.Context, p CreatePublisherParams) (*Account, error) {
	p.Name = strings.TrimSpace(p.Name)
	p.AdminEmail = strings.TrimSpace(strings.ToLower(p.AdminEmail))
	p.AdminFirstName = strings.TrimSpace(p.AdminFirstName)
	p.AdminLastName = strings.TrimSpace(p.AdminLastName)
	p.Timezone = strings.TrimSpace(p.Timezone)

	if p.Name == "" || p.AdminEmail == "" || p.AdminFirstName == "" || p.AdminLastName == "" {
		return nil, httpx.Validation("name, admin email, and admin name are required")
	}
	if p.Timezone == "" {
		p.Timezone = "America/Toronto"
	}
	if _, ok := allowedTimezones[p.Timezone]; !ok {
		return nil, httpx.Validation("invalid timezone")
	}

	if _, err := s.repo.FindUserByEmail(ctx, p.AdminEmail); err == nil {
		return nil, httpx.Conflict("email already registered")
	} else if err != ErrNotFound {
		return nil, err
	}

	token := randomToken()
	res, err := s.repo.CreatePublisher(ctx, p, token, time.Now().Add(72*time.Hour))
	if err != nil {
		if database.IsUniqueViolation(err) {
			return nil, httpx.Conflict("email already registered")
		}
		return nil, err
	}
	if s.mail != nil {
		adminName := strings.TrimSpace(p.AdminFirstName + " " + p.AdminLastName)
		_ = s.mail.SendInvite(res.AdminEmail, adminName, res.InviteToken)
	}
	return &res.Publisher, nil
}

func (s *Service) CreatePlatformBuyer(ctx context.Context, p CreateBuyerParams) (*Account, error) {
	summary, err := s.CreateBuyer(ctx, p)
	if err != nil {
		return nil, err
	}
	acct, err := s.repo.GetAccount(ctx, summary.ID)
	if err != nil {
		return nil, err
	}
	return acct, nil
}
