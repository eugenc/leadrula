package accounts

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"time"

	"github.com/echayko/leadrula/backend/internal/auth"
	"github.com/echayko/leadrula/backend/pkg/httpx"
)

// Mailer sends transactional emails. Implemented by the notifications package.
type Mailer interface {
	SendInvite(to, token string) error
	SendPasswordReset(to, token string) error
}

type Service struct {
	repo   *Repository
	tokens *auth.TokenManager
	mail   Mailer
}

func NewService(repo *Repository, tokens *auth.TokenManager, mail Mailer) *Service {
	return &Service{repo: repo, tokens: tokens, mail: mail}
}

type LoginResult struct {
	Access  string          `json:"access"`
	Refresh string          `json:"refresh"`
	User    map[string]any  `json:"user"`
}

func (s *Service) Login(ctx context.Context, email, password string) (*LoginResult, error) {
	u, err := s.repo.FindUserByEmail(ctx, email)
	if err != nil || u.PasswordHash == nil || !u.IsActive {
		return nil, httpx.NewError(httpx.CodeUnauthorized, "invalid credentials")
	}
	ok, err := auth.VerifyPassword(password, *u.PasswordHash)
	if err != nil || !ok {
		return nil, httpx.NewError(httpx.CodeUnauthorized, "invalid credentials")
	}
	_ = s.repo.TouchLogin(ctx, u.ID)
	return s.issue(u)
}

func (s *Service) issue(u *AuthUser) (*LoginResult, error) {
	access, err := s.tokens.IssueAccess(auth.Identity{
		UserPublicID:    u.PublicID,
		AccountPublicID: u.AccountPubID,
		AccountType:     u.AccountType,
		Role:            u.Role,
	})
	if err != nil {
		return nil, err
	}
	refresh, err := s.tokens.IssueRefresh(u.PublicID)
	if err != nil {
		return nil, err
	}
	return &LoginResult{
		Access:  access,
		Refresh: refresh,
		User: map[string]any{
			"id": u.PublicID, "email": u.Email, "full_name": u.FullName,
			"role": u.Role, "account_type": u.AccountType, "account_id": u.AccountPubID,
		},
	}, nil
}

func (s *Service) Refresh(ctx context.Context, refreshToken string) (*LoginResult, error) {
	claims, err := s.tokens.ParseRefresh(refreshToken)
	if err != nil {
		return nil, httpx.NewError(httpx.CodeUnauthorized, "invalid refresh token")
	}
	p, err := s.repo.LoadPrincipal(ctx, claims.Subject)
	if err != nil {
		return nil, httpx.NewError(httpx.CodeUnauthorized, "user not found")
	}
	access, err := s.tokens.IssueAccess(auth.Identity{
		UserPublicID:    p.UserPublicID,
		AccountPublicID: p.AccountPublicID,
		AccountType:     p.AccountType,
		Role:            p.Role,
	})
	if err != nil {
		return nil, err
	}
	newRefresh, err := s.tokens.IssueRefresh(p.UserPublicID)
	if err != nil {
		return nil, err
	}
	return &LoginResult{Access: access, Refresh: newRefresh}, nil
}

// Me returns the current user and account as a JSON map.
func (s *Service) Me(ctx context.Context, p *auth.Principal) (map[string]any, error) {
	u, err := s.repo.GetUser(ctx, p.UserID)
	if err != nil {
		return nil, err
	}
	a, err := s.repo.GetAccount(ctx, p.AccountID)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"user": map[string]any{
			"id": u.PublicID, "email": u.Email, "full_name": u.FullName,
			"role": u.Role, "is_active": u.IsActive, "prefs": rawJSON(u.Prefs),
		},
		"account": map[string]any{
			"id": a.PublicID, "type": a.Type, "name": a.Name, "timezone": a.Timezone,
		},
	}, nil
}

func (s *Service) RequestPasswordReset(ctx context.Context, email string) error {
	u, err := s.repo.FindUserByEmail(ctx, email)
	if err != nil {
		return nil // do not leak which emails exist
	}
	token := randomToken()
	if err := s.repo.CreatePasswordReset(ctx, u.ID, token, time.Now().Add(2*time.Hour)); err != nil {
		return err
	}
	if s.mail != nil {
		_ = s.mail.SendPasswordReset(u.Email, token)
	}
	return nil
}

func (s *Service) ConfirmPasswordReset(ctx context.Context, token, newPassword string) error {
	row, err := s.repo.FindResetByToken(ctx, token)
	if err != nil {
		return httpx.NewError(httpx.CodeValidation, "invalid token")
	}
	if row.Used || time.Now().After(row.ExpiresAt) {
		return httpx.NewError(httpx.CodeValidation, "token expired or used")
	}
	hash, err := auth.HashPassword(newPassword)
	if err != nil {
		return err
	}
	return s.repo.ConsumeReset(ctx, row.ID, row.UserID, hash)
}

func (s *Service) Invite(ctx context.Context, accountID int64, email, role string) (*Invite, error) {
	if role == "" {
		role = "user"
	}
	token := randomToken()
	inv, err := s.repo.CreateInvite(ctx, accountID, email, role, token, time.Now().Add(72*time.Hour))
	if err != nil {
		return nil, err
	}
	if s.mail != nil {
		_ = s.mail.SendInvite(email, token)
	}
	return inv, nil
}

func (s *Service) AcceptInvite(ctx context.Context, token, fullName, password string) (*LoginResult, error) {
	inv, err := s.repo.FindInviteByToken(ctx, token)
	if err != nil {
		return nil, httpx.NewError(httpx.CodeValidation, "invalid invite token")
	}
	if inv.Accepted || time.Now().After(inv.ExpiresAt) {
		return nil, httpx.NewError(httpx.CodeValidation, "invite expired or already used")
	}
	hash, err := auth.HashPassword(password)
	if err != nil {
		return nil, err
	}
	if _, err := s.repo.AcceptInvite(ctx, inv, fullName, hash); err != nil {
		return nil, err
	}
	u, err := s.repo.FindUserByEmail(ctx, inv.Email)
	if err != nil {
		return nil, err
	}
	return s.issue(u)
}

func (s *Service) ListUsers(ctx context.Context, accountID int64) ([]User, error) {
	return s.repo.ListUsers(ctx, accountID)
}

func (s *Service) UpdateUser(ctx context.Context, accountID, userID int64, role *string, isActive *bool) (*User, error) {
	return s.repo.UpdateUser(ctx, accountID, userID, role, isActive)
}

func (s *Service) DeleteUser(ctx context.Context, accountID, userID int64) error {
	return s.repo.DeleteUser(ctx, accountID, userID)
}

func randomToken() string {
	b := make([]byte, 32)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func rawJSON(b []byte) any {
	if len(b) == 0 {
		return map[string]any{}
	}
	return jsonRaw(b)
}

// jsonRaw wraps bytes so the encoder emits them verbatim.
type jsonRaw []byte

func (j jsonRaw) MarshalJSON() ([]byte, error) {
	if len(j) == 0 {
		return []byte("null"), nil
	}
	return j, nil
}
