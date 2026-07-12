package messaging

import (
	"context"
	"log"
	"sort"
	"strings"

	"github.com/echayko/leadrula/backend/internal/auth"
	"github.com/echayko/leadrula/backend/internal/partnerships"
	"github.com/echayko/leadrula/backend/pkg/httpx"
)

// BroadcastJob is a broadcast summary returned to the sender.
type BroadcastJob struct {
	ID          string `json:"id"`
	Status      string `json:"status"`
	TotalCount  int    `json:"total_count"`
	SentCount   int    `json:"sent_count"`
	FailedCount int    `json:"failed_count"`
}

// BroadcastRecipient is a selectable broadcast target for the publisher UI.
type BroadcastRecipient struct {
	ID        string `json:"id"` // account public_id
	Name      string `json:"name"`
	HandlerID string `json:"handler_id"`
	Type      string `json:"type"` // buyer | publisher
}

// CreateBroadcast queues a broadcast to selected buyers and partner publishers.
// Publishers only. Each recipient receives their own direct thread.
func (s *Service) CreateBroadcast(ctx context.Context, p *auth.Principal, body string, recipientPublicIDs []string) (*BroadcastJob, error) {
	hp, err := s.forMessaging(ctx, p)
	if err != nil {
		return nil, err
	}
	p = hp
	if p.AccountType != "publisher" {
		return nil, httpx.Forbidden("only publishers can broadcast")
	}
	if p.Role == "follower" {
		return nil, httpx.Forbidden("insufficient permissions")
	}
	if strings.TrimSpace(body) == "" {
		return nil, httpx.Validation("broadcast body is required")
	}
	if len(recipientPublicIDs) == 0 {
		return nil, httpx.Validation("at least one recipient is required")
	}

	eligible, err := s.broadcastEligibleByPublicID(ctx, p.AccountID)
	if err != nil {
		return nil, err
	}
	recipients := make([]int64, 0, len(recipientPublicIDs))
	seen := map[string]bool{}
	for _, pub := range recipientPublicIDs {
		pub = strings.TrimSpace(pub)
		if pub == "" || seen[pub] {
			continue
		}
		seen[pub] = true
		id, ok := eligible[pub]
		if !ok {
			return nil, httpx.Validation("invalid broadcast recipient")
		}
		recipients = append(recipients, id)
	}
	if len(recipients) == 0 {
		return nil, httpx.Validation("at least one recipient is required")
	}

	var jobID int64
	var jobPub string
	err = s.pool.QueryRow(ctx,
		`INSERT INTO broadcast_jobs(sender_id, sender_user_id, body, status, total_count)
		 VALUES ($1,$2,$3,'processing',$4) RETURNING id, public_id::text`,
		p.AccountID, p.UserID, body, len(recipients)).Scan(&jobID, &jobPub)
	if err != nil {
		return nil, err
	}

	go s.processBroadcast(context.Background(), jobID, p.AccountID, p.UserID, body, recipients)

	return &BroadcastJob{ID: jobPub, Status: "processing", TotalCount: len(recipients)}, nil
}

// ListBroadcastRecipients returns buyers (connected + contract) and partner publishers
// the publisher may broadcast to.
func (s *Service) ListBroadcastRecipients(ctx context.Context, p *auth.Principal) ([]BroadcastRecipient, error) {
	hp, err := s.forMessaging(ctx, p)
	if err != nil {
		return nil, err
	}
	p = hp
	if p.AccountType != "publisher" {
		return nil, httpx.Forbidden("only publishers can broadcast")
	}
	if p.Role == "follower" {
		return nil, httpx.Forbidden("insufficient permissions")
	}

	eligible, err := s.broadcastEligibleByPublicID(ctx, p.AccountID)
	if err != nil {
		return nil, err
	}
	if len(eligible) == 0 {
		return []BroadcastRecipient{}, nil
	}

	pubs := make([]string, 0, len(eligible))
	for pub := range eligible {
		pubs = append(pubs, pub)
	}
	sort.Strings(pubs)

	rows, err := s.pool.Query(ctx,
		`SELECT public_id::text, name, handler_id, type::text
		 FROM accounts WHERE public_id = ANY($1) AND deleted_at IS NULL
		 ORDER BY name`, pubs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]BroadcastRecipient, 0, len(pubs))
	for rows.Next() {
		var r BroadcastRecipient
		var typ string
		if err := rows.Scan(&r.ID, &r.Name, &r.HandlerID, &typ); err != nil {
			return nil, err
		}
		if typ == "buyer" {
			r.Type = "buyer"
		} else if typ == "publisher" {
			r.Type = "publisher"
		} else {
			continue
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// broadcastEligibleByPublicID maps eligible recipient public_id → internal account id.
func (s *Service) broadcastEligibleByPublicID(ctx context.Context, publisherID int64) (map[string]int64, error) {
	out := map[string]int64{}

	buyerIDs, err := s.broadcastBuyerRecipients(ctx, publisherID)
	if err != nil {
		return nil, err
	}
	if len(buyerIDs) > 0 {
		rows, err := s.pool.Query(ctx,
			`SELECT id, public_id::text FROM accounts WHERE id = ANY($1) AND deleted_at IS NULL`, buyerIDs)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		for rows.Next() {
			var id int64
			var pub string
			if err := rows.Scan(&id, &pub); err != nil {
				return nil, err
			}
			out[pub] = id
		}
		if err := rows.Err(); err != nil {
			return nil, err
		}
	}

	partners, err := partnerships.ListPartnerPublishers(ctx, s.pool, publisherID)
	if err != nil {
		return nil, err
	}
	for _, pp := range partners {
		if pp.PublicID != "" {
			out[pp.PublicID] = pp.ID
		}
	}
	return out, nil
}

func (s *Service) broadcastBuyerRecipients(ctx context.Context, publisherID int64) ([]int64, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT recipient_id FROM connect_requests WHERE initiator_id=$1 AND status='accepted'
		UNION
		SELECT initiator_id FROM connect_requests WHERE recipient_id=$1 AND status='accepted'
		UNION
		SELECT buyer_id FROM contracts WHERE publisher_id=$1 AND status='active' AND deleted_at IS NULL`,
		publisherID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return dedupeInts(out), rows.Err()
}

func (s *Service) processBroadcast(ctx context.Context, jobID, senderAccountID, senderUserID int64, body string, recipients []int64) {
	sender := &auth.Principal{AccountID: senderAccountID, UserID: senderUserID, AccountType: "publisher"}
	var sent, failed int
	for _, recip := range recipients {
		if err := s.deliverBroadcastTo(ctx, sender, recip, body); err != nil {
			failed++
			log.Printf("broadcast job=%d recipient=%d failed: %v", jobID, recip, err)
			continue
		}
		sent++
	}
	if _, err := s.pool.Exec(ctx,
		`UPDATE broadcast_jobs SET status='completed', sent_count=$2, failed_count=$3, completed_at=now() WHERE id=$1`,
		jobID, sent, failed); err != nil {
		log.Printf("broadcast job=%d finalize failed: %v", jobID, err)
	}
}

func (s *Service) deliverBroadcastTo(ctx context.Context, sender *auth.Principal, recipientID int64, body string) error {
	threadID, err := s.findDirectThread(ctx, sender.AccountID, recipientID, "general", nil, nil)
	if err != nil {
		return err
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if threadID == 0 {
		if err := tx.QueryRow(ctx,
			`INSERT INTO threads(type, context, status) VALUES ('direct','general','active') RETURNING id`).Scan(&threadID); err != nil {
			return err
		}
		if err := insertExternalMember(ctx, tx, threadID, sender.AccountID, "owner"); err != nil {
			return err
		}
		if err := insertExternalMember(ctx, tx, threadID, recipientID, "member"); err != nil {
			return err
		}
	}
	msgID, err := s.insertMessage(ctx, tx, threadID, sender, body, "text", nil, nil)
	if err != nil {
		return err
	}
	emails, err := s.notifyRecipients(ctx, tx, threadID, sender, body)
	if err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return err
	}
	if s.notif != nil {
		s.notif.SendEmails(emails)
	}
	if msg, err := s.getMessage(ctx, s.pool, msgID, sender); err == nil {
		s.fanoutMessage(ctx, threadID, "new_message", msg)
	}
	return nil
}
