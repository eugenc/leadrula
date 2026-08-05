package integrations

import "context"

const inboundStageSyncInflightReason = "inbound stage sync in flight for lead"

func (s *Service) crmInboundStageSyncInflight(ctx context.Context, leadID, connID int64) (bool, error) {
	if s == nil || s.pool == nil || leadID == 0 || connID == 0 {
		return false, nil
	}
	var inflight bool
	err := s.pool.QueryRow(ctx,
		`SELECT EXISTS(
		   SELECT 1 FROM crm_inbound_stage_sync_retries
		   WHERE lead_id = $1 AND connection_id = $2
		 )`, leadID, connID).Scan(&inflight)
	return inflight, err
}

func (s *Service) deferJobForInboundStageSync(ctx context.Context, jobID int64) {
	if s == nil || s.pool == nil || jobID == 0 {
		return
	}
	_, _ = s.pool.Exec(ctx,
		`UPDATE integration_delivery_queue
		 SET status = 'pending', next_attempt_at = now() + interval '5 seconds',
		     attempts = GREATEST(attempts - 1, 0), updated_at = now()
		 WHERE id = $1`, jobID)
}
