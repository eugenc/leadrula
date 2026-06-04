package collaboration

import (
	"context"
	"strings"

	"github.com/echayko/leadrula/backend/internal/auth"
	"github.com/echayko/leadrula/backend/internal/notifications"
	"github.com/echayko/leadrula/backend/pkg/httpx"
)

type Service struct {
	repo   *Repository
	notif  *notifications.Service
	tokens *auth.TokenManager
}

func NewService(repo *Repository, notif *notifications.Service, tokens *auth.TokenManager) *Service {
	return &Service{repo: repo, notif: notif, tokens: tokens}
}

func (s *Service) Repo() *Repository { return s.repo }

func (s *Service) GrantOnCreate(ctx context.Context, publisherID, buyerID, requestedBy int64) error {
	tx, err := s.repo.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	c, err := s.repo.CreateActive(ctx, tx, publisherID, buyerID, requestedBy, true)
	if err != nil {
		return err
	}
	_ = c
	if err := s.repo.InsertAudit(ctx, tx, "granted", publisherID, buyerID, requestedBy, map[string]any{
		"auto_granted": true,
	}); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *Service) solePartnershipPublisher(ctx context.Context, buyerID int64) (int64, error) {
	pubID, err := s.repo.SoleActivePartnershipPublisher(ctx, buyerID)
	if err == ErrNotFound {
		return 0, httpx.Validation("use /buyer/collaboration/publishers/{publisherId} when multiple publishers are linked")
	}
	return pubID, err
}

func (s *Service) StatusForBuyer(ctx context.Context, buyerID int64) (*StatusResponse, error) {
	pubID, err := s.solePartnershipPublisher(ctx, buyerID)
	if err != nil {
		return nil, err
	}
	return s.status(ctx, pubID, buyerID)
}

func (s *Service) PublisherForBuyer(ctx context.Context, buyerID int64) (*BuyerPublisher, error) {
	pubID, err := s.solePartnershipPublisher(ctx, buyerID)
	if err != nil {
		return nil, err
	}
	publicID, name, website, err := s.repo.GetAccountProfile(ctx, pubID)
	if err != nil {
		return nil, err
	}
	st, err := s.status(ctx, pubID, buyerID)
	if err != nil {
		return nil, err
	}
	return &BuyerPublisher{
		ID:                  publicID,
		Name:                name,
		Website:             website,
		CollaborationStatus: st.Status,
	}, nil
}

func (s *Service) AuditLogListForBuyer(ctx context.Context, buyerID int64, p AuditListParams) (*AuditListResult, error) {
	p.BuyerID = buyerID
	return s.repo.ListAuditForBuyer(ctx, p)
}

func (s *Service) AuditActorsForBuyer(ctx context.Context, buyerID int64) ([]AuditActor, error) {
	return s.repo.ListAuditActors(ctx, buyerID)
}

func (s *Service) AuditLogListForPublisher(ctx context.Context, publisherID int64, p AuditListParams) (*AuditListResult, error) {
	p.PublisherID = publisherID
	return s.repo.ListAuditForPublisher(ctx, p)
}

func (s *Service) AuditActorsForPublisher(ctx context.Context, publisherID int64) ([]AuditActor, error) {
	return s.repo.ListAuditActorsForPublisher(ctx, publisherID)
}

func (s *Service) StatusForPublisher(ctx context.Context, publisherID, buyerID int64) (*StatusResponse, error) {
	return s.status(ctx, publisherID, buyerID)
}

func (s *Service) status(ctx context.Context, publisherID, buyerID int64) (*StatusResponse, error) {
	c, err := s.repo.GetByPair(ctx, publisherID, buyerID)
	if err == ErrNotFound {
		return &StatusResponse{Status: "none"}, nil
	}
	if err != nil {
		return nil, err
	}
	pubName, _ := s.repo.AccountName(ctx, publisherID)
	buyerName, _ := s.repo.AccountName(ctx, buyerID)
	buyerPubID, _ := s.repo.GetAccountPublicID(ctx, buyerID)

	res := &StatusResponse{
		Status:        c.Status,
		Version:       c.Version,
		AutoGranted:   c.AutoGranted,
		PublisherName: pubName,
		BuyerName:     buyerName,
		BuyerID:       buyerPubID,
		CreatedAt:     &c.CreatedAt,
		RevokedAt:     c.RevokedAt,
	}
	if c.TargetPublisherUserID != nil {
		res.TargetPublisherUserName, _ = s.repo.UserName(ctx, *c.TargetPublisherUserID)
	}
	if c.RequestedByUserID != nil {
		res.RequestedByName, _ = s.repo.UserName(ctx, *c.RequestedByUserID)
	}
	audit, _ := s.repo.ListAudit(ctx, publisherID, buyerID, 20)
	res.AuditLog = audit
	return res, nil
}

func (s *Service) ListSummaries(ctx context.Context, publisherID int64) ([]BuyerCollabSummary, error) {
	return s.repo.ListByPublisher(ctx, publisherID)
}

func (s *Service) RequestFromPublisher(ctx context.Context, p *auth.Principal, buyerAccountID int64) (*StatusResponse, error) {
	if !p.IsAdmin() {
		return nil, httpx.Forbidden("admin required")
	}
	var accountType string
	if err := s.repo.pool.QueryRow(ctx, `SELECT type FROM accounts WHERE id = $1`, buyerAccountID).Scan(&accountType); err != nil {
		return nil, httpx.NotFound("buyer not found")
	}
	if accountType != "buyer" {
		return nil, httpx.NotFound("buyer not found")
	}

	existing, err := s.repo.GetByPair(ctx, p.AccountID, buyerAccountID)
	if err == nil {
		switch existing.Status {
		case StatusActive:
			return nil, httpx.Conflict("collaboration already active")
		case StatusPendingBuyer:
			return nil, httpx.Conflict("request already pending")
		case StatusPendingPublisher:
			return nil, httpx.Conflict("buyer invite already pending")
		case StatusRevoked:
			c, err := s.repo.ResetToPendingBuyer(ctx, existing.ID, p.UserID)
			if err != nil {
				return nil, err
			}
			if err := s.auditAndNotifyRequest(ctx, p.AccountID, buyerAccountID, p.UserID, c.ID, "publisher"); err != nil {
				return nil, err
			}
			return s.status(ctx, p.AccountID, buyerAccountID)
		}
	} else if err != ErrNotFound {
		return nil, err
	}

	c, err := s.repo.CreatePendingBuyer(ctx, p.AccountID, buyerAccountID, p.UserID)
	if err == ErrConflict {
		return nil, httpx.Conflict("collaboration request already exists")
	}
	if err != nil {
		return nil, err
	}
	if err := s.auditAndNotifyRequest(ctx, p.AccountID, buyerAccountID, p.UserID, c.ID, "publisher"); err != nil {
		return nil, err
	}
	return s.status(ctx, p.AccountID, buyerAccountID)
}

func (s *Service) InvitePublisherUser(ctx context.Context, p *auth.Principal, email string) (*StatusResponse, error) {
	pubID, err := s.solePartnershipPublisher(ctx, p.AccountID)
	if err != nil {
		return nil, err
	}
	return s.InvitePublisherUserForPublisher(ctx, p, pubID, email)
}

func (s *Service) InvitePublisherUserForPublisher(ctx context.Context, p *auth.Principal, publisherID int64, email string) (*StatusResponse, error) {
	if !p.IsAdmin() {
		return nil, httpx.Forbidden("admin required")
	}
	if err := s.requireActivePartnership(ctx, publisherID, p.AccountID); err != nil {
		return nil, err
	}
	email = strings.TrimSpace(strings.ToLower(email))
	if email == "" {
		return nil, httpx.Validation("email is required")
	}

	targetUserID, role, active, err := s.repo.FindPublisherUserByEmail(ctx, publisherID, email)
	if err != nil {
		return nil, httpx.Validation("email must belong to an active publisher admin")
	}
	if !active || role != "admin" {
		return nil, httpx.Validation("email must belong to an active publisher admin")
	}

	existing, err := s.repo.GetByPair(ctx, publisherID, p.AccountID)
	if err == nil {
		switch existing.Status {
		case StatusActive:
			return nil, httpx.Conflict("collaboration already active")
		case StatusPendingPublisher:
			return nil, httpx.Conflict("invite already pending")
		case StatusPendingBuyer:
			return nil, httpx.Conflict("publisher request already pending")
		case StatusRevoked:
			c, err := s.repo.ResetToPendingPublisher(ctx, existing.ID, targetUserID, p.UserID)
			if err != nil {
				return nil, err
			}
			if err := s.auditAndNotifyInvite(ctx, publisherID, p.AccountID, p.UserID, targetUserID, c.ID); err != nil {
				return nil, err
			}
			return s.status(ctx, publisherID, p.AccountID)
		}
	} else if err != ErrNotFound {
		return nil, err
	}

	c, err := s.repo.CreatePendingPublisher(ctx, publisherID, p.AccountID, targetUserID, p.UserID)
	if err == ErrConflict {
		return nil, httpx.Conflict("collaboration request already exists")
	}
	if err != nil {
		return nil, err
	}
	if err := s.auditAndNotifyInvite(ctx, publisherID, p.AccountID, p.UserID, targetUserID, c.ID); err != nil {
		return nil, err
	}
	return s.status(ctx, publisherID, p.AccountID)
}

func (s *Service) Accept(ctx context.Context, p *auth.Principal, publisherID, buyerAccountID int64) (*StatusResponse, error) {
	c, err := s.repo.GetByPair(ctx, publisherID, buyerAccountID)
	if err != nil {
		return nil, httpx.NotFound("no pending collaboration request")
	}

	switch c.Status {
	case StatusPendingBuyer:
		if p.AccountType != "buyer" || !p.IsAdmin() || p.AccountID != buyerAccountID {
			return nil, httpx.Forbidden("buyer admin required")
		}
	case StatusPendingPublisher:
		if p.AccountType != "publisher" || c.TargetPublisherUserID == nil || p.UserID != *c.TargetPublisherUserID {
			return nil, httpx.Forbidden("target publisher user required")
		}
	default:
		return nil, httpx.Validation("no pending request to accept")
	}

	activated, err := s.repo.Activate(ctx, c.ID)
	if err != nil {
		return nil, err
	}
	if err := s.repo.InsertAudit(ctx, s.repo.pool, "request_accepted", publisherID, buyerAccountID, p.UserID, map[string]any{}); err != nil {
		return nil, err
	}
	if err := s.repo.InsertAudit(ctx, s.repo.pool, "granted", publisherID, buyerAccountID, p.UserID, map[string]any{}); err != nil {
		return nil, err
	}
	_ = activated
	return s.status(ctx, publisherID, buyerAccountID)
}

func (s *Service) AcceptForBuyer(ctx context.Context, p *auth.Principal) (*StatusResponse, error) {
	pubID, err := s.solePartnershipPublisher(ctx, p.AccountID)
	if err != nil {
		return nil, err
	}
	return s.Accept(ctx, p, pubID, p.AccountID)
}

func (s *Service) AcceptForBuyerPublisher(ctx context.Context, p *auth.Principal, publisherID int64) (*StatusResponse, error) {
	if err := s.requireActivePartnership(ctx, publisherID, p.AccountID); err != nil {
		return nil, err
	}
	return s.Accept(ctx, p, publisherID, p.AccountID)
}

func (s *Service) AcceptForPublisher(ctx context.Context, p *auth.Principal, buyerAccountID int64) (*StatusResponse, error) {
	return s.Accept(ctx, p, p.AccountID, buyerAccountID)
}

func (s *Service) AcceptByBuyerPublicID(ctx context.Context, p *auth.Principal, buyerPublicID string) (*StatusResponse, error) {
	buyerID, buyerType, err := s.repo.GetAccountByPublicID(ctx, buyerPublicID)
	if err != nil || buyerType != "buyer" {
		return nil, httpx.NotFound("buyer not found")
	}
	return s.Accept(ctx, p, p.AccountID, buyerID)
}

func (s *Service) Reject(ctx context.Context, p *auth.Principal, publisherID, buyerAccountID int64) (*StatusResponse, error) {
	c, err := s.repo.GetByPair(ctx, publisherID, buyerAccountID)
	if err != nil {
		return nil, httpx.NotFound("no pending collaboration request")
	}

	switch c.Status {
	case StatusPendingBuyer:
		if p.AccountType != "buyer" || !p.IsAdmin() || p.AccountID != buyerAccountID {
			return nil, httpx.Forbidden("buyer admin required")
		}
	case StatusPendingPublisher:
		if p.AccountType != "publisher" || c.TargetPublisherUserID == nil || p.UserID != *c.TargetPublisherUserID {
			return nil, httpx.Forbidden("target publisher user required")
		}
	default:
		return nil, httpx.Validation("no pending request to reject")
	}

	if err := s.repo.RejectPending(ctx, c.ID); err != nil {
		return nil, err
	}
	if err := s.repo.InsertAudit(ctx, s.repo.pool, "request_rejected", publisherID, buyerAccountID, p.UserID, map[string]any{}); err != nil {
		return nil, err
	}
	return s.status(ctx, publisherID, buyerAccountID)
}

func (s *Service) RejectForBuyer(ctx context.Context, p *auth.Principal) (*StatusResponse, error) {
	pubID, err := s.solePartnershipPublisher(ctx, p.AccountID)
	if err != nil {
		return nil, err
	}
	return s.Reject(ctx, p, pubID, p.AccountID)
}

func (s *Service) RejectForBuyerPublisher(ctx context.Context, p *auth.Principal, publisherID int64) (*StatusResponse, error) {
	if err := s.requireActivePartnership(ctx, publisherID, p.AccountID); err != nil {
		return nil, err
	}
	return s.Reject(ctx, p, publisherID, p.AccountID)
}

func (s *Service) RejectForPublisher(ctx context.Context, p *auth.Principal, buyerAccountID int64) (*StatusResponse, error) {
	return s.Reject(ctx, p, p.AccountID, buyerAccountID)
}

func (s *Service) RejectByBuyerPublicID(ctx context.Context, p *auth.Principal, buyerPublicID string) (*StatusResponse, error) {
	buyerID, buyerType, err := s.repo.GetAccountByPublicID(ctx, buyerPublicID)
	if err != nil || buyerType != "buyer" {
		return nil, httpx.NotFound("buyer not found")
	}
	return s.Reject(ctx, p, p.AccountID, buyerID)
}

func (s *Service) Revoke(ctx context.Context, p *auth.Principal) (*StatusResponse, error) {
	pubID, err := s.solePartnershipPublisher(ctx, p.AccountID)
	if err != nil {
		return nil, err
	}
	return s.RevokeForBuyerPublisher(ctx, p, pubID)
}

func (s *Service) RevokeForBuyerPublisher(ctx context.Context, p *auth.Principal, publisherID int64) (*StatusResponse, error) {
	if !p.IsAdmin() || p.AccountType != "buyer" {
		return nil, httpx.Forbidden("buyer admin required")
	}
	if err := s.requireActivePartnership(ctx, publisherID, p.AccountID); err != nil {
		return nil, err
	}
	c, err := s.repo.GetByPair(ctx, publisherID, p.AccountID)
	if err != nil {
		return nil, httpx.NotFound("no active collaboration")
	}
	if c.Status != StatusActive {
		return nil, httpx.Validation("collaboration is not active")
	}
	if _, err := s.repo.Revoke(ctx, c.ID, p.UserID); err != nil {
		return nil, err
	}
	if err := s.repo.InsertAudit(ctx, s.repo.pool, "revoked", publisherID, p.AccountID, p.UserID, map[string]any{}); err != nil {
		return nil, err
	}
	return s.status(ctx, publisherID, p.AccountID)
}

func (s *Service) requireActivePartnership(ctx context.Context, publisherID, buyerID int64) error {
	var ok bool
	err := s.repo.pool.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM partnerships WHERE publisher_id = $1 AND buyer_id = $2 AND status = 'active')`,
		publisherID, buyerID).Scan(&ok)
	if err != nil {
		return err
	}
	if !ok {
		return httpx.NotFound("partnership not active")
	}
	return nil
}

func (s *Service) auditAndNotifyRequest(ctx context.Context, pubID, buyerID, actorID int64, collabID int64, direction string) error {
	if err := s.repo.InsertAudit(ctx, s.repo.pool, "request_sent", pubID, buyerID, actorID, map[string]any{
		"direction": direction, "collaboration_id": collabID,
	}); err != nil {
		return err
	}
	adminIDs, err := s.adminUserIDs(ctx, buyerID)
	if err != nil {
		return err
	}
	pubName, _ := s.repo.AccountName(ctx, pubID)
	buyerPubID, _ := s.repo.GetAccountPublicID(ctx, buyerID)
	return s.notif.Enqueue(ctx, s.repo.pool, adminIDs, "collaboration_request", map[string]any{
		"direction": "publisher_to_buyer", "publisher_name": pubName, "buyer_id": buyerPubID, "collaboration_id": collabID,
	})
}

func (s *Service) auditAndNotifyInvite(ctx context.Context, pubID, buyerID, actorID, targetUserID int64, collabID int64) error {
	if err := s.repo.InsertAudit(ctx, s.repo.pool, "request_sent", pubID, buyerID, actorID, map[string]any{
		"direction": "buyer_to_publisher", "collaboration_id": collabID,
	}); err != nil {
		return err
	}
	buyerName, _ := s.repo.AccountName(ctx, buyerID)
	buyerPubID, _ := s.repo.GetAccountPublicID(ctx, buyerID)
	return s.notif.Enqueue(ctx, s.repo.pool, []int64{targetUserID}, "collaboration_request", map[string]any{
		"direction": "buyer_to_publisher", "buyer_name": buyerName, "buyer_id": buyerPubID, "collaboration_id": collabID,
	})
}

func (s *Service) adminUserIDs(ctx context.Context, accountID int64) ([]int64, error) {
	rows, err := s.repo.pool.Query(ctx,
		`SELECT id FROM users WHERE account_id = $1 AND role = 'admin' AND is_active = true`, accountID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

func (s *Service) LogImpersonationAction(ctx context.Context, p *auth.Principal, method, path string, changes []auth.ImpersonationChange) {
	if p == nil || p.Impersonator == nil {
		return
	}
	pubID := p.Impersonator.AccountID
	buyerID := p.AccountID
	meta := map[string]any{"method": method, "path": path}
	if len(changes) > 0 {
		items := make([]map[string]string, len(changes))
		for i, c := range changes {
			items[i] = map[string]string{"field": c.Field, "from": c.From, "to": c.To}
		}
		meta["changes"] = items
	}
	_ = s.repo.InsertAudit(ctx, s.repo.pool, "impersonation_action", pubID, buyerID, p.Impersonator.UserID, meta)
}

type ImpersonateResult struct {
	Access string         `json:"access"`
	User   map[string]any `json:"user"`
}

func (s *Service) StartImpersonation(ctx context.Context, p *auth.Principal, buyerPublicID string) (*ImpersonateResult, error) {
	if p.AccountType != "publisher" || !p.IsAdmin() {
		return nil, httpx.Forbidden("publisher admin required")
	}
	buyerID, buyerType, err := s.repo.GetAccountByPublicID(ctx, buyerPublicID)
	if err != nil || buyerType != "buyer" {
		return nil, httpx.NotFound("buyer not found")
	}
	c, err := s.repo.GetByPair(ctx, p.AccountID, buyerID)
	if err != nil || c.Status != StatusActive {
		return nil, httpx.Forbidden("collaboration not active")
	}
	buyerPubID, err := s.repo.GetAccountPublicID(ctx, buyerID)
	if err != nil {
		return nil, err
	}
	buyerName, _ := s.repo.AccountName(ctx, buyerID)
	email, fullName, _ := s.repo.GetUserBasic(ctx, p.UserID)
	access, err := s.tokens.IssueAccess(auth.Identity{
		UserPublicID:     p.UserPublicID,
		AccountPublicID:  buyerPubID,
		AccountType:      "buyer",
		Role:             "admin",
		Impersonating:    true,
		ImpersonatorAcct: p.AccountPublicID,
		CollabVersion:    c.Version,
	})
	if err != nil {
		return nil, err
	}
	if err := s.repo.InsertAudit(ctx, s.repo.pool, "impersonation_start", p.AccountID, buyerID, p.UserID, map[string]any{
		"buyer_name": buyerName,
	}); err != nil {
		return nil, err
	}
	return &ImpersonateResult{
		Access: access,
		User: map[string]any{
			"id": p.UserPublicID, "email": email, "full_name": fullName,
			"role": "admin", "account_type": "buyer", "account_id": buyerPubID,
			"impersonating": true,
			"buyer_account_name": buyerName,
			"impersonator": map[string]any{
				"id": p.UserPublicID, "full_name": fullName, "account_id": p.AccountPublicID,
			},
		},
	}, nil
}

func (s *Service) EndImpersonation(ctx context.Context, p *auth.Principal) error {
	if p.Impersonator == nil {
		return nil
	}
	pubID := p.Impersonator.AccountID
	buyerID := p.AccountID
	return s.repo.InsertAudit(ctx, s.repo.pool, "impersonation_end", pubID, buyerID, p.Impersonator.UserID, map[string]any{})
}
