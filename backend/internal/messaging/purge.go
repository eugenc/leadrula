package messaging

import (
	"context"
	"log"
	"time"
)

// RunRetentionLoop runs the retention purge once a day until ctx is cancelled.
func (s *Service) RunRetentionLoop(ctx context.Context) {
	ticker := time.NewTicker(24 * time.Hour)
	defer ticker.Stop()
	// Run once shortly after boot.
	select {
	case <-time.After(time.Minute):
		s.runPurge(ctx)
	case <-ctx.Done():
		return
	}
	for {
		select {
		case <-ticker.C:
			s.runPurge(ctx)
		case <-ctx.Done():
			return
		}
	}
}

func (s *Service) runPurge(ctx context.Context) {
	// Remove attachment blobs for expired messages first.
	rows, err := s.pool.Query(ctx, `
		SELECT ma.storage_key FROM message_attachments ma
		JOIN messages m ON m.id=ma.message_id
		WHERE m.purge_after < now()`)
	if err != nil {
		log.Printf("retention purge: list attachments: %v", err)
	} else {
		var keys []string
		for rows.Next() {
			var k string
			if err := rows.Scan(&k); err == nil {
				keys = append(keys, k)
			}
		}
		rows.Close()
		if s.store != nil && s.store.Enabled() {
			for _, k := range keys {
				if err := s.store.Delete(ctx, k); err != nil {
					log.Printf("retention purge: delete blob %s: %v", k, err)
				}
			}
		}
	}

	// Delete expired message rows (cascades attachment rows).
	if tag, err := s.pool.Exec(ctx, `DELETE FROM messages WHERE purge_after < now()`); err != nil {
		log.Printf("retention purge: delete messages: %v", err)
	} else if tag.RowsAffected() > 0 {
		log.Printf("retention purge: removed %d expired messages", tag.RowsAffected())
	}

	// Delete threads whose retention window has fully elapsed.
	if _, err := s.pool.Exec(ctx, `DELETE FROM threads WHERE purge_after < now()`); err != nil {
		log.Printf("retention purge: delete threads: %v", err)
	}
}
