package webhooks

import (
	"context"
	"encoding/json"
	"log"
	"time"
)

func (s *Service) enqueueCRMInboundStageSyncRetry(ctx context.Context, webhook Webhook, leadID int64, flat map[string]any) {
	if s == nil || s.pool == nil || leadID == 0 || webhook.IntegrationConnectionID == nil {
		return
	}
	payload, err := json.Marshal(flat)
	if err != nil {
		return
	}
	connID := *webhook.IntegrationConnectionID
	_, err = s.pool.Exec(ctx,
		`INSERT INTO crm_inbound_stage_sync_retries
		   (account_id, lead_id, connection_id, webhook_id, payload, next_attempt_at)
		 VALUES ($1, $2, $3, $4, $5, now() + interval '5 seconds')
		 ON CONFLICT (lead_id, connection_id) DO UPDATE SET
		   webhook_id = EXCLUDED.webhook_id,
		   payload = EXCLUDED.payload,
		   attempts = 0,
		   next_attempt_at = EXCLUDED.next_attempt_at,
		   updated_at = now()`,
		webhook.AccountID, leadID, connID, webhook.ID, payload)
	if err != nil {
		log.Printf("crm inbound stage sync retry enqueue lead %d conn %d: %v", leadID, connID, err)
	}
}

// RunCRMInboundStageSyncRetryWorker re-attempts inbound CRM stage sync blocked by pending outbound jobs.
func (s *Service) RunCRMInboundStageSyncRetryWorker(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.processCRMInboundStageSyncRetries(ctx)
		}
	}
}

func (s *Service) processCRMInboundStageSyncRetries(ctx context.Context) {
	if s == nil || s.pool == nil || s.leadSvc == nil {
		return
	}
	rows, err := s.pool.Query(ctx,
		`UPDATE crm_inbound_stage_sync_retries
		 SET attempts = attempts + 1, updated_at = now()
		 WHERE id IN (
		   SELECT id FROM crm_inbound_stage_sync_retries
		   WHERE attempts < max_attempts
		     AND next_attempt_at <= now()
		   ORDER BY next_attempt_at
		   LIMIT 10
		   FOR UPDATE SKIP LOCKED
		 )
		 RETURNING id, account_id, lead_id, connection_id, webhook_id, payload, attempts, max_attempts`)
	if err != nil {
		log.Printf("crm inbound stage sync retry: %v", err)
		return
	}
	defer rows.Close()

	type retryRow struct {
		id, accountID, leadID, connID, webhookID int64
		payload                                  []byte
		attempts, maxAttempts                    int
	}
	var batch []retryRow
	for rows.Next() {
		var row retryRow
		if err := rows.Scan(&row.id, &row.accountID, &row.leadID, &row.connID, &row.webhookID, &row.payload, &row.attempts, &row.maxAttempts); err != nil {
			continue
		}
		batch = append(batch, row)
	}
	if err := rows.Err(); err != nil {
		log.Printf("crm inbound stage sync retry scan: %v", err)
		return
	}

	for _, row := range batch {
		s.runCRMInboundStageSyncRetry(ctx, row.id, row.accountID, row.leadID, row.connID, row.webhookID, row.payload, row.attempts, row.maxAttempts)
	}
}

func (s *Service) runCRMInboundStageSyncRetry(ctx context.Context, retryID, accountID, leadID, connID, webhookID int64, payload []byte, attempts, maxAttempts int) {
	var pendingOutbound bool
	if err := s.pool.QueryRow(ctx,
		`SELECT EXISTS(
		   SELECT 1 FROM integration_delivery_queue
		   WHERE lead_id = $1 AND connection_id = $2
		     AND status IN ('pending', 'processing')
		 )`, leadID, connID).Scan(&pendingOutbound); err != nil {
		s.scheduleCRMInboundStageSyncRetry(ctx, retryID, attempts, maxAttempts, "delivery check failed")
		return
	}
	if pendingOutbound {
		s.scheduleCRMInboundStageSyncRetry(ctx, retryID, attempts, maxAttempts, "outbound delivery pending for lead")
		return
	}

	var flat map[string]any
	if err := json.Unmarshal(payload, &flat); err != nil {
		s.deleteCRMInboundStageSyncRetry(ctx, retryID)
		return
	}

	var webhookName string
	if err := s.pool.QueryRow(ctx, `SELECT name FROM webhooks WHERE id=$1`, webhookID).Scan(&webhookName); err != nil {
		s.scheduleCRMInboundStageSyncRetry(ctx, retryID, attempts, maxAttempts, "webhook not found")
		return
	}

	connIDCopy := connID
	webhook := Webhook{
		ID:                      webhookID,
		AccountID:               accountID,
		IntegrationConnectionID: &connIDCopy,
		Name:                    webhookName,
	}
	tryApplyGHLInboundStageSync(ctx, s, webhook, leadID, flat)
	s.deleteCRMInboundStageSyncRetry(ctx, retryID)
}

func (s *Service) scheduleCRMInboundStageSyncRetry(ctx context.Context, retryID int64, attempts, maxAttempts int, reason string) {
	if attempts >= maxAttempts {
		_, _ = s.pool.Exec(ctx, `DELETE FROM crm_inbound_stage_sync_retries WHERE id=$1`, retryID)
		log.Printf("crm inbound stage sync retry %d exhausted after %d attempts (%s)", retryID, attempts, reason)
		return
	}
	_, _ = s.pool.Exec(ctx,
		`UPDATE crm_inbound_stage_sync_retries
		 SET next_attempt_at = now() + interval '5 seconds', updated_at = now()
		 WHERE id = $1`, retryID)
}

func (s *Service) deleteCRMInboundStageSyncRetry(ctx context.Context, retryID int64) {
	_, _ = s.pool.Exec(ctx, `DELETE FROM crm_inbound_stage_sync_retries WHERE id=$1`, retryID)
}
