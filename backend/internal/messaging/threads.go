package messaging

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/echayko/leadrula/backend/internal/auth"
	"github.com/echayko/leadrula/backend/pkg/httpx"
	"github.com/jackc/pgx/v5"
)

// DirectRequest starts or opens a direct thread with another account.
type DirectRequest struct {
	RecipientAccountID string `json:"recipient_account_id"` // account public_id
	Context            string `json:"context"`              // general | lead | contract
	LeadID             string `json:"lead_id,omitempty"`    // lead public_id
	ContractID         string `json:"contract_id,omitempty"`
	Title              string `json:"title,omitempty"`
	Body               string `json:"body,omitempty"`
}

// GroupRequest creates a named group thread.
type GroupRequest struct {
	Title      string   `json:"title"`
	MemberIDs  []string `json:"member_ids"`  // account public_ids (external group)
	Internal   bool     `json:"internal"`    // true → same-account user group
	UserIDs    []string `json:"user_ids"`    // user public_ids (internal group)
	Body       string   `json:"body,omitempty"`
}

// InternalDirectRequest starts a 1-on-1 with a teammate.
type InternalDirectRequest struct {
	UserID string `json:"user_id"` // teammate user public_id
	Body   string `json:"body,omitempty"`
}

// CreateDirect opens (or reuses) a direct thread with another account.
func (s *Service) CreateDirect(ctx context.Context, p *auth.Principal, req DirectRequest) (*Thread, error) {
	hp, err := s.forMessaging(ctx, p)
	if err != nil {
		return nil, err
	}
	p = hp
	recip, recipType, err := s.accountByPublicID(ctx, req.RecipientAccountID)
	if err != nil {
		return nil, err
	}
	if p.Role == "follower" && recipType != "platform" {
		return nil, httpx.Forbidden("followers can only message teammates")
	}
	if recip == p.AccountID {
		return nil, httpx.Validation("cannot message your own account")
	}

	context := req.Context
	if context == "" {
		context = "general"
	}
	var leadID, contractID *int64
	var contextLabel string
	switch context {
	case "general":
	case "lead":
		id, label, err := s.resolveLeadContext(ctx, req.LeadID)
		if err != nil {
			return nil, err
		}
		leadID, contextLabel = &id, label
	case "contract":
		id, label, err := s.resolveContractContext(ctx, req.ContractID)
		if err != nil {
			return nil, err
		}
		contractID, contextLabel = &id, label
	default:
		return nil, httpx.Validation("invalid context")
	}
	_ = contextLabel

	if existing, err := s.findDirectThread(ctx, p.AccountID, recip, context, leadID, contractID); err != nil {
		return nil, err
	} else if existing != 0 {
		return s.GetThreadByID(ctx, p, existing)
	}

	gated, err := s.requiresConnectGate(ctx, p, recip, recipType)
	if err != nil {
		return nil, err
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	status := "active"
	if gated {
		status = "pending"
	}
	var title *string
	if strings.TrimSpace(req.Title) != "" {
		t := strings.TrimSpace(req.Title)
		title = &t
	}
	var threadID int64
	var connectAt *time.Time
	if status == "pending" {
		now := time.Now()
		connectAt = &now
	}
	err = tx.QueryRow(ctx,
		`INSERT INTO threads(type, context, status, title, lead_id, contract_id, initiator_id, connect_requested_at)
		 VALUES ('direct', $1::thread_context, $2::thread_status, $3, $4, $5, $6, $7)
		 RETURNING id`,
		context, status, title, leadID, contractID, p.AccountID, connectAt).Scan(&threadID)
	if err != nil {
		return nil, err
	}
	if err := insertExternalMember(ctx, tx, threadID, p.AccountID, "owner"); err != nil {
		return nil, err
	}
	if err := insertExternalMember(ctx, tx, threadID, recip, "member"); err != nil {
		return nil, err
	}

	if gated {
		preview := req.Body
		if len(preview) > 280 {
			preview = preview[:280]
		}
		if _, err := tx.Exec(ctx,
			`INSERT INTO connect_requests(initiator_id, recipient_id, thread_id, status, message_preview)
			 VALUES ($1,$2,$3,'pending',$4)
			 ON CONFLICT (initiator_id, recipient_id) DO UPDATE SET thread_id=$3, status='pending', message_preview=$4, created_at=now()`,
			p.AccountID, recip, threadID, preview); err != nil {
			return nil, err
		}
	}

	if strings.TrimSpace(req.Body) != "" {
		if _, err := s.insertMessage(ctx, tx, threadID, p, req.Body, "text", nil, nil); err != nil {
			return nil, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	th, err := s.GetThreadByID(ctx, p, threadID)
	if err != nil {
		return nil, err
	}
	if !gated {
		s.fanoutThreadCreated(ctx, threadID)
	}
	return th, nil
}

// OpenLeadThread opens (or creates) a lead-context thread between the caller
// and the lead's counterpart account.
func (s *Service) OpenLeadThread(ctx context.Context, p *auth.Principal, leadPublicID string) (*Thread, error) {
	hp, err := s.forMessaging(ctx, p)
	if err != nil {
		return nil, err
	}
	p = hp
	var ownerID, publisherID int64
	if err := s.pool.QueryRow(ctx,
		`SELECT owner_account_id, publisher_id FROM leads WHERE public_id=$1`, leadPublicID).
		Scan(&ownerID, &publisherID); err != nil {
		return nil, httpx.NotFound("lead not found")
	}
	counterpart, err := s.counterpartOf(p.AccountID, publisherID, ownerID)
	if err != nil {
		return nil, err
	}
	pub, err := s.accountPublicID(ctx, counterpart)
	if err != nil {
		return nil, err
	}
	return s.CreateDirect(ctx, p, DirectRequest{RecipientAccountID: pub, Context: "lead", LeadID: leadPublicID})
}

// OpenContractThread opens (or creates) a contract-context thread with the
// contract's counterpart account.
func (s *Service) OpenContractThread(ctx context.Context, p *auth.Principal, contractPublicID string) (*Thread, error) {
	hp, err := s.forMessaging(ctx, p)
	if err != nil {
		return nil, err
	}
	p = hp
	var publisherID, buyerID int64
	if err := s.pool.QueryRow(ctx,
		`SELECT publisher_id, buyer_id FROM contracts WHERE public_id=$1`, contractPublicID).
		Scan(&publisherID, &buyerID); err != nil {
		return nil, httpx.NotFound("contract not found")
	}
	counterpart, err := s.counterpartOf(p.AccountID, publisherID, buyerID)
	if err != nil {
		return nil, err
	}
	pub, err := s.accountPublicID(ctx, counterpart)
	if err != nil {
		return nil, err
	}
	return s.CreateDirect(ctx, p, DirectRequest{RecipientAccountID: pub, Context: "contract", ContractID: contractPublicID})
}

// OpenSupportThread opens (or creates) a general support thread with the
// platform account. All roles including followers may use this path.
func (s *Service) OpenSupportThread(ctx context.Context, p *auth.Principal) (*Thread, error) {
	hp, err := s.forMessaging(ctx, p)
	if err != nil {
		return nil, err
	}
	p = hp
	if p.AccountType == "platform" {
		return nil, httpx.Validation("use partner messaging from platform accounts")
	}
	var platformPub string
	if err := s.pool.QueryRow(ctx,
		`SELECT public_id::text FROM accounts WHERE type='platform' AND deleted_at IS NULL ORDER BY id LIMIT 1`).
		Scan(&platformPub); err != nil {
		return nil, httpx.NotFound("support is not available")
	}
	th, err := s.CreateDirect(ctx, p, DirectRequest{
		RecipientAccountID: platformPub,
		Context:            "general",
		Title:              "Support",
	})
	if err != nil {
		return nil, err
	}
	_, _ = s.pool.Exec(ctx,
		`UPDATE threads SET title='Support' WHERE public_id=$1 AND (title IS NULL OR title='')`, th.ID)
	threadID, err := s.resolveThreadID(ctx, th.ID)
	if err != nil {
		return th, nil
	}
	return s.GetThreadByID(ctx, p, threadID)
}

// counterpartOf returns whichever of a/b is not the caller's account.
func (s *Service) counterpartOf(caller, a, b int64) (int64, error) {
	switch caller {
	case a:
		return b, nil
	case b:
		return a, nil
	default:
		return 0, httpx.Forbidden("you are not a party to this record")
	}
}

func (s *Service) accountPublicID(ctx context.Context, accountID int64) (string, error) {
	var pub string
	if err := s.pool.QueryRow(ctx, `SELECT public_id::text FROM accounts WHERE id=$1`, accountID).Scan(&pub); err != nil {
		return "", httpx.NotFound("account not found")
	}
	return pub, nil
}

// CreateInternalDirect starts a 1-on-1 with a teammate on the same account.
func (s *Service) CreateInternalDirect(ctx context.Context, p *auth.Principal, req InternalDirectRequest) (*Thread, error) {
	hp, err := s.forMessaging(ctx, p)
	if err != nil {
		return nil, err
	}
	p = hp
	otherUserID, err := s.teammateUserID(ctx, p.AccountID, req.UserID)
	if err != nil {
		return nil, err
	}
	if otherUserID == p.UserID {
		return nil, httpx.Validation("cannot message yourself")
	}
	if existing, err := s.findInternalDirect(ctx, p.AccountID, p.UserID, otherUserID); err != nil {
		return nil, err
	} else if existing != 0 {
		return s.GetThreadByID(ctx, p, existing)
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	var threadID int64
	if err := tx.QueryRow(ctx,
		`INSERT INTO threads(type, context, status, account_id) VALUES ('internal','general','active',$1) RETURNING id`,
		p.AccountID).Scan(&threadID); err != nil {
		return nil, err
	}
	if err := insertInternalMember(ctx, tx, threadID, p.AccountID, p.UserID, "owner"); err != nil {
		return nil, err
	}
	if err := insertInternalMember(ctx, tx, threadID, p.AccountID, otherUserID, "member"); err != nil {
		return nil, err
	}
	if strings.TrimSpace(req.Body) != "" {
		if _, err := s.insertMessage(ctx, tx, threadID, p, req.Body, "text", nil, nil); err != nil {
			return nil, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	th, err := s.GetThreadByID(ctx, p, threadID)
	if err != nil {
		return nil, err
	}
	s.fanoutThreadCreated(ctx, threadID)
	return th, nil
}

// CreateGroup creates a named group thread (internal user group or external
// account group). Unconnected external members get pending invites.
func (s *Service) CreateGroup(ctx context.Context, p *auth.Principal, req GroupRequest) (*Thread, error) {
	hp, err := s.forMessaging(ctx, p)
	if err != nil {
		return nil, err
	}
	p = hp
	if strings.TrimSpace(req.Title) == "" {
		return nil, httpx.Validation("group title is required")
	}
	if req.Internal {
		return s.createInternalGroup(ctx, p, req)
	}
	if p.Role == "follower" {
		return nil, httpx.Forbidden("followers can only create team groups")
	}
	if len(req.MemberIDs) == 0 {
		return nil, httpx.Validation("select at least one member")
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	var threadID int64
	if err := tx.QueryRow(ctx,
		`INSERT INTO threads(type, context, status, title) VALUES ('group','general','active',$1) RETURNING id`,
		req.Title).Scan(&threadID); err != nil {
		return nil, err
	}
	if err := insertExternalMember(ctx, tx, threadID, p.AccountID, "owner"); err != nil {
		return nil, err
	}
	for _, pubID := range req.MemberIDs {
		acc, accType, err := s.accountByPublicID(ctx, pubID)
		if err != nil {
			return nil, err
		}
		if acc == p.AccountID {
			continue
		}
		gated, err := s.requiresConnectGate(ctx, p, acc, accType)
		if err != nil {
			return nil, err
		}
		invite := "active"
		if gated {
			invite = "pending"
		}
		if _, err := tx.Exec(ctx,
			`INSERT INTO thread_members(thread_id, account_id, role, invite_status)
			 VALUES ($1,$2,'member',$3) ON CONFLICT DO NOTHING`,
			threadID, acc, invite); err != nil {
			return nil, err
		}
	}
	if strings.TrimSpace(req.Body) != "" {
		if _, err := s.insertMessage(ctx, tx, threadID, p, req.Body, "text", nil, nil); err != nil {
			return nil, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	th, err := s.GetThreadByID(ctx, p, threadID)
	if err != nil {
		return nil, err
	}
	s.fanoutThreadCreated(ctx, threadID)
	return th, nil
}

func (s *Service) createInternalGroup(ctx context.Context, p *auth.Principal, req GroupRequest) (*Thread, error) {
	if len(req.UserIDs) == 0 {
		return nil, httpx.Validation("select at least one teammate")
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	var threadID int64
	if err := tx.QueryRow(ctx,
		`INSERT INTO threads(type, context, status, title, account_id) VALUES ('group','general','active',$1,$2) RETURNING id`,
		req.Title, p.AccountID).Scan(&threadID); err != nil {
		return nil, err
	}
	if err := insertInternalMember(ctx, tx, threadID, p.AccountID, p.UserID, "owner"); err != nil {
		return nil, err
	}
	for _, pubID := range req.UserIDs {
		uid, err := s.teammateUserID(ctx, p.AccountID, pubID)
		if err != nil {
			return nil, err
		}
		if uid == p.UserID {
			continue
		}
		if err := insertInternalMember(ctx, tx, threadID, p.AccountID, uid, "member"); err != nil {
			return nil, err
		}
	}
	if strings.TrimSpace(req.Body) != "" {
		if _, err := s.insertMessage(ctx, tx, threadID, p, req.Body, "text", nil, nil); err != nil {
			return nil, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	th, err := s.GetThreadByID(ctx, p, threadID)
	if err != nil {
		return nil, err
	}
	s.fanoutThreadCreated(ctx, threadID)
	return th, nil
}

func insertExternalMember(ctx context.Context, q pgx.Tx, threadID, accountID int64, role string) error {
	_, err := q.Exec(ctx,
		`INSERT INTO thread_members(thread_id, account_id, role) VALUES ($1,$2,$3) ON CONFLICT DO NOTHING`,
		threadID, accountID, role)
	return err
}

func insertInternalMember(ctx context.Context, q pgx.Tx, threadID, accountID, userID int64, role string) error {
	_, err := q.Exec(ctx,
		`INSERT INTO thread_members(thread_id, account_id, user_id, role) VALUES ($1,$2,$3,$4) ON CONFLICT DO NOTHING`,
		threadID, accountID, userID, role)
	return err
}

// ListThreads returns the principal's inbox (or archived) threads, optionally
// filtered by a search query.
func (s *Service) ListThreads(ctx context.Context, p *auth.Principal, archived bool, query string) ([]Thread, error) {
	hp, err := s.forMessaging(ctx, p)
	if err != nil {
		return nil, err
	}
	p = hp
	args := []any{p.UserID, p.AccountID}
	archiveClause := "tm.archived_at IS NULL"
	if archived {
		archiveClause = "tm.archived_at IS NOT NULL"
	}
	search := ""
	q := strings.TrimSpace(query)
	if len(q) >= 2 {
		args = append(args, "%"+q+"%")
		n := fmt.Sprintf("$%d", len(args))
		search = `AND (
			t.title ILIKE ` + n + `
			OR lead.first_name ILIKE ` + n + ` OR lead.last_name ILIKE ` + n + `
			OR ct.name ILIKE ` + n + `
			OR EXISTS(SELECT 1 FROM thread_members tmx JOIN accounts ax ON ax.id=tmx.account_id
			          LEFT JOIN users ux ON ux.id=tmx.user_id
			          WHERE tmx.thread_id=t.id AND (ax.name ILIKE ` + n + ` OR ax.handler_id ILIKE ` + n + ` OR ux.full_name ILIKE ` + n + `))
			OR EXISTS(SELECT 1 FROM messages mx WHERE mx.thread_id=t.id AND mx.deleted_at IS NULL AND mx.body ILIKE ` + n + `)
		)`
	}

	followerFilter := ""
	if p.Role == "follower" {
		followerFilter = "AND t.type = 'internal'"
	}

	sql := `
		SELECT t.id, t.public_id::text, t.type::text, t.context::text, t.status::text, t.title,
		       lead.public_id::text, ct.public_id::text, lead.first_name, lead.last_name, ct.name,
		       t.last_message_at, t.created_at, t.blocked_by,
		       tm.muted, tm.invite_status,
		       (SELECT count(*) FROM messages m WHERE m.thread_id=t.id AND m.deleted_at IS NULL
		          AND (CASE WHEN t.type='internal' THEN COALESCE(m.sender_user_id,0) <> $1 ELSE m.sender_id <> $2 END)
		          AND (tm.last_read_at IS NULL OR m.created_at > tm.last_read_at)) AS unread
		FROM threads t
		JOIN thread_members tm ON tm.thread_id=t.id AND ` + memberScope + ` AND tm.left_at IS NULL
		LEFT JOIN leads lead ON lead.id=t.lead_id
		LEFT JOIN contracts ct ON ct.id=t.contract_id
		WHERE t.status='active' AND ` + archiveClause + ` AND tm.invite_status='active' ` + followerFilter + ` ` + search + `
		ORDER BY t.last_message_at DESC NULLS LAST, t.created_at DESC
		LIMIT 200`

	rows, err := s.pool.Query(ctx, sql, args...)
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

// threadRow keys threads by internal id during assembly.
func scanThreadRows(rows pgx.Rows, p *auth.Principal) ([]Thread, []int64, error) {
	defer rows.Close()
	var out []Thread
	var ids []int64
	// parallel slice of internal ids
	type keyed struct {
		id int64
		t  *Thread
	}
	var keys []keyed
	for rows.Next() {
		var (
			id                              int64
			th                              Thread
			title, leadPub, ctPub           *string
			leadFirst, leadLast, ctName     *string
			blockedBy                       *int64
			invite                          string
		)
		if err := rows.Scan(&id, &th.ID, &th.Type, &th.Context, &th.Status, &title,
			&leadPub, &ctPub, &leadFirst, &leadLast, &ctName,
			&th.LastMessageAt, &th.CreatedAt, &blockedBy,
			&th.Muted, &invite, &th.UnreadCount); err != nil {
			return nil, nil, err
		}
		th.Title = title
		th.LeadID = leadPub
		th.ContractID = ctPub
		th.ContextLabel = contextLabel(th.Context, leadFirst, leadLast, ctName)
		th.BlockedByMe = blockedBy != nil && *blockedBy == p.AccountID
		th.CanSend = th.Status == "active" && invite == "active"
		out = append(out, th)
		keys = append(keys, keyed{id: id})
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return nil, nil, err
	}
	return out, ids, nil
}

func contextLabel(context string, leadFirst, leadLast, ctName *string) string {
	switch context {
	case "lead":
		name := strings.TrimSpace(deref(leadFirst) + " " + deref(leadLast))
		if name == "" {
			name = "lead"
		}
		return "Re: " + name
	case "contract":
		if ctName != nil && *ctName != "" {
			return "Re: " + *ctName
		}
		return "Re: contract"
	default:
		return ""
	}
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// attachMembersAndLast fills members, display name, and last message preview.
func (s *Service) attachMembersAndLast(ctx context.Context, p *auth.Principal, threads []Thread, ids []int64) error {
	if len(threads) == 0 {
		return nil
	}
	// Map public_id → *Thread for assembly.
	byPub := make(map[string]*Thread, len(threads))
	for i := range threads {
		byPub[threads[i].ID] = &threads[i]
	}

	memRows, err := s.pool.Query(ctx,
		`SELECT t.public_id::text, tm.account_id, tm.user_id, tm.role, tm.invite_status, tm.muted, tm.last_read_at,
		        COALESCE(u.full_name, a.name)
		 FROM thread_members tm
		 JOIN threads t ON t.id=tm.thread_id
		 JOIN accounts a ON a.id=tm.account_id
		 LEFT JOIN users u ON u.id=tm.user_id
		 WHERE tm.thread_id = ANY($1) AND tm.left_at IS NULL`, ids)
	if err != nil {
		return err
	}
	defer memRows.Close()
	for memRows.Next() {
		var pub string
		var m Member
		if err := memRows.Scan(&pub, &m.AccountID, &m.UserID, &m.Role, &m.InviteStatus, &m.Muted, &m.LastReadAt, &m.Name); err != nil {
			return err
		}
		if th := byPub[pub]; th != nil {
			th.Members = append(th.Members, m)
		}
	}
	if err := memRows.Err(); err != nil {
		return err
	}

	for i := range threads {
		th := &threads[i]
		if th.Title != nil && *th.Title != "" {
			th.DisplayName = *th.Title
		} else {
			th.DisplayName = counterpartName(th, p)
		}
	}

	// Last message preview per thread.
	lastRows, err := s.pool.Query(ctx,
		`SELECT DISTINCT ON (m.thread_id) t.public_id::text, m.public_id::text, m.type, m.body, m.deleted_at,
		        m.created_at, COALESCE(u.full_name, a.name)
		 FROM messages m
		 JOIN threads t ON t.id=m.thread_id
		 JOIN accounts a ON a.id=m.sender_id
		 LEFT JOIN users u ON u.id=m.sender_user_id
		 WHERE m.thread_id = ANY($1)
		 ORDER BY m.thread_id, m.created_at DESC`, ids)
	if err != nil {
		return err
	}
	defer lastRows.Close()
	for lastRows.Next() {
		var pub string
		var lm Message
		var deletedAt *time.Time
		if err := lastRows.Scan(&pub, &lm.ID, &lm.Type, &lm.Body, &deletedAt, &lm.CreatedAt, &lm.SenderName); err != nil {
			return err
		}
		lm.ThreadID = pub
		lm.DeletedAt = deletedAt
		if th := byPub[pub]; th != nil {
			th.LastMessage = &lm
		}
	}
	return lastRows.Err()
}

func counterpartName(th *Thread, p *auth.Principal) string {
	var names []string
	for _, m := range th.Members {
		if isMe(m, p) {
			continue
		}
		names = append(names, m.Name)
	}
	if len(names) == 0 {
		return "Conversation"
	}
	if len(names) <= 2 {
		return strings.Join(names, ", ")
	}
	return fmt.Sprintf("%s, %s +%d", names[0], names[1], len(names)-2)
}

func isMe(m Member, p *auth.Principal) bool {
	if m.UserID != nil {
		return *m.UserID == p.UserID
	}
	return m.AccountID == p.AccountID
}

// GetThreadByID assembles a single thread for a principal.
func (s *Service) GetThreadByID(ctx context.Context, p *auth.Principal, threadID int64) (*Thread, error) {
	hp, err := s.forMessaging(ctx, p)
	if err != nil {
		return nil, err
	}
	p = hp
	rows, err := s.pool.Query(ctx, `
		SELECT t.id, t.public_id::text, t.type::text, t.context::text, t.status::text, t.title,
		       lead.public_id::text, ct.public_id::text, lead.first_name, lead.last_name, ct.name,
		       t.last_message_at, t.created_at, t.blocked_by,
		       tm.muted, tm.invite_status,
		       (SELECT count(*) FROM messages m WHERE m.thread_id=t.id AND m.deleted_at IS NULL
		          AND (CASE WHEN t.type='internal' THEN COALESCE(m.sender_user_id,0) <> $1 ELSE m.sender_id <> $2 END)
		          AND (tm.last_read_at IS NULL OR m.created_at > tm.last_read_at)) AS unread
		FROM threads t
		JOIN thread_members tm ON tm.thread_id=t.id AND `+memberScope+` AND tm.left_at IS NULL
		LEFT JOIN leads lead ON lead.id=t.lead_id
		LEFT JOIN contracts ct ON ct.id=t.contract_id
		WHERE t.id=$3`, p.UserID, p.AccountID, threadID)
	if err != nil {
		return nil, err
	}
	threads, ids, err := scanThreadRows(rows, p)
	if err != nil {
		return nil, err
	}
	if len(threads) == 0 {
		return nil, httpx.Forbidden("not a member of this thread")
	}
	if err := s.attachMembersAndLast(ctx, p, threads, ids); err != nil {
		return nil, err
	}
	return &threads[0], nil
}

// GetThread resolves a public id and returns the thread for a principal.
func (s *Service) GetThread(ctx context.Context, p *auth.Principal, publicID string) (*Thread, error) {
	id, err := s.resolveThreadID(ctx, publicID)
	if err != nil {
		return nil, err
	}
	return s.GetThreadByID(ctx, p, id)
}

// SetMuted toggles per-thread mute for the principal's membership.
func (s *Service) SetMuted(ctx context.Context, p *auth.Principal, publicID string, muted bool) error {
	hp, err := s.forMessaging(ctx, p)
	if err != nil {
		return err
	}
	p = hp
	id, err := s.resolveThreadID(ctx, publicID)
	if err != nil {
		return err
	}
	if _, err := s.loadMembership(ctx, p, id); err != nil {
		return err
	}
	_, err = s.pool.Exec(ctx,
		`UPDATE thread_members tm SET muted=$4 WHERE tm.thread_id=$3 AND `+memberScope,
		p.UserID, p.AccountID, id, muted)
	return err
}

// SetArchived archives/unarchives a thread for the principal only.
func (s *Service) SetArchived(ctx context.Context, p *auth.Principal, publicID string, archived bool) error {
	hp, err := s.forMessaging(ctx, p)
	if err != nil {
		return err
	}
	p = hp
	id, err := s.resolveThreadID(ctx, publicID)
	if err != nil {
		return err
	}
	if _, err := s.loadMembership(ctx, p, id); err != nil {
		return err
	}
	val := "NULL"
	if archived {
		val = "now()"
	}
	_, err = s.pool.Exec(ctx,
		`UPDATE thread_members tm SET archived_at=`+val+` WHERE tm.thread_id=$3 AND `+memberScope,
		p.UserID, p.AccountID, id)
	return err
}

// BlockThread sets a thread read-only. UnblockThread restores it (blocker only).
func (s *Service) BlockThread(ctx context.Context, p *auth.Principal, publicID string) error {
	hp, err := s.forMessaging(ctx, p)
	if err != nil {
		return err
	}
	p = hp
	id, err := s.resolveThreadID(ctx, publicID)
	if err != nil {
		return err
	}
	if _, err := s.loadMembership(ctx, p, id); err != nil {
		return err
	}
	_, err = s.pool.Exec(ctx,
		`UPDATE threads SET status='blocked', blocked_by=$2 WHERE id=$1 AND status='active'`,
		id, p.AccountID)
	if err == nil {
		s.fanoutThreadUpdated(ctx, id)
	}
	return err
}

func (s *Service) UnblockThread(ctx context.Context, p *auth.Principal, publicID string) error {
	hp, err := s.forMessaging(ctx, p)
	if err != nil {
		return err
	}
	p = hp
	id, err := s.resolveThreadID(ctx, publicID)
	if err != nil {
		return err
	}
	tag, err := s.pool.Exec(ctx,
		`UPDATE threads SET status='active', blocked_by=NULL WHERE id=$1 AND status='blocked' AND blocked_by=$2`,
		id, p.AccountID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return httpx.Forbidden("only the account that blocked this thread can unblock it")
	}
	s.fanoutThreadUpdated(ctx, id)
	return nil
}

// AcceptGroupInvite / DeclineGroupInvite handle pending group membership.
func (s *Service) AcceptGroupInvite(ctx context.Context, p *auth.Principal, publicID string) error {
	return s.setInviteStatus(ctx, p, publicID, "active")
}

func (s *Service) DeclineGroupInvite(ctx context.Context, p *auth.Principal, publicID string) error {
	return s.setInviteStatus(ctx, p, publicID, "declined")
}

func (s *Service) setInviteStatus(ctx context.Context, p *auth.Principal, publicID, status string) error {
	hp, err := s.forMessaging(ctx, p)
	if err != nil {
		return err
	}
	p = hp
	id, err := s.resolveThreadID(ctx, publicID)
	if err != nil {
		return err
	}
	tag, err := s.pool.Exec(ctx,
		`UPDATE thread_members tm SET invite_status=$4 WHERE tm.thread_id=$3 AND `+memberScope+` AND tm.invite_status='pending'`,
		p.UserID, p.AccountID, id, status)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return httpx.NotFound("no pending invite for this thread")
	}
	return nil
}

// ── lookup helpers ───────────────────────────────────────────

func (s *Service) accountByPublicID(ctx context.Context, publicID string) (int64, string, error) {
	var id int64
	var typ string
	err := s.pool.QueryRow(ctx,
		`SELECT id, type::text FROM accounts WHERE public_id=$1 AND deleted_at IS NULL`, publicID).Scan(&id, &typ)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, "", httpx.NotFound("account not found")
		}
		return 0, "", err
	}
	return id, typ, nil
}

func (s *Service) teammateUserID(ctx context.Context, accountID int64, userPublicID string) (int64, error) {
	var id int64
	err := s.pool.QueryRow(ctx,
		`SELECT id FROM users WHERE public_id=$1 AND account_id=$2 AND is_active`, userPublicID, accountID).Scan(&id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, httpx.NotFound("teammate not found")
		}
		return 0, err
	}
	return id, nil
}

func (s *Service) resolveLeadContext(ctx context.Context, leadPublicID string) (int64, string, error) {
	var id int64
	var first, last string
	err := s.pool.QueryRow(ctx,
		`SELECT id, first_name, last_name FROM leads WHERE public_id=$1`, leadPublicID).Scan(&id, &first, &last)
	if err != nil {
		return 0, "", httpx.NotFound("lead not found")
	}
	return id, "Re: " + strings.TrimSpace(first+" "+last), nil
}

func (s *Service) resolveContractContext(ctx context.Context, contractPublicID string) (int64, string, error) {
	var id int64
	var name string
	err := s.pool.QueryRow(ctx,
		`SELECT id, name FROM contracts WHERE public_id=$1`, contractPublicID).Scan(&id, &name)
	if err != nil {
		return 0, "", httpx.NotFound("contract not found")
	}
	return id, "Re: " + name, nil
}

func (s *Service) findDirectThread(ctx context.Context, a, b int64, context string, leadID, contractID *int64) (int64, error) {
	var id int64
	err := s.pool.QueryRow(ctx, `
		SELECT t.id FROM threads t
		WHERE t.type='direct' AND t.context=$3
		  AND (($4::bigint IS NULL AND t.lead_id IS NULL) OR t.lead_id=$4)
		  AND (($5::bigint IS NULL AND t.contract_id IS NULL) OR t.contract_id=$5)
		  AND EXISTS(SELECT 1 FROM thread_members m WHERE m.thread_id=t.id AND m.account_id=$1 AND m.user_id IS NULL)
		  AND EXISTS(SELECT 1 FROM thread_members m WHERE m.thread_id=t.id AND m.account_id=$2 AND m.user_id IS NULL)
		LIMIT 1`, a, b, context, leadID, contractID).Scan(&id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, nil
		}
		return 0, err
	}
	return id, nil
}

func (s *Service) findInternalDirect(ctx context.Context, accountID, userA, userB int64) (int64, error) {
	var id int64
	err := s.pool.QueryRow(ctx, `
		SELECT t.id FROM threads t
		WHERE t.type='internal' AND t.account_id=$1
		  AND EXISTS(SELECT 1 FROM thread_members m WHERE m.thread_id=t.id AND m.user_id=$2)
		  AND EXISTS(SELECT 1 FROM thread_members m WHERE m.thread_id=t.id AND m.user_id=$3)
		  AND (SELECT count(*) FROM thread_members m WHERE m.thread_id=t.id)=2
		LIMIT 1`, accountID, userA, userB).Scan(&id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, nil
		}
		return 0, err
	}
	return id, nil
}
