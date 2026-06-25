package leads

import (
	"context"

	"github.com/echayko/leadrula/backend/internal/routing"
)

// TryApplyWebhookOriginRoute applies a route whose origin is the given webhook.
// Returns true when a matching route was found and applied.
func (s *Service) TryApplyWebhookOriginRoute(ctx context.Context, accountID, webhookID, leadID int64, payloadFlat map[string]any) bool {
	if s == nil || leadID == 0 || webhookID == 0 {
		return false
	}
	rt, err := routing.MatchRouteByOriginWebhook(ctx, s.repo.pool, accountID, webhookID, leadID, payloadFlat)
	if err != nil || rt == nil {
		return false
	}
	return s.applyMatchedOriginRoute(ctx, rt, leadID)
}

// TryApplyConnectionOriginRoute applies a route whose origin is the given integration connection.
// Returns true when a matching route was found and applied.
func (s *Service) TryApplyConnectionOriginRoute(ctx context.Context, accountID, connectionID, leadID int64, payloadFlat map[string]any) bool {
	if s == nil || leadID == 0 || connectionID == 0 {
		return false
	}
	rt, err := routing.MatchRouteByOriginConnection(ctx, s.repo.pool, accountID, connectionID, leadID, payloadFlat)
	if err != nil || rt == nil {
		return false
	}
	return s.applyMatchedOriginRoute(ctx, rt, leadID)
}

func (s *Service) applyMatchedOriginRoute(ctx context.Context, rt *routing.Route, leadID int64) bool {
	tx, err := s.repo.pool.Begin(ctx)
	if err != nil {
		return false
	}
	defer tx.Rollback(ctx)

	lead, err := s.repo.GetByID(ctx, tx, leadID)
	if err != nil {
		return false
	}
	if lead.Status == "returned" && lead.OwnerAccountID == lead.PublisherID {
		return false
	}

	deps := RouteApplyDeps{Repo: s.repo, Accounts: s.accounts, Notif: s.notif, Integrations: s.integrations}
	meta := originRouteMeta(rt)
	enqueue, emails, err := TryApplyMatchedRoute(ctx, tx, deps, rt, leadID, meta)
	if err != nil {
		return false
	}
	if err := tx.Commit(ctx); err != nil {
		return false
	}
	s.notif.SendEmails(emails)
	if enqueue {
		TryEnqueueIntegrations(ctx, s.repo.Pool(), s.repo, s.integrations, rt.ID, leadID, rt.MatchedBranchPosition, nil)
	}
	return true
}

func originRouteMeta(rt *routing.Route) RouteExecutionMeta {
	meta := RouteExecutionMeta{}
	switch rt.Origin {
	case "webhook":
		meta.TriggerType = "webhook"
		if rt.OriginWebhookName != nil {
			meta.TriggerLabel = *rt.OriginWebhookName
		}
	case "integration":
		meta.TriggerType = "integration"
		if rt.OriginConnectionName != nil {
			meta.TriggerLabel = *rt.OriginConnectionName
		}
	default:
		meta.TriggerType = rt.Origin
	}
	return meta
}
