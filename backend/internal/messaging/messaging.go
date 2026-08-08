// Package messaging implements in-app threads, messages, the marketplace
// connect gate, broadcasts, audit mode, internal team chat, and real-time
// delivery over WebSocket.
package messaging

import (
	"context"
	"errors"
	"time"

	"github.com/echayko/leadrula/backend/internal/accounts"
	"github.com/echayko/leadrula/backend/internal/auth"
	"github.com/echayko/leadrula/backend/internal/notifications"
	"github.com/echayko/leadrula/backend/internal/storage"
	"github.com/echayko/leadrula/backend/pkg/httpx"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// editWindow is how long after sending a message can be edited or deleted.
const editWindow = 60 * time.Second

// memberScope matches a thread_members row against a principal: internal rows
// match by user_id, external rows match by account_id. Bind $1=userID $2=accountID.
const memberScope = `(tm.user_id = $1 OR (tm.user_id IS NULL AND tm.account_id = $2))`

// Thread is a conversation summary returned to clients.
type Thread struct {
	ID              string     `json:"id"` // public_id
	Type            string     `json:"type"`
	Context         string     `json:"context"`
	Status          string     `json:"status"`
	Title           *string    `json:"title,omitempty"`
	LeadID          *string    `json:"lead_id,omitempty"`
	ContractID      *string    `json:"contract_id,omitempty"`
	ContextLabel    string     `json:"context_label,omitempty"`
	DisplayName     string     `json:"display_name"`
	LastMessageAt   *time.Time `json:"last_message_at,omitempty"`
	LastMessage     *Message   `json:"last_message,omitempty"`
	UnreadCount     int        `json:"unread_count"`
	Muted           bool       `json:"muted"`
	CanSend         bool       `json:"can_send"`
	BlockedByMe     bool       `json:"blocked_by_me"`
	Members         []Member   `json:"members,omitempty"`
	CreatedAt       time.Time  `json:"created_at"`
}

// Member is a thread participant.
type Member struct {
	AccountID   int64      `json:"-"`
	UserID      *int64     `json:"-"`
	Name        string     `json:"name"`
	Role        string     `json:"role"`
	InviteStatus string    `json:"invite_status"`
	Muted       bool       `json:"muted"`
	LastReadAt  *time.Time `json:"last_read_at,omitempty"`
}

// Message is a single message returned to clients.
type Message struct {
	ID          string       `json:"id"` // public_id
	ThreadID    string       `json:"thread_id"`
	SenderName  string       `json:"sender_name"`
	Mine        bool         `json:"mine"`
	Body        *string      `json:"body,omitempty"`
	Type        string       `json:"type"`
	LeadID      *string      `json:"lead_id,omitempty"`
	LeadCard    *LeadCard    `json:"lead_card,omitempty"`
	ReplyTo     *ReplyRef    `json:"reply_to,omitempty"`
	Attachments []Attachment `json:"attachments,omitempty"`
	EditedAt    *time.Time   `json:"edited_at,omitempty"`
	DeletedAt   *time.Time   `json:"deleted_at,omitempty"`
	CanEdit     bool         `json:"can_edit"`
	CanDelete   bool         `json:"can_delete"`
	CreatedAt   time.Time    `json:"created_at"`
}

// ReplyRef is a compact preview of the message being replied to.
type ReplyRef struct {
	ID         string  `json:"id"`
	SenderName string  `json:"sender_name"`
	Body       *string `json:"body,omitempty"`
}

// LeadCard is the compact lead reference rendered for a shared lead.
type LeadCard struct {
	ID    string `json:"id"` // lead public_id
	Name  string `json:"name"`
	Phone string `json:"phone,omitempty"`
	City  string `json:"city,omitempty"`
	State string `json:"state,omitempty"`
}

// Attachment is a file attached to a message.
type Attachment struct {
	ID          string `json:"id"` // public_id
	Filename    string `json:"filename"`
	ContentType string `json:"content_type"`
	ByteSize    int64  `json:"byte_size"`
}

// Service holds messaging dependencies.
type Service struct {
	pool     *pgxpool.Pool
	hub      *Hub
	store    *storage.DisputeAttachmentStore
	notif    *notifications.Service
	accounts *accounts.Repository
}

// NewService builds the messaging service. The attachment store reuses the
// dispute attachment S3 config (authenticated GET, no public URLs).
func NewService(pool *pgxpool.Pool, hub *Hub, store *storage.DisputeAttachmentStore, notif *notifications.Service, accountsRepo *accounts.Repository) *Service {
	return &Service{pool: pool, hub: hub, store: store, notif: notif, accounts: accountsRepo}
}

func canModifyNow(createdAt time.Time) bool {
	return time.Since(createdAt) <= editWindow
}

// forMessaging resolves the home inbox identity. While switched or impersonating,
// messaging uses the origin account inbox, not the active session account.
func (s *Service) forMessaging(ctx context.Context, p *auth.Principal) (*auth.Principal, error) {
	return s.homePrincipal(ctx, p)
}

func (s *Service) homePrincipal(ctx context.Context, p *auth.Principal) (*auth.Principal, error) {
	if p == nil {
		return nil, httpx.Forbidden("unauthenticated")
	}
	if p.Impersonator != nil {
		hp := *p.Impersonator
		hp.UserID = p.UserID
		hp.UserPublicID = p.UserPublicID
		hp.Impersonator = nil
		hp.SwitchedFrom = ""
		hp.SwitchedFromPublisherID = 0
		return &hp, nil
	}
	if p.SwitchedFrom != "" {
		var originID int64
		var originType, originPub string
		if err := s.pool.QueryRow(ctx,
			`SELECT id, type::text, public_id::text FROM accounts WHERE public_id=$1 AND deleted_at IS NULL`,
			p.SwitchedFrom).Scan(&originID, &originType, &originPub); err != nil {
			return nil, httpx.Forbidden("origin account not found")
		}
		if originType == "platform" {
			return p, nil
		}
		var role string
		if err := s.pool.QueryRow(ctx,
			`SELECT role::text FROM users WHERE id=$1 AND account_id=$2 AND is_active`,
			p.UserID, originID).Scan(&role); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return nil, httpx.Forbidden("not a member of origin account")
			}
			return nil, err
		}
		hp := &auth.Principal{
			UserID:          p.UserID,
			UserPublicID:    p.UserPublicID,
			AccountID:       originID,
			AccountPublicID: originPub,
			AccountType:     originType,
			Role:            role,
		}
		if originType == "platform" || role == "admin" {
			hp.FullAccess = true
		}
		return hp, nil
	}
	return p, nil
}

// requiresConnectGate reports whether a first message between two external
// accounts must go through the connect request flow. Platform senders/recipients
// and any existing relationship (contract, accepted connect, direct-buyer
// partnership, or publisher partnership) bypass the gate.
func (s *Service) requiresConnectGate(ctx context.Context, actor *auth.Principal, recipientAccountID int64, recipientType string) (bool, error) {
	if actor.AccountType == "platform" || recipientType == "platform" {
		return false, nil
	}
	a, b := actor.AccountID, recipientAccountID

	var contractExists bool
	if err := s.pool.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM contracts
		   WHERE ((publisher_id=$1 AND buyer_id=$2) OR (publisher_id=$2 AND buyer_id=$1))
		     AND status='active' AND deleted_at IS NULL)`, a, b).Scan(&contractExists); err != nil {
		return false, err
	}
	if contractExists {
		return false, nil
	}

	var connectAccepted bool
	if err := s.pool.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM connect_requests
		   WHERE ((initiator_id=$1 AND recipient_id=$2) OR (initiator_id=$2 AND recipient_id=$1))
		     AND status='accepted')`, a, b).Scan(&connectAccepted); err != nil {
		return false, err
	}
	if connectAccepted {
		return false, nil
	}

	// Direct buyer with an active partnership bypasses the gate.
	var directPartner bool
	if err := s.pool.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM partnerships p
		   JOIN accounts buyer ON buyer.id = p.buyer_id
		   WHERE ((p.publisher_id=$1 AND p.buyer_id=$2) OR (p.publisher_id=$2 AND p.buyer_id=$1))
		     AND p.status='active' AND buyer.buyer_kind='direct')`, a, b).Scan(&directPartner); err != nil {
		return false, err
	}
	if directPartner {
		return false, nil
	}

	// Publisher-to-publisher with an active publisher partnership bypasses.
	var pubPartner bool
	lo, hi := a, b
	if lo > hi {
		lo, hi = hi, lo
	}
	if err := s.pool.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM publisher_partnerships
		   WHERE publisher_a_id=$1 AND publisher_b_id=$2 AND status='active')`, lo, hi).Scan(&pubPartner); err != nil {
		return false, err
	}
	if pubPartner {
		return false, nil
	}

	return true, nil
}

// threadMembership loads the principal's membership row id and role for a thread.
// Returns ErrNoRows-style forbidden when the principal is not a member.
type membership struct {
	memberID int64
	role     string
	muted    bool
	invite   string
}

func (s *Service) loadMembership(ctx context.Context, p *auth.Principal, threadID int64) (*membership, error) {
	m := &membership{}
	err := s.pool.QueryRow(ctx,
		`SELECT tm.id, tm.role, tm.muted, tm.invite_status
		 FROM thread_members tm
		 WHERE tm.thread_id=$3 AND `+memberScope+` AND tm.left_at IS NULL`,
		p.UserID, p.AccountID, threadID).Scan(&m.memberID, &m.role, &m.muted, &m.invite)
	if err != nil {
		return nil, httpx.Forbidden("not a member of this thread")
	}
	return m, nil
}

// resolveThreadID converts a thread public_id to its internal id.
func (s *Service) resolveThreadID(ctx context.Context, publicID string) (int64, error) {
	var id int64
	if err := s.pool.QueryRow(ctx, `SELECT id FROM threads WHERE public_id=$1`, publicID).Scan(&id); err != nil {
		return 0, httpx.NotFound("thread not found")
	}
	return id, nil
}
