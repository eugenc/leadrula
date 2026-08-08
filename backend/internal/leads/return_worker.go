package leads

import (
	"context"
	"log"
	"time"
)

func (s *Service) RunReturnWorker(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := s.processScheduledReturns(ctx); err != nil {
				log.Printf("return worker: %v", err)
			}
		}
	}
}

func (s *Service) processScheduledReturns(ctx context.Context) error {
	if err := s.reclaimStaleScheduledReturns(ctx); err != nil {
		log.Printf("return worker: reclaim stale processing: %v", err)
	}
	rows, err := s.repo.Pool().Query(ctx, `
		UPDATE scheduled_lead_returns
		SET status = 'processing', updated_at = now()
		WHERE id IN (
			SELECT id FROM scheduled_lead_returns
			WHERE status = 'pending' AND execute_at <= now()
			ORDER BY execute_at
			LIMIT 10
			FOR UPDATE SKIP LOCKED
		)
		RETURNING id`)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			continue
		}
		go s.runScheduledReturn(context.Background(), id)
	}
	return rows.Err()
}

func (s *Service) runScheduledReturn(ctx context.Context, scheduledID int64) {
	tx, err := s.repo.Pool().Begin(ctx)
	if err != nil {
		log.Printf("return worker: begin tx scheduled=%d: %v", scheduledID, err)
		return
	}
	defer tx.Rollback(ctx)
	out, err := ExecuteScheduledReturn(ctx, tx, ReturnDeps{Repo: s.repo, Notif: s.notif}, scheduledID)
	if err != nil {
		log.Printf("return worker: execute scheduled=%d: %v", scheduledID, err)
		_, _ = tx.Exec(ctx, `UPDATE scheduled_lead_returns SET status = 'pending', updated_at = now() WHERE id = $1 AND status = 'processing'`, scheduledID)
		_ = tx.Commit(ctx)
		return
	}
	if err := tx.Commit(ctx); err != nil {
		log.Printf("return worker: commit scheduled=%d: %v", scheduledID, err)
		return
	}
	if len(out.Emails) > 0 {
		s.notif.SendEmails(out.Emails)
	}
}

func (s *Service) reclaimStaleScheduledReturns(ctx context.Context) error {
	_, err := s.repo.Pool().Exec(ctx, `
		UPDATE scheduled_lead_returns
		SET status = 'pending', updated_at = now()
		WHERE status = 'processing'
		  AND updated_at < now() - interval '2 minutes'`)
	return err
}
