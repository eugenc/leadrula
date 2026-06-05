package accounts

import (
	"context"

	"github.com/echayko/leadrula/backend/pkg/httpx"
)

func (s *Service) SoftDeletePublisher(ctx context.Context, publicID string) error {
	return s.softDeleteAccount(ctx, publicID, "publisher")
}

func (s *Service) SoftDeleteBuyer(ctx context.Context, publicID string) error {
	return s.softDeleteAccount(ctx, publicID, "buyer")
}

func (s *Service) softDeleteAccount(ctx context.Context, publicID, accountType string) error {
	if err := s.repo.SoftDeleteAccount(ctx, publicID, accountType); err != nil {
		if err == ErrNotFound {
			return httpx.NotFound("account not found")
		}
		return err
	}
	return nil
}
