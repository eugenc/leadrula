package accounts

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	"github.com/echayko/leadrula/backend/internal/auth"
	"github.com/echayko/leadrula/backend/internal/database"
	"github.com/echayko/leadrula/backend/internal/permissions"
	"github.com/echayko/leadrula/backend/internal/storage"
	"github.com/echayko/leadrula/backend/pkg/httpx"
)

// Mailer sends transactional emails. Implemented by the notifications package.
type Mailer interface {
	SendInvite(to, fullName, token string) error
	SendPasswordReset(to, fullName, token string) error
}

type Service struct {
	repo    *Repository
	tokens  *auth.TokenManager
	mail    Mailer
	avatars *storage.AvatarStore
}

func NewService(repo *Repository, tokens *auth.TokenManager, mail Mailer, avatars *storage.AvatarStore) *Service {
	return &Service{repo: repo, tokens: tokens, mail: mail, avatars: avatars}
}

type LoginResult struct {
	Access  string         `json:"access"`
	Refresh string         `json:"refresh"`
	User    map[string]any `json:"user"`
}

func (s *Service) Login(ctx context.Context, email, password string) (*LoginResult, error) {
	email = strings.TrimSpace(strings.ToLower(email))
	users, err := s.repo.FindUsersByEmail(ctx, email)
	if err != nil {
		return nil, err
	}
	var matched []*AuthUser
	for i := range users {
		u := &users[i]
		if u.PasswordHash == nil || !u.IsActive {
			continue
		}
		if u.AccountOpStatus == AccountStatusSuspended {
			continue
		}
		ok, err := auth.VerifyPassword(password, *u.PasswordHash)
		if err != nil || !ok {
			continue
		}
		matched = append(matched, u)
	}
	if len(matched) == 0 {
		return nil, httpx.NewError(httpx.CodeUnauthorized, "invalid credentials")
	}
	u := matched[0]
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
	userRole := p.Role
	if p.Impersonator != nil {
		userRole = p.Role
	}
	effective := p.Perms
	if p.FullAccess {
		effective = permissions.FullAccess(p.AccountType)
	}
	res := map[string]any{
		"user": map[string]any{
			"id": u.PublicID, "email": u.Email, "full_name": u.FullName,
			"role": userRole, "is_active": u.IsActive, "prefs": rawJSON(u.Prefs),
			"avatar_url": avatarURLFromPrefs(u.Prefs),
			"effective_permissions": permissions.ToMap(effective),
		},
		"account": accountMeFields(a),
	}
	if p.Impersonator != nil {
		res["impersonating"] = true
		res["buyer_account_name"] = a.Name
		res["impersonator"] = map[string]any{
			"id": p.Impersonator.UserPublicID, "account_id": p.Impersonator.AccountPublicID,
		}
	}
	if p.SwitchedFrom != "" {
		res["is_switched"] = true
		res["switched_from"] = p.SwitchedFrom
	}
	switchable, _ := s.ListSwitchable(ctx, p)
	res["switchable_count"] = len(switchable)
	return res, nil
}

func (s *Service) UpdateMyAccount(ctx context.Context, p *auth.Principal, params UpdateMyAccountParams) (*Account, error) {
	if !p.CanAction(permissions.ActionSettingsAdmin) {
		return nil, httpx.Forbidden("admin role required")
	}
	if p.AccountType != "buyer" && p.AccountType != "publisher" {
		return nil, httpx.Validation("account type not supported")
	}
	if !params.HasChanges() {
		return nil, httpx.Validation("no fields to update")
	}
	if params.Timezone != nil && strings.TrimSpace(*params.Timezone) == "" {
		return nil, httpx.Validation("timezone is required")
	}

	switch p.AccountType {
	case "buyer":
		return s.UpdateBuyer(ctx, p.AccountID, UpdateBuyerParams{
			Name:         params.Name,
			Website:      params.Website,
			Timezone:     params.Timezone,
			ContactEmail: params.ContactEmail,
			Phone:        params.Phone,
			AddressLine1: params.AddressLine1,
			AddressLine2: params.AddressLine2,
			City:         params.City,
			State:        params.State,
			PostalCode:   params.PostalCode,
			Country:      params.Country,
		})
	case "publisher":
		return s.UpdatePublisher(ctx, p.AccountPublicID, UpdatePublisherParams{
			Name:         params.Name,
			Website:      params.Website,
			Timezone:     params.Timezone,
			ContactEmail: params.ContactEmail,
			Phone:        params.Phone,
			AddressLine1: params.AddressLine1,
			AddressLine2: params.AddressLine2,
			City:         params.City,
			State:        params.State,
			PostalCode:   params.PostalCode,
			Country:      params.Country,
		})
	default:
		return nil, httpx.Validation("account type not supported")
	}
}

// PatchPrefs shallow-merges patch into the user's prefs JSON and returns the merged object.
func (s *Service) PatchPrefs(ctx context.Context, userID int64, patch map[string]any) (map[string]any, error) {
	u, err := s.repo.GetUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	existing := map[string]any{}
	if len(u.Prefs) > 0 {
		_ = json.Unmarshal(u.Prefs, &existing)
	}
	for k, v := range patch {
		existing[k] = v
	}
	merged, err := json.Marshal(existing)
	if err != nil {
		return nil, err
	}
	if err := s.repo.UpdatePrefs(ctx, userID, merged); err != nil {
		return nil, err
	}
	return existing, nil
}

// UploadAvatar stores the image in object storage and saves its public URL in prefs.
func (s *Service) UploadAvatar(ctx context.Context, accountID, userID int64, contentType string, body io.Reader) (string, error) {
	if s.avatars == nil || !s.avatars.Enabled() {
		return "", httpx.NewError(httpx.CodeValidation, "avatar uploads are not configured")
	}
	ext, ok := avatarExt(contentType)
	if !ok {
		return "", httpx.Validation("unsupported image type; use JPEG, PNG, or WebP")
	}
	data, err := io.ReadAll(io.LimitReader(body, maxAvatarBytes+1))
	if err != nil {
		return "", err
	}
	if len(data) == 0 {
		return "", httpx.Validation("empty file")
	}
	if len(data) > maxAvatarBytes {
		return "", httpx.Validation("image must be 2 MB or smaller")
	}

	u, err := s.repo.GetUser(ctx, userID)
	if err != nil {
		if err == ErrNotFound {
			return "", httpx.NotFound("user not found")
		}
		return "", err
	}
	if u.AccountID != accountID {
		return "", httpx.Forbidden("user not in this account")
	}

	key := fmt.Sprintf("avatars/%s%s", u.PublicID, ext)
	url, err := s.avatars.Put(ctx, key, contentType, bytes.NewReader(data))
	if err != nil {
		return "", err
	}
	if _, err := s.PatchPrefs(ctx, userID, map[string]any{"avatar_url": url}); err != nil {
		return "", err
	}
	return url, nil
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
		_ = s.mail.SendPasswordReset(u.Email, u.FullName, token)
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

func (s *Service) Invite(ctx context.Context, accountID int64, email, fullName, role string, permRaw []byte) (*Invite, error) {
	email = strings.TrimSpace(strings.ToLower(email))
	fullName = strings.TrimSpace(fullName)
	if email == "" {
		return nil, httpx.Validation("email is required")
	}
	if role == "" {
		role = "user"
	}

	if _, err := s.repo.FindUserByEmailInAccount(ctx, accountID, email); err == nil {
		return nil, httpx.Conflict("email already registered")
	} else if err != ErrNotFound {
		return nil, err
	}
	if _, err := s.repo.FindPendingInviteByEmail(ctx, accountID, email); err == nil {
		return nil, httpx.Conflict("invite already pending for this email")
	} else if err != ErrNotFound {
		return nil, err
	}

	token := randomToken()
	expires := time.Now().Add(72 * time.Hour)
	if existing, err := s.repo.FindUnacceptedInviteByEmail(ctx, accountID, email); err == nil {
		rolePtr := &role
		namePtr := &fullName
		var permPtr *[]byte
		if len(permRaw) > 0 {
			permPtr = &permRaw
		}
		inv, err := s.repo.UpdateInvite(ctx, accountID, existing.ID, nil, namePtr, rolePtr, &token, &expires, permPtr)
		if err != nil {
			return nil, err
		}
		if err := s.sendInviteEmail(email, fullName, token); err != nil {
			return nil, err
		}
		return inv, nil
	} else if err != ErrNotFound {
		return nil, err
	}

	inv, err := s.repo.CreateInvite(ctx, accountID, email, fullName, role, token, expires, permRaw)
	if err != nil {
		return nil, err
	}
	if err := s.sendInviteEmail(email, fullName, token); err != nil {
		return nil, err
	}
	return inv, nil
}

var allowedTimezones = map[string]struct{}{
	"America/Toronto":     {},
	"America/New_York":    {},
	"America/Chicago":     {},
	"America/Denver":      {},
	"America/Los_Angeles": {},
	"America/Phoenix":     {},
	"America/Anchorage":   {},
	"Pacific/Honolulu":    {},
	"UTC":                 {},
}

func (s *Service) sendInviteEmail(to, fullName, token string) error {
	if s.mail == nil {
		return nil
	}
	if err := s.mail.SendInvite(to, fullName, token); err != nil {
		return httpx.ServiceUnavailable("invite email could not be sent")
	}
	return nil
}

func (s *Service) CreateBuyer(ctx context.Context, p CreateBuyerParams) (*CreateBuyerResult, error) {
	p.Name = strings.TrimSpace(p.Name)
	p.Website = strings.TrimSpace(p.Website)
	p.AdminEmail = strings.TrimSpace(strings.ToLower(p.AdminEmail))
	p.AdminFirstName = strings.TrimSpace(p.AdminFirstName)
	p.AdminLastName = strings.TrimSpace(p.AdminLastName)
	p.Timezone = strings.TrimSpace(p.Timezone)

	if p.Name == "" || p.AdminEmail == "" || p.AdminFirstName == "" || p.AdminLastName == "" {
		return nil, httpx.Validation("name, admin email, and admin name are required")
	}
	if p.StartingBalance < 0 {
		return nil, httpx.Validation("starting balance must be zero or positive")
	}
	if p.Timezone == "" {
		p.Timezone = "America/Toronto"
	}
	if _, ok := allowedTimezones[p.Timezone]; !ok {
		return nil, httpx.Validation("invalid timezone")
	}

	existing, err := s.repo.FindUserByEmail(ctx, p.AdminEmail)
	if err != nil && err != ErrNotFound {
		return nil, err
	}
	if existing != nil {
		if existing.AccountType != "buyer" {
			return nil, httpx.Conflict("email already registered")
		}
		existingAdmin, err := s.repo.FindBuyerAdminByEmail(ctx, p.AdminEmail)
		if err != nil {
			if err == ErrNotFound {
				return nil, httpx.Conflict("email already registered")
			}
			return nil, err
		}
		res, err := s.repo.CreateBuyerWithExistingAdmin(ctx, p, existingAdmin)
		if err != nil {
			if database.IsUniqueViolation(err) {
				return nil, httpx.Conflict("email already registered")
			}
			return nil, err
		}
		return res, nil
	}

	token := randomToken()
	res, err := s.repo.CreateBuyer(ctx, p, token, time.Now().Add(72*time.Hour))
	if err != nil {
		if database.IsUniqueViolation(err) {
			return nil, httpx.Conflict("email already registered")
		}
		return nil, err
	}
	adminName := strings.TrimSpace(p.AdminFirstName + " " + p.AdminLastName)
	if err := s.sendInviteEmail(res.AdminEmail, adminName, res.InviteToken); err != nil {
		return res, err
	}
	return res, nil
}

func (s *Service) UpdateBuyer(ctx context.Context, id int64, p UpdateBuyerParams) (*Account, error) {
	if p.Name != nil {
		name := strings.TrimSpace(*p.Name)
		if name == "" {
			return nil, httpx.Validation("name is required")
		}
		p.Name = &name
	}
	if p.Website != nil {
		website := strings.TrimSpace(*p.Website)
		p.Website = &website
	}
	if p.Timezone != nil {
		tz := strings.TrimSpace(*p.Timezone)
		if tz == "" {
			tz = "America/Toronto"
		}
		if _, ok := allowedTimezones[tz]; !ok {
			return nil, httpx.Validation("invalid timezone")
		}
		p.Timezone = &tz
	}
	if p.BuyerKind != nil {
		kind := strings.TrimSpace(*p.BuyerKind)
		if kind != BuyerKindDirect && kind != BuyerKindMarketplace {
			return nil, httpx.Validation("invalid buyer_kind")
		}
		p.BuyerKind = &kind
	}
	if err := normalizeBusinessProfile(
		&p.ContactEmail, &p.Phone, &p.AddressLine1, &p.AddressLine2,
		&p.City, &p.State, &p.PostalCode, &p.Country,
	); err != nil {
		return nil, err
	}

	a, err := s.repo.UpdateBuyer(ctx, id, p)
	if err != nil {
		if err == ErrNotFound {
			return nil, httpx.NotFound("buyer not found")
		}
		return nil, err
	}
	return a, nil
}

func (s *Service) UpdatePublisher(ctx context.Context, publicID string, p UpdatePublisherParams) (*Account, error) {
	if p.Name != nil {
		name := strings.TrimSpace(*p.Name)
		if name == "" {
			return nil, httpx.Validation("name is required")
		}
		p.Name = &name
	}
	if p.Timezone != nil {
		tz := strings.TrimSpace(*p.Timezone)
		if tz == "" {
			tz = "America/Toronto"
		}
		if _, ok := allowedTimezones[tz]; !ok {
			return nil, httpx.Validation("invalid timezone")
		}
		p.Timezone = &tz
	}
	if p.Website != nil {
		website := strings.TrimSpace(*p.Website)
		p.Website = &website
	}
	if err := normalizeBusinessProfile(
		&p.ContactEmail, &p.Phone, &p.AddressLine1, &p.AddressLine2,
		&p.City, &p.State, &p.PostalCode, &p.Country,
	); err != nil {
		return nil, err
	}

	a, err := s.repo.UpdatePublisher(ctx, publicID, p)
	if err != nil {
		if err == ErrNotFound {
			return nil, httpx.NotFound("publisher not found")
		}
		return nil, err
	}
	return a, nil
}

func (s *Service) UpdateBuyerByPublicID(ctx context.Context, publicID string, p UpdateBuyerParams) (*Account, error) {
	if p.Name != nil {
		name := strings.TrimSpace(*p.Name)
		if name == "" {
			return nil, httpx.Validation("name is required")
		}
		p.Name = &name
	}
	if p.Website != nil {
		website := strings.TrimSpace(*p.Website)
		p.Website = &website
	}
	if p.Timezone != nil {
		tz := strings.TrimSpace(*p.Timezone)
		if tz == "" {
			tz = "America/Toronto"
		}
		if _, ok := allowedTimezones[tz]; !ok {
			return nil, httpx.Validation("invalid timezone")
		}
		p.Timezone = &tz
	}
	if p.BuyerKind != nil {
		kind := strings.TrimSpace(*p.BuyerKind)
		if kind != BuyerKindDirect && kind != BuyerKindMarketplace {
			return nil, httpx.Validation("invalid buyer_kind")
		}
		p.BuyerKind = &kind
	}
	if err := normalizeBusinessProfile(
		&p.ContactEmail, &p.Phone, &p.AddressLine1, &p.AddressLine2,
		&p.City, &p.State, &p.PostalCode, &p.Country,
	); err != nil {
		return nil, err
	}

	a, err := s.repo.UpdateBuyerByPublicID(ctx, publicID, p)
	if err != nil {
		if err == ErrNotFound {
			return nil, httpx.NotFound("buyer not found")
		}
		return nil, err
	}
	return a, nil
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
	name := fullName
	if inv.FullName != "" {
		name = inv.FullName
	}
	if _, err := s.repo.AcceptInvite(ctx, inv, name, hash); err != nil {
		return nil, err
	}
	u, err := s.repo.FindUserByEmail(ctx, inv.Email)
	if err != nil {
		return nil, err
	}
	return s.issue(u)
}

func (s *Service) ListUsers(ctx context.Context, accountID int64) ([]UserListItem, error) {
	acct, err := s.repo.GetAccount(ctx, accountID)
	if err != nil {
		return nil, err
	}
	members, err := s.repo.ListUsers(ctx, accountID)
	if err != nil {
		return nil, err
	}
	invites, err := s.repo.ListPendingInvites(ctx, accountID)
	if err != nil {
		return nil, err
	}

	out := make([]UserListItem, 0, len(members)+len(invites))
	for _, inv := range invites {
		perms, effective := inviteListFields(inv, acct.Type)
		out = append(out, UserListItem{
			InviteID:             inv.ID,
			Email:                inv.Email,
			FullName:             inv.FullName,
			Role:                 inv.Role,
			Status:               inviteStatus(inv.ExpiresAt),
			Permissions:          perms,
			EffectivePermissions: effective,
		})
	}
	for _, u := range members {
		status := "active"
		if !u.IsActive {
			status = "inactive"
		}
		perms, effective := userListFields(u, acct.Type)
		out = append(out, UserListItem{
			ID:                   u.ID,
			PublicID:             u.PublicID,
			Email:                u.Email,
			FullName:             u.FullName,
			Role:                 u.Role,
			Status:               status,
			AvatarURL:            avatarURLFromPrefs(u.Prefs),
			Permissions:          perms,
			EffectivePermissions: effective,
		})
	}

	sort.Slice(out, func(i, j int) bool {
		iInvite := isInviteListStatus(out[i].Status)
		jInvite := isInviteListStatus(out[j].Status)
		if iInvite && !jInvite {
			return true
		}
		if !iInvite && jInvite {
			return false
		}
		ni := strings.ToLower(out[i].FullName)
		if ni == "" {
			ni = strings.ToLower(out[i].Email)
		}
		nj := strings.ToLower(out[j].FullName)
		if nj == "" {
			nj = strings.ToLower(out[j].Email)
		}
		return ni < nj
	})

	return out, nil
}

func (s *Service) UpdateUser(ctx context.Context, accountID, userID int64, p UpdateUserParams) (*UserListItem, error) {
	existing, err := s.repo.GetUserInAccount(ctx, accountID, userID)
	if err != nil {
		if err == ErrNotFound {
			return nil, httpx.NotFound("user not found")
		}
		return nil, err
	}
	acct, err := s.repo.GetAccount(ctx, accountID)
	if err != nil {
		return nil, err
	}
	if p.Permissions != nil {
		var o permissions.Overrides
		if err := json.Unmarshal(*p.Permissions, &o); err != nil {
			return nil, httpx.Validation("invalid permissions")
		}
		if err := permissions.ValidateOverrides(acct.Type, o); err != nil {
			return nil, httpx.Validation(err.Error())
		}
	}
	if p.Email != nil {
		email := strings.TrimSpace(strings.ToLower(*p.Email))
		if email == "" {
			return nil, httpx.Validation("email is required")
		}
		p.Email = &email
		if existing, err := s.repo.FindUserByEmailInAccount(ctx, accountID, email); err == nil && existing.ID != userID {
			return nil, httpx.Conflict("email already registered")
		} else if err != nil && err != ErrNotFound {
			return nil, err
		}
	}
	if p.FullName != nil {
		name := strings.TrimSpace(*p.FullName)
		p.FullName = &name
	}

	newRole := existing.Role
	if p.Role != nil {
		newRole = *p.Role
	}
	newPerms := existing.Permissions
	if p.Permissions != nil {
		newPerms = *p.Permissions
	}
	if err := s.ensureFullAdminRemains(ctx, accountID, userID, newRole, newPerms); err != nil {
		return nil, err
	}

	u, err := s.repo.UpdateUser(ctx, accountID, userID, p)
	if err != nil {
		if err == ErrNotFound {
			return nil, httpx.NotFound("user not found")
		}
		if database.IsUniqueViolation(err) {
			return nil, httpx.Conflict("email already registered")
		}
		return nil, err
	}

	status := "active"
	if !u.IsActive {
		status = "inactive"
	}
	perms, effective := userListFields(*u, acct.Type)
	return &UserListItem{
		ID:                   u.ID,
		PublicID:             u.PublicID,
		Email:                u.Email,
		FullName:             u.FullName,
		Role:                 u.Role,
		Status:               status,
		AvatarURL:            avatarURLFromPrefs(u.Prefs),
		Permissions:          perms,
		EffectivePermissions: effective,
	}, nil
}

func (s *Service) DeleteUser(ctx context.Context, accountID, actorUserID, userID int64) error {
	if actorUserID == userID {
		return httpx.BusinessRule("cannot remove yourself")
	}

	u, err := s.repo.GetUserInAccount(ctx, accountID, userID)
	if err != nil {
		if err == ErrNotFound {
			return httpx.NotFound("user not found")
		}
		return err
	}
	if !u.IsActive {
		return nil
	}

	acct, err := s.repo.GetAccount(ctx, accountID)
	if err != nil {
		return err
	}
	if permissions.Resolve(u.Role, acct.Type, u.Permissions).IsFullAdmin() {
		remaining, err := s.countFullAccessAdmins(ctx, accountID, userID)
		if err != nil {
			return err
		}
		if remaining == 0 {
			return httpx.BusinessRule("cannot remove the last admin")
		}
	}

	if err := s.repo.ClearUserLiveRefs(ctx, accountID, userID); err != nil {
		return err
	}

	inactive := false
	if _, err := s.repo.UpdateUser(ctx, accountID, userID, UpdateUserParams{IsActive: &inactive}); err != nil {
		if err == ErrNotFound {
			return httpx.NotFound("user not found")
		}
		return err
	}
	return nil
}

func (s *Service) UpdateInvite(ctx context.Context, accountID, inviteID int64, p UpdateInviteParams) (*UserListItem, error) {
	inv, err := s.repo.GetPendingInvite(ctx, accountID, inviteID)
	if err != nil {
		if err == ErrNotFound {
			return nil, httpx.NotFound("invite not found")
		}
		return nil, err
	}

	var email, fullName, role, token *string
	var expires *time.Time
	resend := false

	if p.Email != nil {
		newEmail := strings.TrimSpace(strings.ToLower(*p.Email))
		if newEmail == "" {
			return nil, httpx.Validation("email is required")
		}
		if newEmail != inv.Email {
			if _, err := s.repo.FindUserByEmailInAccount(ctx, accountID, newEmail); err == nil {
				return nil, httpx.Conflict("email already registered")
			} else if err != ErrNotFound {
				return nil, err
			}
			if other, err := s.repo.FindPendingInviteByEmail(ctx, accountID, newEmail); err == nil && other.ID != inviteID {
				return nil, httpx.Conflict("invite already pending for this email")
			} else if err != nil && err != ErrNotFound {
				return nil, err
			}
			t := randomToken()
			token = &t
			e := time.Now().Add(72 * time.Hour)
			expires = &e
			email = &newEmail
			resend = true
		}
	}
	if p.FullName != nil {
		name := strings.TrimSpace(*p.FullName)
		fullName = &name
	}
	if p.Role != nil {
		role = p.Role
	}

	acct, err := s.repo.GetAccount(ctx, accountID)
	if err != nil {
		return nil, err
	}
	if p.Permissions != nil {
		var o permissions.Overrides
		if err := json.Unmarshal(*p.Permissions, &o); err != nil {
			return nil, httpx.Validation("invalid permissions")
		}
		if err := permissions.ValidateOverrides(acct.Type, o); err != nil {
			return nil, httpx.Validation(err.Error())
		}
	}
	newRole := inv.Role
	if role != nil {
		newRole = *role
	}
	newPerms := inv.Permissions
	if p.Permissions != nil {
		newPerms = *p.Permissions
	}
	if err := s.ensureFullAdminRemainsWithInvite(ctx, accountID, inviteID, newRole, newPerms); err != nil {
		return nil, err
	}

	updated, err := s.repo.UpdateInvite(ctx, accountID, inviteID, email, fullName, role, token, expires, p.Permissions)
	if err != nil {
		if err == ErrNotFound {
			return nil, httpx.NotFound("invite not found")
		}
		return nil, err
	}

	sendToken := inv.Token
	if token != nil {
		sendToken = *token
	}
	if resend && s.mail != nil {
		_ = s.mail.SendInvite(updated.Email, updated.FullName, sendToken)
	}

	perms, effective := inviteListFields(*updated, acct.Type)
	return &UserListItem{
		InviteID:             updated.ID,
		Email:                updated.Email,
		FullName:             updated.FullName,
		Role:                 updated.Role,
		Status:               inviteStatus(updated.ExpiresAt),
		Permissions:          perms,
		EffectivePermissions: effective,
	}, nil
}

func (s *Service) DeleteInvite(ctx context.Context, accountID, inviteID int64) error {
	if err := s.repo.DeleteInvite(ctx, accountID, inviteID); err != nil {
		if err == ErrNotFound {
			return httpx.NotFound("invite not found")
		}
		return err
	}
	return nil
}

func (s *Service) ResendInvite(ctx context.Context, accountID, inviteID int64) error {
	inv, err := s.repo.GetPendingInvite(ctx, accountID, inviteID)
	if err != nil {
		if err == ErrNotFound {
			return httpx.NotFound("invite not found")
		}
		return err
	}

	token := randomToken()
	expires := time.Now().Add(72 * time.Hour)
	if _, err := s.repo.UpdateInvite(ctx, accountID, inviteID, nil, nil, nil, &token, &expires, nil); err != nil {
		return err
	}
	return s.sendInviteEmail(inv.Email, inv.FullName, token)
}

func (s *Service) ResendBuyerAdminInvite(ctx context.Context, buyerAccountID int64) error {
	inv, err := s.repo.FindPendingAdminInvite(ctx, buyerAccountID)
	if err != nil {
		if err == ErrNotFound {
			return httpx.NotFound("no pending admin invite")
		}
		return err
	}
	return s.ResendInvite(ctx, buyerAccountID, inv.ID)
}

func inviteStatus(expiresAt time.Time) string {
	if time.Now().After(expiresAt) {
		return "expired"
	}
	return "pending"
}

func isInviteListStatus(status string) bool {
	return status == "pending" || status == "expired"
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
