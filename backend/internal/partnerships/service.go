package partnerships

import (
	"context"
	"strings"

	"github.com/echayko/leadrula/backend/internal/accounts"
	"github.com/echayko/leadrula/backend/internal/auth"
	"github.com/echayko/leadrula/backend/internal/notifications"
	"github.com/echayko/leadrula/backend/pkg/httpx"
)

type Service struct {
	repo    *Repository
	accounts *accounts.Repository
	notif   *notifications.Service
}

func NewService(repo *Repository, accounts *accounts.Repository, notif *notifications.Service) *Service {
	return &Service{repo: repo, accounts: accounts, notif: notif}
}

func (s *Service) GrantOnCreate(ctx context.Context, publisherID, buyerID, requestedBy int64) error {
	return s.repo.UpsertActive(ctx, publisherID, buyerID, requestedBy)
}

func (s *Service) ListForPublisher(ctx context.Context, publisherID int64) ([]ListItem, error) {
	return s.repo.ListForPublisher(ctx, publisherID)
}

func (s *Service) ListForBuyer(ctx context.Context, buyerID int64) ([]ListItem, error) {
	return s.repo.ListForBuyer(ctx, buyerID)
}

func (s *Service) HasActive(ctx context.Context, publisherID, buyerID int64) (bool, error) {
	return s.repo.HasActive(ctx, publisherID, buyerID)
}

func (s *Service) RequestFromPublisher(ctx context.Context, p *auth.Principal, buyerHandlerID string) (*ListItem, error) {
	if !p.IsAdmin() {
		return nil, httpx.Forbidden("admin required")
	}
	buyerHandlerID = strings.TrimSpace(strings.ToUpper(buyerHandlerID))
	if buyerHandlerID == "" {
		return nil, httpx.Validation("buyer_handler_id is required")
	}
	if !strings.HasPrefix(buyerHandlerID, "B-") {
		return nil, httpx.Validation("invalid buyer handler id")
	}

	buyer, err := s.accounts.GetAccountByHandlerID(ctx, buyerHandlerID)
	if err != nil {
		if err == accounts.ErrNotFound {
			return nil, httpx.NotFound("buyer not found")
		}
		return nil, err
	}
	if buyer.Type != "buyer" {
		return nil, httpx.NotFound("buyer not found")
	}

	existing, err := s.repo.GetByPair(ctx, p.AccountID, buyer.ID)
	if err == nil {
		switch existing.Status {
		case StatusActive:
			return nil, httpx.Conflict("partnership already active")
		case StatusPendingBuyer:
			return nil, httpx.Conflict("request already pending")
		case StatusPendingPublisher:
			return nil, httpx.Conflict("buyer request already pending")
		case StatusRejected, StatusRevoked:
			partner, err := s.repo.ResetToPendingBuyer(ctx, existing.ID, p.UserID)
			if err != nil {
				return nil, err
			}
			if err := s.notifyRequest(ctx, p.AccountID, buyer.ID, p.UserID, partner.ID, "publisher_to_buyer"); err != nil {
				return nil, err
			}
			return s.itemForPublisher(ctx, partner)
		default:
			return nil, httpx.Conflict("partnership already exists")
		}
	}
	if err != ErrNotFound {
		return nil, err
	}

	partner, err := s.repo.CreatePendingBuyer(ctx, p.AccountID, buyer.ID, p.UserID)
	if err != nil {
		if err == ErrConflict {
			return nil, httpx.Conflict("partnership already exists")
		}
		return nil, err
	}
	if err := s.notifyRequest(ctx, p.AccountID, buyer.ID, p.UserID, partner.ID, "publisher_to_buyer"); err != nil {
		return nil, err
	}
	return s.itemForPublisher(ctx, partner)
}

func (s *Service) RequestFromBuyer(ctx context.Context, p *auth.Principal, publisherHandlerID string) (*ListItem, error) {
	if !p.IsAdmin() {
		return nil, httpx.Forbidden("admin required")
	}
	publisherHandlerID = strings.TrimSpace(strings.ToUpper(publisherHandlerID))
	if publisherHandlerID == "" {
		return nil, httpx.Validation("publisher_handler_id is required")
	}
	if !strings.HasPrefix(publisherHandlerID, "P-") {
		return nil, httpx.Validation("invalid publisher handler id")
	}

	pub, err := s.accounts.GetAccountByHandlerID(ctx, publisherHandlerID)
	if err != nil {
		if err == accounts.ErrNotFound {
			return nil, httpx.NotFound("publisher not found")
		}
		return nil, err
	}
	if pub.Type != "publisher" {
		return nil, httpx.NotFound("publisher not found")
	}

	existing, err := s.repo.GetByPair(ctx, pub.ID, p.AccountID)
	if err == nil {
		switch existing.Status {
		case StatusActive:
			return nil, httpx.Conflict("partnership already active")
		case StatusPendingPublisher:
			return nil, httpx.Conflict("request already pending")
		case StatusPendingBuyer:
			return nil, httpx.Conflict("publisher request already pending")
		case StatusRejected, StatusRevoked:
			partner, err := s.repo.ResetToPendingPublisher(ctx, existing.ID, p.UserID)
			if err != nil {
				return nil, err
			}
			if err := s.notifyRequest(ctx, pub.ID, p.AccountID, p.UserID, partner.ID, "buyer_to_publisher"); err != nil {
				return nil, err
			}
			return s.itemForBuyer(ctx, partner)
		default:
			return nil, httpx.Conflict("partnership already exists")
		}
	}
	if err != ErrNotFound {
		return nil, err
	}

	partner, err := s.repo.CreatePendingPublisher(ctx, pub.ID, p.AccountID, p.UserID)
	if err != nil {
		if err == ErrConflict {
			return nil, httpx.Conflict("partnership already exists")
		}
		return nil, err
	}
	if err := s.notifyRequest(ctx, pub.ID, p.AccountID, p.UserID, partner.ID, "buyer_to_publisher"); err != nil {
		return nil, err
	}
	return s.itemForBuyer(ctx, partner)
}

func (s *Service) AcceptForPublisher(ctx context.Context, p *auth.Principal, partnershipID int64) (*ListItem, error) {
	if !p.IsAdmin() {
		return nil, httpx.Forbidden("admin required")
	}
	partner, err := s.repo.GetByID(ctx, partnershipID)
	if err != nil {
		if err == ErrNotFound {
			return nil, httpx.NotFound("partnership not found")
		}
		return nil, err
	}
	if partner.PublisherID != p.AccountID {
		return nil, httpx.NotFound("partnership not found")
	}
	if partner.Status != StatusPendingPublisher {
		return nil, httpx.Validation("no pending request to accept")
	}
	activated, err := s.repo.Activate(ctx, partnershipID)
	if err != nil {
		return nil, err
	}
	if err := s.notifyAccepted(ctx, activated, "publisher"); err != nil {
		return nil, err
	}
	return s.itemForPublisher(ctx, activated)
}

func (s *Service) RejectForPublisher(ctx context.Context, p *auth.Principal, partnershipID int64) error {
	if !p.IsAdmin() {
		return httpx.Forbidden("admin required")
	}
	partner, err := s.repo.GetByID(ctx, partnershipID)
	if err != nil {
		if err == ErrNotFound {
			return httpx.NotFound("partnership not found")
		}
		return err
	}
	if partner.PublisherID != p.AccountID {
		return httpx.NotFound("partnership not found")
	}
	if partner.Status != StatusPendingPublisher {
		return httpx.Validation("no pending request to reject")
	}
	return s.repo.Reject(ctx, partnershipID)
}

func (s *Service) AcceptForBuyer(ctx context.Context, p *auth.Principal, partnershipID int64) (*ListItem, error) {
	if !p.IsAdmin() {
		return nil, httpx.Forbidden("admin required")
	}
	partner, err := s.repo.GetByID(ctx, partnershipID)
	if err != nil {
		if err == ErrNotFound {
			return nil, httpx.NotFound("partnership not found")
		}
		return nil, err
	}
	if partner.BuyerID != p.AccountID {
		return nil, httpx.NotFound("partnership not found")
	}
	if partner.Status != StatusPendingBuyer {
		return nil, httpx.Validation("no pending request to accept")
	}
	activated, err := s.repo.Activate(ctx, partnershipID)
	if err != nil {
		return nil, err
	}
	if err := s.notifyAccepted(ctx, activated, "buyer"); err != nil {
		return nil, err
	}
	return s.itemForBuyer(ctx, activated)
}

func (s *Service) RejectForBuyer(ctx context.Context, p *auth.Principal, partnershipID int64) error {
	if !p.IsAdmin() {
		return httpx.Forbidden("admin required")
	}
	partner, err := s.repo.GetByID(ctx, partnershipID)
	if err != nil {
		if err == ErrNotFound {
			return httpx.NotFound("partnership not found")
		}
		return err
	}
	if partner.BuyerID != p.AccountID {
		return httpx.NotFound("partnership not found")
	}
	if partner.Status != StatusPendingBuyer {
		return httpx.Validation("no pending request to reject")
	}
	return s.repo.Reject(ctx, partnershipID)
}

func (s *Service) notifyRequest(ctx context.Context, pubID, buyerID, actorID, partnershipID int64, direction string) error {
	pubName, _ := s.repo.AccountName(ctx, pubID)
	buyerName, _ := s.repo.AccountName(ctx, buyerID)
	payload := map[string]any{
		"partnership_id": partnershipID,
		"direction":      direction,
		"publisher_name": pubName,
		"buyer_name":     buyerName,
	}
	var targetAccountID int64
	if direction == "publisher_to_buyer" {
		targetAccountID = buyerID
	} else {
		targetAccountID = pubID
	}
	adminIDs, err := s.repo.AdminUserIDs(ctx, s.repo.pool, targetAccountID)
	if err != nil {
		return err
	}
	return s.notif.Enqueue(ctx, s.repo.pool, adminIDs, "partnership_request", payload)
}

func (s *Service) notifyAccepted(ctx context.Context, p *Partnership, acceptedBy string) error {
	pubName, _ := s.repo.AccountName(ctx, p.PublisherID)
	buyerName, _ := s.repo.AccountName(ctx, p.BuyerID)
	payload := map[string]any{
		"partnership_id": p.ID,
		"accepted_by":    acceptedBy,
		"publisher_name": pubName,
		"buyer_name":     buyerName,
	}
	var targetAccountID int64
	if acceptedBy == "publisher" {
		targetAccountID = p.BuyerID
	} else {
		targetAccountID = p.PublisherID
	}
	adminIDs, err := s.repo.AdminUserIDs(ctx, s.repo.pool, targetAccountID)
	if err != nil {
		return err
	}
	return s.notif.Enqueue(ctx, s.repo.pool, adminIDs, "partnership_accepted", payload)
}

func (s *Service) itemForPublisher(ctx context.Context, p *Partnership) (*ListItem, error) {
	name, _ := s.repo.AccountName(ctx, p.BuyerID)
	hid, _ := s.repo.AccountHandlerID(ctx, p.BuyerID)
	return &ListItem{
		ID:               p.ID,
		Status:           p.Status,
		RequestedBy:      p.RequestedBy,
		PartnerName:      name,
		PartnerHandlerID: hid,
		CreatedAt:        p.CreatedAt,
	}, nil
}

func (s *Service) itemForBuyer(ctx context.Context, p *Partnership) (*ListItem, error) {
	name, _ := s.repo.AccountName(ctx, p.PublisherID)
	hid, _ := s.repo.AccountHandlerID(ctx, p.PublisherID)
	return &ListItem{
		ID:               p.ID,
		Status:           p.Status,
		RequestedBy:      p.RequestedBy,
		PartnerName:      name,
		PartnerHandlerID: hid,
		CreatedAt:        p.CreatedAt,
	}, nil
}
