package messaging

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/echayko/leadrula/backend/internal/auth"
	"github.com/echayko/leadrula/backend/internal/database"
	"github.com/echayko/leadrula/backend/internal/notifications"
	"github.com/echayko/leadrula/backend/pkg/httpx"
)

// SendRequest is a new message payload.
type SendRequest struct {
	Body      string `json:"body"`
	ReplyToID string `json:"reply_to_id,omitempty"` // message public_id
	LeadID    string `json:"lead_id,omitempty"`     // share a lead card
}

// UploadFile is one attachment supplied with a message.
type UploadFile struct {
	Filename    string
	ContentType string
	Size        int64
	Data        []byte
}

// SendMessage posts a message to a thread and fans it out in real time.
func (s *Service) SendMessage(ctx context.Context, p *auth.Principal, threadPublicID string, req SendRequest, files []UploadFile) (*Message, error) {
	hp, err := s.forMessaging(ctx, p)
	if err != nil {
		return nil, err
	}
	p = hp
	threadID, err := s.resolveThreadID(ctx, threadPublicID)
	if err != nil {
		return nil, err
	}
	mem, err := s.loadMembership(ctx, p, threadID)
	if err != nil {
		return nil, err
	}
	if mem.invite != "active" {
		return nil, httpx.Forbidden("accept the group invite before sending")
	}

	var status string
	var initiator *int64
	if err := s.pool.QueryRow(ctx, `SELECT status, initiator_id FROM threads WHERE id=$1`, threadID).Scan(&status, &initiator); err != nil {
		return nil, err
	}
	if status == "blocked" {
		return nil, httpx.Forbidden("this thread is blocked")
	}
	if status == "pending" {
		return nil, httpx.Forbidden("waiting for the connect request to be accepted")
	}

	msgType := "text"
	var leadID *int64
	if strings.TrimSpace(req.LeadID) != "" {
		id, err := s.validateShareableLead(ctx, p, threadID, req.LeadID)
		if err != nil {
			return nil, err
		}
		leadID, msgType = &id, "lead_card"
	} else if len(files) > 0 {
		msgType = "attachment"
	} else if strings.TrimSpace(req.Body) == "" {
		return nil, httpx.Validation("message body is required")
	}

	var replyTo *int64
	if strings.TrimSpace(req.ReplyToID) != "" {
		var rid int64
		if err := s.pool.QueryRow(ctx,
			`SELECT id FROM messages WHERE public_id=$1 AND thread_id=$2`, req.ReplyToID, threadID).Scan(&rid); err == nil {
			replyTo = &rid
		}
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	msgID, err := s.insertMessage(ctx, tx, threadID, p, req.Body, msgType, leadID, replyTo)
	if err != nil {
		return nil, err
	}
	if err := s.storeAttachments(ctx, tx, msgID, threadID, files); err != nil {
		return nil, err
	}
	emails, err := s.notifyRecipients(ctx, tx, threadID, p, req.Body)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	if s.notif != nil {
		s.notif.SendEmails(emails)
	}

	msg, err := s.getMessage(ctx, s.pool, msgID, p)
	if err != nil {
		return nil, err
	}
	s.fanoutMessage(ctx, threadID, "new_message", msg)
	return msg, nil
}

// insertMessage writes a message row inside a transaction.
func (s *Service) insertMessage(ctx context.Context, q database.Querier, threadID int64, p *auth.Principal, body, msgType string, leadID, replyTo *int64) (int64, error) {
	var bodyVal *string
	if strings.TrimSpace(body) != "" {
		bodyVal = &body
	}
	var senderUser *int64
	if p.UserID != 0 {
		senderUser = &p.UserID
	}
	var id int64
	err := q.QueryRow(ctx,
		`INSERT INTO messages(thread_id, sender_id, sender_user_id, body, type, lead_id, reply_to_id)
		 VALUES ($1,$2,$3,$4,$5,$6,$7) RETURNING id`,
		threadID, p.AccountID, senderUser, bodyVal, msgType, leadID, replyTo).Scan(&id)
	return id, err
}

// EditMessage updates a message body within the edit window.
func (s *Service) EditMessage(ctx context.Context, p *auth.Principal, messagePublicID, body string) (*Message, error) {
	hp, err := s.forMessaging(ctx, p)
	if err != nil {
		return nil, err
	}
	p = hp
	if strings.TrimSpace(body) == "" {
		return nil, httpx.Validation("message body is required")
	}
	var id, threadID int64
	var senderUser *int64
	var createdAt time.Time
	var deletedAt *time.Time
	err = s.pool.QueryRow(ctx,
		`SELECT id, thread_id, sender_user_id, created_at, deleted_at FROM messages WHERE public_id=$1`, messagePublicID).
		Scan(&id, &threadID, &senderUser, &createdAt, &deletedAt)
	if err != nil {
		return nil, httpx.NotFound("message not found")
	}
	if senderUser == nil || *senderUser != p.UserID {
		return nil, httpx.Forbidden("you can only edit your own messages")
	}
	if deletedAt != nil {
		return nil, httpx.Validation("message was deleted")
	}
	if !canModifyNow(createdAt) {
		return nil, httpx.Validation("the 60-second edit window has passed")
	}
	if _, err := s.pool.Exec(ctx,
		`UPDATE messages SET body=$2, body_edited=body, edited_at=now() WHERE id=$1`, id, body); err != nil {
		return nil, err
	}
	msg, err := s.getMessage(ctx, s.pool, id, p)
	if err != nil {
		return nil, err
	}
	s.fanoutMessage(ctx, threadID, "message_edited", msg)
	return msg, nil
}

// DeleteMessage soft-deletes a message within the edit window.
func (s *Service) DeleteMessage(ctx context.Context, p *auth.Principal, messagePublicID string) error {
	hp, err := s.forMessaging(ctx, p)
	if err != nil {
		return err
	}
	p = hp
	var id, threadID int64
	var threadPub string
	var senderUser *int64
	var createdAt time.Time
	err = s.pool.QueryRow(ctx,
		`SELECT m.id, m.thread_id, t.public_id, m.sender_user_id, m.created_at
		 FROM messages m JOIN threads t ON t.id=m.thread_id WHERE m.public_id=$1`, messagePublicID).
		Scan(&id, &threadID, &threadPub, &senderUser, &createdAt)
	if err != nil {
		return httpx.NotFound("message not found")
	}
	if senderUser == nil || *senderUser != p.UserID {
		return httpx.Forbidden("you can only delete your own messages")
	}
	if !canModifyNow(createdAt) {
		return httpx.Validation("the 60-second delete window has passed")
	}
	if _, err := s.pool.Exec(ctx, `UPDATE messages SET deleted_at=now(), body=NULL WHERE id=$1`, id); err != nil {
		return err
	}
	payload, _ := json.Marshal(map[string]any{"message_id": messagePublicID})
	s.fanoutRaw(ctx, threadID, WSEvent{Type: "message_deleted", ThreadID: threadPub, Payload: payload})
	return nil
}

// GetMessages returns thread history (oldest first) for a member.
func (s *Service) GetMessages(ctx context.Context, p *auth.Principal, threadPublicID string) ([]Message, error) {
	hp, err := s.forMessaging(ctx, p)
	if err != nil {
		return nil, err
	}
	p = hp
	threadID, err := s.resolveThreadID(ctx, threadPublicID)
	if err != nil {
		return nil, err
	}
	if _, err := s.loadMembership(ctx, p, threadID); err != nil {
		// Platform audit path handles non-members separately.
		if !s.hasAuditAccess(ctx, p, threadID) {
			return nil, err
		}
	}
	var status string
	var initiator *int64
	if err := s.pool.QueryRow(ctx, `SELECT status, initiator_id FROM threads WHERE id=$1`, threadID).Scan(&status, &initiator); err != nil {
		return nil, err
	}
	if status == "pending" && !s.hasAuditAccess(ctx, p, threadID) {
		if initiator == nil || *initiator != p.AccountID {
			return nil, httpx.Forbidden("connect request is still pending")
		}
	}
	return s.queryMessages(ctx, threadID, p)
}

func (s *Service) queryMessages(ctx context.Context, threadID int64, p *auth.Principal) ([]Message, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT m.id, m.public_id::text, t.public_id::text, m.type, m.body, m.deleted_at, m.edited_at, m.created_at,
		       m.sender_user_id, m.sender_id,
		       COALESCE(su.full_name, sa.name),
		       lead.public_id::text, lead.first_name, lead.last_name, lead.phone, lead.city, lead.state,
		       rm.public_id::text, COALESCE(ru.full_name, ra.name), rm.body
		FROM messages m
		JOIN threads t ON t.id=m.thread_id
		JOIN accounts sa ON sa.id=m.sender_id
		LEFT JOIN users su ON su.id=m.sender_user_id
		LEFT JOIN leads lead ON lead.id=m.lead_id
		LEFT JOIN messages rm ON rm.id=m.reply_to_id
		LEFT JOIN accounts ra ON ra.id=rm.sender_id
		LEFT JOIN users ru ON ru.id=rm.sender_user_id
		WHERE m.thread_id=$1
		ORDER BY m.created_at ASC
		LIMIT 500`, threadID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Message
	var ids []int64
	byID := map[int64]*Message{}
	for rows.Next() {
		var (
			id, senderID int64
			m            Message
			senderUser   *int64
			deletedAt    *time.Time
			leadPub      *string
			lf, ll, lph  *string
			lcity, lst   *string
			replyPub     *string
			replySender  *string
			replyBody    *string
		)
		if err := rows.Scan(&id, &m.ID, &m.ThreadID, &m.Type, &m.Body, &deletedAt, &m.EditedAt, &m.CreatedAt,
			&senderUser, &senderID, &m.SenderName,
			&leadPub, &lf, &ll, &lph, &lcity, &lst,
			&replyPub, &replySender, &replyBody); err != nil {
			return nil, err
		}
		m.DeletedAt = deletedAt
		m.Mine = (senderUser != nil && *senderUser == p.UserID)
		_ = senderID
		if leadPub != nil {
			m.LeadID = leadPub
			m.LeadCard = &LeadCard{
				ID:    *leadPub,
				Name:  strings.TrimSpace(deref(lf) + " " + deref(ll)),
				Phone: deref(lph), City: deref(lcity), State: deref(lst),
			}
		}
		if replyPub != nil {
			m.ReplyTo = &ReplyRef{ID: *replyPub, SenderName: deref(replySender), Body: replyBody}
		}
		if m.Mine && deletedAt == nil && canModifyNow(m.CreatedAt) {
			m.CanEdit = m.Type == "text"
			m.CanDelete = true
		}
		out = append(out, m)
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for i := range out {
		byID[ids[i]] = &out[i]
	}
	if err := s.attachAttachments(ctx, ids, byID); err != nil {
		return nil, err
	}
	return out, nil
}

func (s *Service) attachAttachments(ctx context.Context, messageIDs []int64, byID map[int64]*Message) error {
	if len(messageIDs) == 0 {
		return nil
	}
	rows, err := s.pool.Query(ctx,
		`SELECT message_id, public_id::text, filename, content_type, byte_size
		 FROM message_attachments WHERE message_id = ANY($1) ORDER BY id`, messageIDs)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var mid int64
		var a Attachment
		if err := rows.Scan(&mid, &a.ID, &a.Filename, &a.ContentType, &a.ByteSize); err != nil {
			return err
		}
		if m := byID[mid]; m != nil {
			m.Attachments = append(m.Attachments, a)
		}
	}
	return rows.Err()
}

// getMessage builds a single message for a viewer.
func (s *Service) getMessage(ctx context.Context, q database.Querier, messageID int64, p *auth.Principal) (*Message, error) {
	var threadID int64
	if err := q.QueryRow(ctx, `SELECT thread_id FROM messages WHERE id=$1`, messageID).Scan(&threadID); err != nil {
		return nil, err
	}
	msgs, err := s.queryMessages(ctx, threadID, p)
	if err != nil {
		return nil, err
	}
	var pub string
	if err := q.QueryRow(ctx, `SELECT public_id::text FROM messages WHERE id=$1`, messageID).Scan(&pub); err != nil {
		return nil, err
	}
	for i := range msgs {
		if msgs[i].ID == pub {
			return &msgs[i], nil
		}
	}
	return nil, httpx.NotFound("message not found")
}

// MarkRead advances the principal's read pointer to the latest message.
func (s *Service) MarkRead(ctx context.Context, p *auth.Principal, threadPublicID string) error {
	hp, err := s.forMessaging(ctx, p)
	if err != nil {
		return err
	}
	p = hp
	threadID, err := s.resolveThreadID(ctx, threadPublicID)
	if err != nil {
		return err
	}
	if _, err := s.loadMembership(ctx, p, threadID); err != nil {
		return err
	}
	_, err = s.pool.Exec(ctx, `
		UPDATE thread_members tm
		SET last_read_at=now(),
		    last_read_message_id=(SELECT id FROM messages WHERE thread_id=$3 ORDER BY created_at DESC LIMIT 1)
		WHERE tm.thread_id=$3 AND `+memberScope, p.UserID, p.AccountID, threadID)
	return err
}

// validateShareableLead ensures the lead exists and every external recipient
// account already has access to it (owns it or is publisher/participant).
func (s *Service) validateShareableLead(ctx context.Context, p *auth.Principal, threadID int64, leadPublicID string) (int64, error) {
	var leadID, ownerID, publisherID int64
	err := s.pool.QueryRow(ctx,
		`SELECT id, owner_account_id, publisher_id FROM leads WHERE public_id=$1`, leadPublicID).
		Scan(&leadID, &ownerID, &publisherID)
	if err != nil {
		return 0, httpx.NotFound("lead not found")
	}
	rows, err := s.pool.Query(ctx,
		`SELECT DISTINCT account_id FROM thread_members WHERE thread_id=$1 AND left_at IS NULL AND account_id <> $2`,
		threadID, p.AccountID)
	if err != nil {
		return 0, err
	}
	defer rows.Close()
	for rows.Next() {
		var acc int64
		if err := rows.Scan(&acc); err != nil {
			return 0, err
		}
		if acc != ownerID && acc != publisherID {
			return 0, httpx.Forbidden("a recipient does not have access to this lead")
		}
	}
	return leadID, rows.Err()
}

// ── fanout & notifications ───────────────────────────────────

// fanoutTargets returns the account IDs and user IDs to notify over WebSocket.
func (s *Service) fanoutTargets(ctx context.Context, threadID int64) (accountIDs, userIDs []int64) {
	rows, err := s.pool.Query(ctx,
		`SELECT account_id, user_id FROM thread_members WHERE thread_id=$1 AND left_at IS NULL AND invite_status='active'`,
		threadID)
	if err != nil {
		return nil, nil
	}
	defer rows.Close()
	for rows.Next() {
		var acc int64
		var uid *int64
		if err := rows.Scan(&acc, &uid); err != nil {
			return accountIDs, userIDs
		}
		if uid != nil {
			userIDs = append(userIDs, *uid)
		} else {
			accountIDs = append(accountIDs, acc)
		}
	}
	return accountIDs, userIDs
}

func (s *Service) fanoutMessage(ctx context.Context, threadID int64, eventType string, msg *Message) {
	if s.hub == nil || msg == nil {
		return
	}
	payload, err := json.Marshal(msg)
	if err != nil {
		return
	}
	s.fanoutRaw(ctx, threadID, WSEvent{Type: eventType, ThreadID: msg.ThreadID, Payload: payload})
}

func (s *Service) fanoutRaw(ctx context.Context, threadID int64, evt WSEvent) {
	if s.hub == nil {
		return
	}
	accountIDs, userIDs := s.fanoutTargets(ctx, threadID)
	s.hub.broadcastToAccounts(accountIDs, evt)
	s.hub.broadcastToUsers(userIDs, evt)
}

func (s *Service) fanoutThreadCreated(ctx context.Context, threadID int64) {
	var pub string
	if s.pool.QueryRow(ctx, `SELECT public_id::text FROM threads WHERE id=$1`, threadID).Scan(&pub) != nil {
		return
	}
	payload, _ := json.Marshal(map[string]any{"thread_id": pub})
	s.fanoutRaw(ctx, threadID, WSEvent{Type: "thread_created", ThreadID: pub, Payload: payload})
}

func (s *Service) fanoutThreadUpdated(ctx context.Context, threadID int64) {
	var pub string
	if s.pool.QueryRow(ctx, `SELECT public_id::text FROM threads WHERE id=$1`, threadID).Scan(&pub) != nil {
		return
	}
	payload, _ := json.Marshal(map[string]any{"thread_id": pub})
	s.fanoutRaw(ctx, threadID, WSEvent{Type: "thread_updated", ThreadID: pub, Payload: payload})
}

// notifyRecipients inserts message_received notifications for unmuted members
// other than the sender, returning email jobs to send after commit.
func (s *Service) notifyRecipients(ctx context.Context, q database.Querier, threadID int64, sender *auth.Principal, body string) ([]notifications.EmailJob, error) {
	if s.notif == nil {
		return nil, nil
	}
	rows, err := q.Query(ctx,
		`SELECT account_id, user_id FROM thread_members
		 WHERE thread_id=$1 AND left_at IS NULL AND invite_status='active' AND muted=false`, threadID)
	if err != nil {
		return nil, err
	}
	// accountID → set of user IDs to notify
	perAccount := map[int64][]int64{}
	var externalAccounts []int64
	func() {
		defer rows.Close()
		for rows.Next() {
			var acc int64
			var uid *int64
			if err := rows.Scan(&acc, &uid); err != nil {
				return
			}
			if uid != nil {
				if *uid != sender.UserID {
					perAccount[acc] = append(perAccount[acc], *uid)
				}
			} else if acc != sender.AccountID {
				externalAccounts = append(externalAccounts, acc)
			}
		}
	}()

	preview := strings.TrimSpace(body)
	if preview == "" {
		preview = "Shared a lead"
	}
	if len(preview) > 140 {
		preview = preview[:140]
	}
	var threadPub, senderName string
	_ = s.pool.QueryRow(ctx, `SELECT public_id::text FROM threads WHERE id=$1`, threadID).Scan(&threadPub)
	_ = s.pool.QueryRow(ctx, `SELECT COALESCE(full_name,'') FROM users WHERE id=$1`, sender.UserID).Scan(&senderName)
	payload := map[string]any{"thread_id": threadPub, "sender_name": senderName, "preview": preview}

	var emails []notifications.EmailJob
	// External account members: notify all active users of the account.
	for _, acc := range externalAccounts {
		uids, err := s.activeUserIDs(ctx, q, acc)
		if err != nil {
			return nil, err
		}
		perAccount[acc] = append(perAccount[acc], uids...)
	}
	for acc, uids := range perAccount {
		if len(uids) == 0 {
			continue
		}
		jobs, err := s.notif.Deliver(ctx, q, notifications.DeliverParams{
			AccountID: acc,
			UserIDs:   dedupeInts(uids),
			EventType: "message_received",
			Payload:   payload,
		})
		if err != nil {
			return nil, err
		}
		emails = append(emails, jobs...)
	}
	return emails, nil
}

func (s *Service) activeUserIDs(ctx context.Context, q database.Querier, accountID int64) ([]int64, error) {
	rows, err := q.Query(ctx, `SELECT id FROM users WHERE account_id=$1 AND is_active`, accountID)
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
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func (s *Service) storeAttachments(ctx context.Context, q database.Querier, msgID, threadID int64, files []UploadFile) error {
	if len(files) == 0 {
		return nil
	}
	if s.store == nil || !s.store.Enabled() {
		return httpx.BusinessRule("attachment storage is not configured")
	}
	for _, f := range files {
		suffix := make([]byte, 6)
		_, _ = rand.Read(suffix)
		key := fmt.Sprintf("messages/%d/%d/%s-%s", threadID, msgID, hex.EncodeToString(suffix), safeFilename(f.Filename))
		if err := s.store.Put(ctx, key, f.ContentType, bytes.NewReader(f.Data)); err != nil {
			return err
		}
		if _, err := q.Exec(ctx,
			`INSERT INTO message_attachments(message_id, storage_key, filename, content_type, byte_size)
			 VALUES ($1,$2,$3,$4,$5)`,
			msgID, key, f.Filename, f.ContentType, f.Size); err != nil {
			return err
		}
	}
	return nil
}

func safeFilename(name string) string {
	name = strings.TrimSpace(name)
	name = strings.ReplaceAll(name, "/", "_")
	name = strings.ReplaceAll(name, "\\", "_")
	name = strings.ReplaceAll(name, " ", "_")
	if name == "" {
		return "file"
	}
	if len(name) > 80 {
		name = name[len(name)-80:]
	}
	return name
}

// MessageAttachment streams an attachment blob for an access-controlled download.
func (s *Service) MessageAttachment(ctx context.Context, p *auth.Principal, attachmentPublicID string) (io.ReadCloser, string, string, error) {
	hp, err := s.forMessaging(ctx, p)
	if err != nil {
		return nil, "", "", err
	}
	p = hp
	var storageKey, filename, contentType string
	var threadID int64
	err = s.pool.QueryRow(ctx, `
		SELECT ma.storage_key, ma.filename, ma.content_type, m.thread_id
		FROM message_attachments ma
		JOIN messages m ON m.id=ma.message_id
		WHERE ma.public_id=$1`, attachmentPublicID).Scan(&storageKey, &filename, &contentType, &threadID)
	if err != nil {
		return nil, "", "", httpx.NotFound("attachment not found")
	}
	if _, err := s.loadMembership(ctx, p, threadID); err != nil {
		if !s.hasAuditAccess(ctx, p, threadID) {
			return nil, "", "", err
		}
	}
	if s.store == nil || !s.store.Enabled() {
		return nil, "", "", httpx.BusinessRule("attachment storage is not configured")
	}
	reader, ct, err := s.store.Get(ctx, storageKey)
	if err != nil {
		return nil, "", "", err
	}
	if ct == "" {
		ct = contentType
	}
	return reader, ct, filename, nil
}

// dedupeInts removes duplicate ids while preserving order.
func dedupeInts(in []int64) []int64 {
	seen := map[int64]struct{}{}
	var out []int64
	for _, v := range in {
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		out = append(out, v)
	}
	return out
}
