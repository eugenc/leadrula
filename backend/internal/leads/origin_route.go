package leads

import (
	"context"

	"github.com/echayko/leadrula/backend/internal/routing"
)

// TryApplyWebhookOriginRoute applies a route whose origin is the given webhook.
func (s *Service) TryApplyWebhookOriginRoute(ctx context.Context, accountID, webhookID, leadID int64) {
	if s == nil || leadID == 0 || webhookID == 0 {
		return
	}
	rt, err := routing.MatchRouteByOriginWebhook(ctx, s.repo.pool, accountID, webhookID)
	if err != nil || rt == nil {
		return
	}
	s.applyMatchedOriginRoute(ctx, rt, leadID)
}

// TryApplyConnectionOriginRoute applies a route whose origin is the given integration connection.
func (s *Service) TryApplyConnectionOriginRoute(ctx context.Context, accountID, connectionID, leadID int64) {
	if s == nil || leadID == 0 || connectionID == 0 {
		return
	}
	rt, err := routing.MatchRouteByOriginConnection(ctx, s.repo.pool, accountID, connectionID)
	if err != nil || rt == nil {
		return
	}
	s.applyMatchedOriginRoute(ctx, rt, leadID)
}

func (s *Service) applyMatchedOriginRoute(ctx context.Context, rt *routing.Route, leadID int64) {
	tx, err := s.repo.pool.Begin(ctx)
	if err != nil {
		return
	}
	defer tx.Rollback(ctx)

	deps := RouteApplyDeps{Repo: s.repo, Accounts: s.accounts, Notif: s.notif, Integrations: s.integrations}
	enqueue, emails, err := TryApplyMatchedRoute(ctx, tx, deps, rt, leadID)
	if err != nil {
		return
	}
	if err := tx.Commit(ctx); err != nil {
		return
	}
	s.notif.SendEmails(emails)
	if enqueue {
		TryEnqueueIntegrations(ctx, s.repo.Pool(), s.repo, s.integrations, rt.ID, leadID)
	}
}
