package messaging

import (
	"context"

	"github.com/echayko/leadrula/backend/internal/auth"
	"github.com/echayko/leadrula/backend/pkg/httpx"
)

// hasAuditAccess reports whether a platform admin may read a thread because
// audit mode is enabled on it.
func (s *Service) hasAuditAccess(ctx context.Context, p *auth.Principal, threadID int64) bool {
	if p == nil || p.AccountType != "platform" {
		return false
	}
	var enabled bool
	if err := s.pool.QueryRow(ctx, `SELECT audit_mode FROM threads WHERE id=$1`, threadID).Scan(&enabled); err != nil {
		return false
	}
	return enabled
}

// EnableAuditMode flags every thread the account participates in as auditable
// so platform admins can read them for fraud investigation.
func (s *Service) EnableAuditMode(ctx context.Context, p *auth.Principal, accountPublicID string) (int, error) {
	if p.AccountType != "platform" {
		return 0, httpx.Forbidden("platform only")
	}
	accID, _, err := s.accountByPublicID(ctx, accountPublicID)
	if err != nil {
		return 0, err
	}
	tag, err := s.pool.Exec(ctx, `
		UPDATE threads SET audit_mode=true, audit_enabled_by=$2, audit_enabled_at=now()
		WHERE id IN (SELECT thread_id FROM thread_members WHERE account_id=$1)`,
		accID, p.UserID)
	if err != nil {
		return 0, err
	}
	return int(tag.RowsAffected()), nil
}

// DisableAuditMode clears the audit flag on an account's threads.
func (s *Service) DisableAuditMode(ctx context.Context, p *auth.Principal, accountPublicID string) error {
	if p.AccountType != "platform" {
		return httpx.Forbidden("platform only")
	}
	accID, _, err := s.accountByPublicID(ctx, accountPublicID)
	if err != nil {
		return err
	}
	_, err = s.pool.Exec(ctx, `
		UPDATE threads SET audit_mode=false, audit_enabled_by=NULL, audit_enabled_at=NULL
		WHERE id IN (SELECT thread_id FROM thread_members WHERE account_id=$1) AND audit_mode`,
		accID)
	return err
}

// ListAuditThreads returns audit-enabled threads for an account (platform only).
func (s *Service) ListAuditThreads(ctx context.Context, p *auth.Principal, accountPublicID string) ([]Thread, error) {
	if p.AccountType != "platform" {
		return nil, httpx.Forbidden("platform only")
	}
	accID, _, err := s.accountByPublicID(ctx, accountPublicID)
	if err != nil {
		return nil, err
	}
	rows, err := s.pool.Query(ctx, `
		SELECT t.id, t.public_id::text, t.type::text, t.context::text, t.status::text, t.title,
		       lead.public_id::text, ct.public_id::text, lead.first_name, lead.last_name, ct.name,
		       t.last_message_at, t.created_at, t.blocked_by,
		       false, 'active', 0
		FROM threads t
		LEFT JOIN leads lead ON lead.id=t.lead_id
		LEFT JOIN contracts ct ON ct.id=t.contract_id
		WHERE t.audit_mode
		  AND t.id IN (SELECT thread_id FROM thread_members WHERE account_id=$1)
		ORDER BY t.last_message_at DESC NULLS LAST`, accID)
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
