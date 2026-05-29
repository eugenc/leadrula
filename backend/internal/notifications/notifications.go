package notifications

import (
	"context"
	"encoding/json"
	"time"

	"github.com/echayko/leadrula/backend/internal/database"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Notification struct {
	ID        int64           `json:"id"`
	Type      string          `json:"type"`
	Payload   json.RawMessage `json:"payload"`
	ReadAt    *time.Time      `json:"read_at"`
	CreatedAt time.Time       `json:"created_at"`
}

type Service struct {
	pool *pgxpool.Pool
}

func NewService(pool *pgxpool.Pool) *Service { return &Service{pool: pool} }

// Enqueue inserts an in-app notification for each user. Runs inside the caller's
// transaction when q is a tx, so it commits atomically with the triggering work.
func (s *Service) Enqueue(ctx context.Context, q database.Querier, userIDs []int64, ntype string, payload map[string]any) error {
	if len(userIDs) == 0 {
		return nil
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	for _, uid := range userIDs {
		if _, err := q.Exec(ctx,
			`INSERT INTO notifications(user_id, type, payload) VALUES ($1,$2,$3)`,
			uid, ntype, raw); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) List(ctx context.Context, userID int64) ([]Notification, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT id, type, payload, read_at, created_at FROM notifications
		 WHERE user_id = $1 ORDER BY created_at DESC LIMIT 100`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Notification
	for rows.Next() {
		var n Notification
		if err := rows.Scan(&n.ID, &n.Type, &n.Payload, &n.ReadAt, &n.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, n)
	}
	return out, rows.Err()
}

func (s *Service) MarkRead(ctx context.Context, userID, notifID int64) error {
	_, err := s.pool.Exec(ctx,
		`UPDATE notifications SET read_at = now() WHERE id = $1 AND user_id = $2 AND read_at IS NULL`,
		notifID, userID)
	return err
}
