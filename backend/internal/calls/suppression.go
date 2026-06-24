package calls

import (
	"context"
	"fmt"

	"github.com/echayko/leadrula/backend/internal/database"
)

// isSuppressed reports whether this caller is within a billable-connect dedup
// window on the given source (tracking number).
func (s *Service) isSuppressed(ctx context.Context, q database.Querier, sourceID int64, callerHash string) (bool, error) {
	if callerHash == "" {
		return false, nil
	}
	var exists bool
	err := q.QueryRow(ctx,
		`SELECT EXISTS(
		   SELECT 1 FROM call_suppression
		   WHERE source_id=$1 AND caller_phone_hash=$2 AND expires_at > now())`,
		sourceID, callerHash).Scan(&exists)
	return exists, err
}

// addSuppression records a dedup entry after a billable connect.
func (s *Service) addSuppression(ctx context.Context, q database.Querier, sourceID int64, callerHash string, callID int64, windowHours int) error {
	if callerHash == "" || windowHours <= 0 {
		return nil
	}
	_, err := q.Exec(ctx,
		`INSERT INTO call_suppression(source_id, caller_phone_hash, call_id, expires_at)
		 VALUES ($1,$2,$3, now() + $4::interval)`,
		sourceID, callerHash, callID, fmt.Sprintf("%d hours", windowHours))
	return err
}

// markCallBlocked flags a duplicate/blocked call and tags the publisher lead.
func (s *Service) markCallBlocked(ctx context.Context, q database.Querier, callID, leadID int64) error {
	if _, err := q.Exec(ctx,
		`UPDATE calls SET status='blocked', disposition='wrong_number', ended_at=now() WHERE id=$1`, callID); err != nil {
		return err
	}
	return nil
}
