package calls

import (
	"context"
	"log"
	"time"
)

// RunCapResetWorker zeroes stale daily/monthly call counters as a backstop to the
// lazy reset done at routing time. Mirrors billing.RunDisputeDeadlineWorker.
func (s *Service) RunCapResetWorker(ctx context.Context) {
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()
	for {
		if err := s.ResetAllStaleCaps(ctx); err != nil {
			log.Printf("calls cap reset: %v", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}
