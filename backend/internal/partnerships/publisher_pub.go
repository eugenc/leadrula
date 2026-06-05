package partnerships

import (
	"context"
	"strings"

	"github.com/echayko/leadrula/backend/internal/accounts"
	"github.com/echayko/leadrula/backend/internal/auth"
	"github.com/echayko/leadrula/backend/pkg/httpx"
	"github.com/jackc/pgx/v5/pgxpool"
)

// LinkPublishers creates or activates a publisher↔publisher partnership.
func LinkPublishers(ctx context.Context, pool *pgxpool.Pool, pubA, pubB int64) error {
	if pubA == pubB {
		return httpx.Validation("cannot link publisher to itself")
	}
	a, b := pubA, pubB
	if a > b {
		a, b = b, a
	}
	_, err := pool.Exec(ctx,
		`INSERT INTO publisher_partnerships(publisher_a_id, publisher_b_id, status)
		 VALUES ($1,$2,'active')
		 ON CONFLICT (publisher_a_id, publisher_b_id) DO UPDATE SET status = 'active'`,
		a, b)
	return err
}

func (s *Service) RequestPublisherPartnership(ctx context.Context, p *auth.Principal, counterpartyHandlerID string) error {
	if !p.IsAdmin() {
		return httpx.Forbidden("admin required")
	}
	counterpartyHandlerID = strings.TrimSpace(strings.ToUpper(counterpartyHandlerID))
	if !strings.HasPrefix(counterpartyHandlerID, "P-") {
		return httpx.Validation("invalid publisher handler id")
	}
	other, err := s.accounts.GetAccountByHandlerID(ctx, counterpartyHandlerID)
	if err != nil {
		if err == accounts.ErrNotFound {
			return httpx.NotFound("publisher not found")
		}
		return err
	}
	if other.Type != "publisher" {
		return httpx.NotFound("publisher not found")
	}
	return LinkPublishers(ctx, s.repo.Pool(), p.AccountID, other.ID)
}
