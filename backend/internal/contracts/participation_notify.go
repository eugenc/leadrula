package contracts

import (
	"context"

	"github.com/echayko/leadrula/backend/internal/database"
	"github.com/echayko/leadrula/backend/internal/notifications"
)

func (s *Service) notifyParticipation(ctx context.Context, q database.Querier, accountID int64, eventType string, payload map[string]any) {
	if s.notif == nil || s.accounts == nil {
		return
	}
	adminIDs, err := s.accounts.AdminUserIDs(ctx, q, accountID)
	if err != nil || len(adminIDs) == 0 {
		return
	}
	emails, err := s.notif.Deliver(ctx, q, notifications.DeliverParams{
		AccountID: accountID,
		UserIDs:   adminIDs,
		EventType: eventType,
		Payload:   payload,
	})
	if err != nil {
		return
	}
	s.notif.SendEmails(emails)
}
