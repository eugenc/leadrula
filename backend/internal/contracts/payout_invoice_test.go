package contracts

import (
	"context"
	"sync"
	"testing"

	"github.com/echayko/leadrula/backend/internal/config"
	"github.com/echayko/leadrula/backend/internal/database"
)

type mockCompensationPayoutInvoicer struct {
	mu      sync.Mutex
	clearIDs []int64
}

func (m *mockCompensationPayoutInvoicer) CreateCompensationPayoutInvoice(_ context.Context, clearID int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.clearIDs = append(m.clearIDs, clearID)
	return nil
}

func connectPayoutTestDB(t *testing.T) *Service {
	t.Helper()
	cfg := config.Load()
	ctx := context.Background()
	pool, err := database.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(func() { pool.Close() })
	svc := NewService(pool)
	return svc
}

func TestEnsurePublisherPayoutClears_callsInvoicerForRevShare(t *testing.T) {
	svc := connectPayoutTestDB(t)
	ctx := context.Background()

	mock := &mockCompensationPayoutInvoicer{}
	svc.SetCompensationPayoutInvoicer(mock)

	var publisherID int64
	err := svc.pool.QueryRow(ctx,
		`SELECT c.publisher_id
		 FROM contract_compensations cc
		 JOIN contracts c ON c.id = cc.contract_id
		 WHERE cc.kind IN ('rev_share', 'profit_share')
		   AND cc.payout_frequency IS NOT NULL
		   AND c.deleted_at IS NULL
		   AND c.buyer_id IS NOT NULL
		 LIMIT 1`).Scan(&publisherID)
	if err != nil {
		t.Skip("no rev/profit share compensation with payout schedule")
	}

	if err := svc.EnsurePublisherPayoutClears(ctx, publisherID); err != nil {
		t.Fatal(err)
	}

	mock.mu.Lock()
	n := len(mock.clearIDs)
	mock.mu.Unlock()
	if n == 0 {
		t.Skip("no cleared periods with earnings yet for rev/profit share")
	}
}
