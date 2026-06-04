package accounts

import (
	"context"

	"github.com/echayko/leadrula/backend/pkg/httpx"
)

func (s *Service) SetOperationalStatus(ctx context.Context, publicID, accountType, status string) (*Account, error) {
	switch status {
	case AccountStatusActive, AccountStatusSuspended:
	default:
		return nil, httpx.Validation("operational_status must be active or suspended")
	}
	a, err := s.repo.SetOperationalStatus(ctx, publicID, accountType, status)
	if err != nil {
		if err == ErrNotFound {
			return nil, httpx.NotFound("account not found")
		}
		return nil, err
	}
	return a, nil
}
