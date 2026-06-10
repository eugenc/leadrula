package notifications

import (
	"context"
	"encoding/json"
	"time"

	"github.com/echayko/leadrula/backend/internal/accounts"
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
	pool     *pgxpool.Pool
	accounts *accounts.Repository
	email    *EmailSender
	baseURL  string
}

func NewService(pool *pgxpool.Pool, accounts *accounts.Repository, email *EmailSender, baseURL string) *Service {
	return &Service{pool: pool, accounts: accounts, email: email, baseURL: baseURL}
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
