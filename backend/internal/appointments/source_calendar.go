package appointments

import "context"

// ContractCalendarConfigured reports whether the contract's active appointment calendar is ready.
func (s *Service) ContractCalendarConfigured(ctx context.Context, contractID int64) (bool, error) {
	return s.contractCalendarConfigured(ctx, contractID)
}
