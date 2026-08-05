package webhooks

import "context"

// acquireCRMInboundStageSyncInflight marks inbound stage sync in progress for lead+connection.
func (s *Service) acquireCRMInboundStageSyncInflight(ctx context.Context, webhook Webhook, leadID int64) bool {
	if s == nil || s.pool == nil || leadID == 0 || webhook.IntegrationConnectionID == nil {
		return false
	}
	connID := *webhook.IntegrationConnectionID
	_, err := s.pool.Exec(ctx,
		`INSERT INTO crm_inbound_stage_sync_retries
		   (account_id, lead_id, connection_id, webhook_id, payload, next_attempt_at)
		 VALUES ($1, $2, $3, $4, '{}', now())
		 ON CONFLICT (lead_id, connection_id) DO UPDATE SET updated_at = now()`,
		webhook.AccountID, leadID, connID, webhook.ID)
	return err == nil
}

func (s *Service) releaseCRMInboundStageSyncInflight(ctx context.Context, leadID, connID int64) {
	if s == nil || s.pool == nil || leadID == 0 || connID == 0 {
		return
	}
	_, _ = s.pool.Exec(ctx,
		`DELETE FROM crm_inbound_stage_sync_retries WHERE lead_id = $1 AND connection_id = $2`,
		leadID, connID)
}
