package contracts

import (
	"context"
	"log"

	"github.com/echayko/leadrula/backend/internal/database"
	"github.com/echayko/leadrula/backend/internal/notifications"
)

func (s *Service) notifyParticipation(ctx context.Context, q database.Querier, accountID int64, eventType string, payload map[string]any) {
	if s.notif == nil || s.accounts == nil {
		log.Printf("contract notify skipped: notifier not configured event=%s account=%d", eventType, accountID)
		return
	}
	adminIDs, err := s.accounts.AdminUserIDs(ctx, q, accountID)
	if err != nil {
		log.Printf("contract notify admin lookup failed event=%s account=%d: %v", eventType, accountID, err)
		return
	}
	if len(adminIDs) == 0 {
		log.Printf("contract notify skipped: no admins event=%s account=%d", eventType, accountID)
		return
	}
	emails, err := s.notif.Deliver(ctx, q, notifications.DeliverParams{
		AccountID: accountID,
		UserIDs:   adminIDs,
		EventType: eventType,
		Payload:   payload,
	})
	if err != nil {
		log.Printf("contract notify deliver failed event=%s account=%d: %v", eventType, accountID, err)
		return
	}
	s.notif.SendEmails(emails)
}

func (s *Service) publisherName(ctx context.Context, publisherID int64) string {
	var name string
	_ = s.pool.QueryRow(ctx, `SELECT name FROM accounts WHERE id = $1`, publisherID).Scan(&name)
	return name
}
