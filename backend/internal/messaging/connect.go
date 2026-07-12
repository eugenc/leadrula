package messaging

import (
	"context"
	"time"

	"github.com/echayko/leadrula/backend/internal/auth"
	"github.com/echayko/leadrula/backend/pkg/httpx"
)

// ConnectRequestView is a pending/sent connect request row for the UI.
type ConnectRequestView struct {
	ID          string    `json:"id"` // connect_request public_id
	ThreadID    string    `json:"thread_id"`
	AccountName string    `json:"account_name"`
	HandlerID   string    `json:"handler_id"`
	Preview     string    `json:"preview,omitempty"`
	Status      string    `json:"status"`
	CreatedAt   time.Time `json:"created_at"`
}

// ListIncomingConnects returns pending requests where the principal is the recipient.
func (s *Service) ListIncomingConnects(ctx context.Context, p *auth.Principal) ([]ConnectRequestView, error) {
	hp, err := s.forMessaging(ctx, p)
	if err != nil {
		return nil, err
	}
	p = hp
	rows, err := s.pool.Query(ctx, `
		SELECT cr.public_id::text, t.public_id::text, a.name, a.handler_id, COALESCE(cr.message_preview,''), cr.status, cr.created_at
		FROM connect_requests cr
		JOIN accounts a ON a.id=cr.initiator_id
		JOIN threads t ON t.id=cr.thread_id
		WHERE cr.recipient_id=$1 AND cr.status='pending'
		ORDER BY cr.created_at DESC`, p.AccountID)
	if err != nil {
		return nil, err
	}
	return scanConnectRequests(rows)
}

// ListSentConnects returns the principal's outgoing requests (pending/declined/blocked).
func (s *Service) ListSentConnects(ctx context.Context, p *auth.Principal) ([]ConnectRequestView, error) {
	hp, err := s.forMessaging(ctx, p)
	if err != nil {
		return nil, err
	}
	p = hp
	rows, err := s.pool.Query(ctx, `
		SELECT cr.public_id::text, t.public_id::text, a.name, a.handler_id, COALESCE(cr.message_preview,''), cr.status, cr.created_at
		FROM connect_requests cr
		JOIN accounts a ON a.id=cr.recipient_id
		JOIN threads t ON t.id=cr.thread_id
		WHERE cr.initiator_id=$1 AND cr.status IN ('pending','declined','blocked')
		ORDER BY cr.created_at DESC`, p.AccountID)
	if err != nil {
		return nil, err
	}
	return scanConnectRequests(rows)
}

func scanConnectRequests(rows interface {
	Next() bool
	Scan(...any) error
	Err() error
	Close()
}) ([]ConnectRequestView, error) {
	defer rows.Close()
	var out []ConnectRequestView
	for rows.Next() {
		var v ConnectRequestView
		if err := rows.Scan(&v.ID, &v.ThreadID, &v.AccountName, &v.HandlerID, &v.Preview, &v.Status, &v.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

// AcceptConnect accepts a pending request (recipient only) and unlocks the pair.
func (s *Service) AcceptConnect(ctx context.Context, p *auth.Principal, requestPublicID string) (*Thread, error) {
	hp, err := s.forMessaging(ctx, p)
	if err != nil {
		return nil, err
	}
	p = hp
	var reqID, threadID int64
	var status string
	err = s.pool.QueryRow(ctx,
		`SELECT id, thread_id, status FROM connect_requests WHERE public_id=$1 AND recipient_id=$2`,
		requestPublicID, p.AccountID).Scan(&reqID, &threadID, &status)
	if err != nil {
		return nil, httpx.NotFound("connect request not found")
	}
	if status != "pending" {
		return nil, httpx.Validation("connect request already resolved")
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx,
		`UPDATE connect_requests SET status='accepted', responded_at=now() WHERE id=$1`, reqID); err != nil {
		return nil, err
	}
	if _, err := tx.Exec(ctx,
		`UPDATE threads SET status='active', connect_accepted_at=now(), connect_accepted_by=$2 WHERE id=$1`,
		threadID, p.AccountID); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	s.fanoutRaw(ctx, threadID, WSEvent{Type: "connect_accepted", ThreadID: threadPublicID(ctx, s, threadID)})
	return s.GetThreadByID(ctx, p, threadID)
}

// DeclineConnect soft-blocks a pending request (recipient only). The thread
// stays frozen and the initiator cannot re-contact.
func (s *Service) DeclineConnect(ctx context.Context, p *auth.Principal, requestPublicID string) error {
	hp, err := s.forMessaging(ctx, p)
	if err != nil {
		return err
	}
	p = hp
	tag, err := s.pool.Exec(ctx,
		`UPDATE connect_requests SET status='blocked', responded_at=now()
		 WHERE public_id=$1 AND recipient_id=$2 AND status='pending'`,
		requestPublicID, p.AccountID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return httpx.NotFound("connect request not found")
	}
	return nil
}

// ListGroupInvites returns threads where the principal has a pending invite.
func (s *Service) ListGroupInvites(ctx context.Context, p *auth.Principal) ([]Thread, error) {
	hp, err := s.forMessaging(ctx, p)
	if err != nil {
		return nil, err
	}
	p = hp
	rows, err := s.pool.Query(ctx, `
		SELECT t.id, t.public_id::text, t.type::text, t.context::text, t.status::text, t.title,
		       NULL::text, NULL::text, NULL::text, NULL::text, NULL::text,
		       t.last_message_at, t.created_at, t.blocked_by,
		       tm.muted, tm.invite_status, 0
		FROM threads t
		JOIN thread_members tm ON tm.thread_id=t.id AND `+memberScope+` AND tm.left_at IS NULL
		WHERE tm.invite_status='pending'
		ORDER BY t.created_at DESC`, p.UserID, p.AccountID)
	if err != nil {
		return nil, err
	}
	threads, ids, err := scanThreadRows(rows, p)
	if err != nil {
		return nil, err
	}
	if err := s.attachMembersAndLast(ctx, p, threads, ids); err != nil {
		return nil, err
	}
	return threads, nil
}

func threadPublicID(ctx context.Context, s *Service, threadID int64) string {
	var pub string
	_ = s.pool.QueryRow(ctx, `SELECT public_id::text FROM threads WHERE id=$1`, threadID).Scan(&pub)
	return pub
}
