package billing

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log"
	"strings"
	"time"

	"github.com/echayko/leadrula/backend/internal/contracts"
	"github.com/echayko/leadrula/backend/internal/database"
	"github.com/echayko/leadrula/backend/internal/notifications"
	"github.com/echayko/leadrula/backend/pkg/httpx"
	"github.com/jackc/pgx/v5"
)

const (
	partyBuyer     = "buyer"
	partyPublisher = "publisher"
)

// UploadFile is one attachment supplied with a dispute message.
type UploadFile struct {
	Filename    string
	ContentType string
	Size        int64
	Data        []byte
}

// disputeState holds the workflow fields needed to resolve a dispute.
type disputeState struct {
	id            int64
	buyerID       int64
	publisherID   int64
	leadID        *int64
	contractID    *int64
	transactionID int64
	initiatedBy   string
	status        string
	awaitingParty string
	amount        float64
	deadlineDays  int
	placementParty *string
	placementDone  bool
}

func clampDeadline(d int) int {
	if d < 7 {
		return 7
	}
	if d > 30 {
		return 30
	}
	return d
}

func otherParty(p string) string {
	if p == partyBuyer {
		return partyPublisher
	}
	return partyBuyer
}

func (d *disputeState) callerParty(accountID int64) (string, error) {
	switch accountID {
	case d.buyerID:
		return partyBuyer, nil
	case d.publisherID:
		return partyPublisher, nil
	}
	return "", httpx.NotFound("dispute not found")
}

func (s *Service) loadDisputeState(ctx context.Context, q database.Querier, id int64, lock bool) (*disputeState, error) {
	suffix := ""
	if lock {
		suffix = " FOR UPDATE OF d"
	}
	d := &disputeState{}
	var awaiting *string
	err := q.QueryRow(ctx,
		`SELECT d.id, d.buyer_id, COALESCE(c.publisher_id,0), d.lead_id, d.contract_id, d.transaction_id,
		        d.initiated_by, d.status, d.awaiting_party, d.amount::float8, d.deadline_days,
		        d.placement_party, d.placement_completed_at IS NOT NULL
		 FROM disputes d
		 LEFT JOIN contracts c ON c.id = COALESCE(d.contract_id,
		     (SELECT contract_id FROM transactions WHERE id = d.transaction_id))
		 WHERE d.id = $1`+suffix, id).
		Scan(&d.id, &d.buyerID, &d.publisherID, &d.leadID, &d.contractID, &d.transactionID,
			&d.initiatedBy, &d.status, &awaiting, &d.amount, &d.deadlineDays,
			&d.placementParty, &d.placementDone)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, httpx.NotFound("dispute not found")
	}
	if err != nil {
		return nil, err
	}
	if awaiting != nil {
		d.awaitingParty = *awaiting
	}
	return d, nil
}

// ── Open ───────────────────────────────────────────────────────────

// OpenReturnDispute lets a publisher dispute a returned lead: it re-charges the
// buyer immediately, freezes the lead (status=disputed) on the publisher's
// current pipeline, and opens a negotiation awaiting the buyer.
func (s *Service) OpenReturnDispute(ctx context.Context, publisherID, userID, leadID int64, reason string, deadlineDays int) (*Dispute, error) {
	reason = strings.TrimSpace(reason)
	if reason == "" {
		return nil, httpx.Validation("a dispute reason is required")
	}
	deadlineDays = clampDeadline(deadlineDays)

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	var ownerID, leadPublisherID int64
	var status string
	err = tx.QueryRow(ctx,
		`SELECT owner_account_id, publisher_id, status::text FROM leads WHERE id=$1 AND deleted_at IS NULL FOR UPDATE`,
		leadID).Scan(&ownerID, &leadPublisherID, &status)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, httpx.NotFound("lead not found")
	}
	if err != nil {
		return nil, err
	}
	if leadPublisherID != publisherID || ownerID != publisherID {
		return nil, httpx.NotFound("lead not found")
	}
	if status != "returned" {
		return nil, httpx.BusinessRule("only returned leads can be disputed")
	}

	buyerID, contractID, rate, err := returnSaleContext(ctx, tx, leadID)
	if err != nil {
		return nil, err
	}

	var existing bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM disputes WHERE lead_id=$1 AND status='open')`, leadID).Scan(&existing); err != nil {
		return nil, err
	}
	if existing {
		return nil, httpx.Conflict("a dispute is already open for this lead")
	}

	if err := setLeadStatus(ctx, tx, leadID, "disputed", "Dispute opened"); err != nil {
		return nil, err
	}
	if err := Debit(ctx, tx, buyerID, rate, leadID, contractID, "lead disputed"); err != nil {
		return nil, err
	}
	txnID, err := latestTxnID(ctx, tx, buyerID, leadID, contractID, "debit", "lead disputed")
	if err != nil {
		return nil, err
	}
	if target, terr := contracts.GetTargetByContract(ctx, tx, contractID); terr == nil {
		if err := contracts.RecordEarningPublisherDispute(ctx, tx, target.CompensationID, leadID, rate); err != nil {
			return nil, err
		}
	}

	deadlineAt := time.Now().Add(time.Duration(deadlineDays) * 24 * time.Hour)
	var dID int64
	if err := tx.QueryRow(ctx,
		`INSERT INTO disputes(transaction_id, buyer_id, reason, status, initiated_by, lead_id, contract_id,
		     amount, deadline_days, response_deadline_at, awaiting_party)
		 VALUES ($1,$2,$3,'open','publisher',$4,$5,$6,$7,$8,'buyer') RETURNING id`,
		txnID, buyerID, reason, leadID, contractID, rate, deadlineDays, deadlineAt).Scan(&dID); err != nil {
		return nil, err
	}
	if _, err := insertDisputeMessage(ctx, tx, dID, userID, publisherID, partyPublisher, "open", reason); err != nil {
		return nil, err
	}
	emails, err := s.notifyDisputeAccount(ctx, tx, buyerID, "lead_disputed", map[string]any{"dispute_id": dID, "lead_id": leadID})
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	s.notif.SendEmails(emails)
	return s.disputeByID(ctx, buyerID, dID)
}

// OpenDispute lets a buyer dispute a debit transaction. No money moves until the
// publisher accepts; it opens a negotiation awaiting the publisher.
func (s *Service) OpenDispute(ctx context.Context, buyerID, userID, transactionID int64, reason string, deadlineDays int) (*Dispute, error) {
	reason = strings.TrimSpace(reason)
	deadlineDays = clampDeadline(deadlineDays)

	var ttype string
	var leadID, contractID *int64
	err := s.pool.QueryRow(ctx,
		`SELECT type::text, lead_id, contract_id FROM transactions WHERE id=$1 AND buyer_id=$2`,
		transactionID, buyerID).Scan(&ttype, &leadID, &contractID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, httpx.NotFound("transaction not found")
	}
	if err != nil {
		return nil, err
	}
	if ttype != "debit" {
		return nil, httpx.BusinessRule("only debit transactions can be disputed")
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	var existing bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM disputes WHERE transaction_id=$1 AND status='open')`, transactionID).Scan(&existing); err != nil {
		return nil, err
	}
	if existing {
		return nil, httpx.Conflict("a dispute is already open for this transaction")
	}

	var amount float64
	if err := tx.QueryRow(ctx, `SELECT abs(amount)::float8 FROM transactions WHERE id=$1`, transactionID).Scan(&amount); err != nil {
		return nil, err
	}
	deadlineAt := time.Now().Add(time.Duration(deadlineDays) * 24 * time.Hour)
	var dID, publisherID int64
	if err := tx.QueryRow(ctx,
		`INSERT INTO disputes(transaction_id, buyer_id, reason, status, initiated_by, lead_id, contract_id,
		     amount, deadline_days, response_deadline_at, awaiting_party)
		 VALUES ($1,$2,$3,'open','buyer',$4,$5,$6,$7,$8,'publisher') RETURNING id`,
		transactionID, buyerID, reason, leadID, contractID, amount, deadlineDays, deadlineAt).Scan(&dID); err != nil {
		return nil, err
	}
	if _, err := insertDisputeMessage(ctx, tx, dID, userID, buyerID, partyBuyer, "open", reason); err != nil {
		return nil, err
	}
	if contractID != nil {
		_ = tx.QueryRow(ctx, `SELECT publisher_id FROM contracts WHERE id=$1`, *contractID).Scan(&publisherID)
	}
	var emails []notifications.EmailJob
	if publisherID != 0 {
		emails, err = s.notifyDisputeAccount(ctx, tx, publisherID, "lead_disputed", map[string]any{"dispute_id": dID, "lead_id": leadID})
		if err != nil {
			return nil, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	s.notif.SendEmails(emails)
	return s.disputeByID(ctx, buyerID, dID)
}

// ── Accept / Reject / Placement ────────────────────────────────────

// ResolveAccept closes a dispute in the accepting party's counterpart favor and
// applies the money + lead movement rules for the initiator. pipelineID/stageID
// are required when the accepting party receives the lead.
func (s *Service) ResolveAccept(ctx context.Context, accountID, userID, disputeID, pipelineID, stageID int64) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	d, err := s.loadDisputeState(ctx, tx, disputeID, true)
	if err != nil {
		return err
	}
	party, err := d.callerParty(accountID)
	if err != nil {
		return err
	}
	if d.status != "open" {
		return httpx.BusinessRule("dispute already resolved")
	}
	if party != d.awaitingParty {
		return httpx.BusinessRule("waiting for the other party to respond")
	}

	cp := closeParams{status: "accepted", outcome: "accepted", resolvedBy: userID}
	switch {
	case d.initiatedBy == partyPublisher && party == partyBuyer:
		// Buyer accepts the re-charge: publisher wins, lead moves to the buyer.
		if pipelineID == 0 || stageID == 0 {
			return httpx.Validation("choose a pipeline and stage for the lead")
		}
		if err := s.moveLeadToBuyer(ctx, tx, d, pipelineID, stageID); err != nil {
			return err
		}
		cp.winner = partyPublisher
		cp.placed = true
		cp.placementPipelineID = pipelineID
		cp.placementStageID = stageID
	case d.initiatedBy == partyPublisher && party == partyPublisher:
		// Publisher concedes: buyer wins, refund, lead stays returned on publisher.
		if err := s.refundBuyer(ctx, tx, d.buyerID, d.amount); err != nil {
			return err
		}
		if d.leadID != nil {
			if err := contracts.ReverseEarningPublisherDispute(ctx, tx, *d.leadID, d.contractID); err != nil {
				return err
			}
			if err := setLeadStatus(ctx, tx, *d.leadID, "returned", "Dispute accepted"); err != nil {
				return err
			}
		}
		cp.winner = partyBuyer
	case d.initiatedBy == partyBuyer && party == partyPublisher:
		// Publisher accepts buyer's dispute: buyer wins, refund, lead moves to publisher.
		if pipelineID == 0 || stageID == 0 {
			return httpx.Validation("choose a pipeline and stage for the lead")
		}
		if err := s.refundBuyer(ctx, tx, d.buyerID, d.amount); err != nil {
			return err
		}
		if err := contracts.RecordEarningDispute(ctx, tx, d.transactionID, d.leadID, d.contractID, d.amount); err != nil {
			return err
		}
		if err := s.moveLeadToPublisher(ctx, tx, d, pipelineID, stageID); err != nil {
			return err
		}
		cp.winner = partyBuyer
		cp.placed = true
		cp.placementPipelineID = pipelineID
		cp.placementStageID = stageID
	default:
		// Buyer withdraws their own dispute: no money, lead stays with buyer.
		if d.leadID != nil {
			if err := setLeadStatus(ctx, tx, *d.leadID, "distributed", "Dispute withdrawn"); err != nil {
				return err
			}
		}
		cp.status = "rejected"
		cp.outcome = "withdrawn"
		cp.winner = partyPublisher
	}

	if err := closeDispute(ctx, tx, d.id, cp); err != nil {
		return err
	}
	notifyAccount := s.partyAccountID(d, otherParty(party))
	emails, err := s.notifyDisputeAccount(ctx, tx, notifyAccount, "dispute_message", map[string]any{"dispute_id": d.id, "event": "resolved", "outcome": cp.outcome})
	if err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return err
	}
	s.notif.SendEmails(emails)
	return nil
}

// ResolveReject keeps the dispute open, records a message + attachments, and
// flips the turn to the other party with a fresh deadline.
func (s *Service) ResolveReject(ctx context.Context, accountID, userID, disputeID int64, body string, files []UploadFile) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	d, err := s.loadDisputeState(ctx, tx, disputeID, true)
	if err != nil {
		return err
	}
	party, err := d.callerParty(accountID)
	if err != nil {
		return err
	}
	if d.status != "open" {
		return httpx.BusinessRule("dispute already resolved")
	}
	if party != d.awaitingParty {
		return httpx.BusinessRule("waiting for the other party to respond")
	}

	msgID, err := insertDisputeMessage(ctx, tx, d.id, userID, accountID, party, "reject", strings.TrimSpace(body))
	if err != nil {
		return err
	}
	if err := s.storeAttachments(ctx, tx, msgID, d.id, files); err != nil {
		return err
	}
	newAwaiting := otherParty(party)
	deadlineAt := time.Now().Add(time.Duration(d.deadlineDays) * 24 * time.Hour)
	if _, err := tx.Exec(ctx, `UPDATE disputes SET awaiting_party=$2, response_deadline_at=$3 WHERE id=$1`,
		d.id, newAwaiting, deadlineAt); err != nil {
		return err
	}
	emails, err := s.notifyDisputeAccount(ctx, tx, s.partyAccountID(d, newAwaiting), "dispute_message", map[string]any{"dispute_id": d.id, "event": "reject"})
	if err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return err
	}
	s.notif.SendEmails(emails)
	return nil
}

// PostMessage appends a plain message (with optional attachments) to a dispute
// thread without changing the turn or resolving it.
func (s *Service) PostMessage(ctx context.Context, accountID, userID, disputeID int64, body string, files []UploadFile) (*DisputeMessage, error) {
	body = strings.TrimSpace(body)
	if body == "" && len(files) == 0 {
		return nil, httpx.Validation("message body or attachment is required")
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	d, err := s.loadDisputeState(ctx, tx, disputeID, true)
	if err != nil {
		return nil, err
	}
	party, err := d.callerParty(accountID)
	if err != nil {
		return nil, err
	}
	msgID, err := insertDisputeMessage(ctx, tx, d.id, userID, accountID, party, "message", body)
	if err != nil {
		return nil, err
	}
	if err := s.storeAttachments(ctx, tx, msgID, d.id, files); err != nil {
		return nil, err
	}
	emails, err := s.notifyDisputeAccount(ctx, tx, s.partyAccountID(d, otherParty(party)), "dispute_message", map[string]any{"dispute_id": d.id, "event": "message"})
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	s.notif.SendEmails(emails)
	msgs, err := s.loadMessages(ctx, s.pool, d.id, msgID)
	if err != nil || len(msgs) == 0 {
		return nil, err
	}
	return &msgs[0], nil
}

// SubmitPlacement lets the losing party of an auto-resolved dispute choose where
// the lead lands once the dispute is already closed.
func (s *Service) SubmitPlacement(ctx context.Context, accountID, userID, disputeID, pipelineID, stageID int64) error {
	if pipelineID == 0 || stageID == 0 {
		return httpx.Validation("choose a pipeline and stage for the lead")
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	d, err := s.loadDisputeState(ctx, tx, disputeID, true)
	if err != nil {
		return err
	}
	party, err := d.callerParty(accountID)
	if err != nil {
		return err
	}
	if d.placementParty == nil {
		return httpx.BusinessRule("no placement is required for this dispute")
	}
	if d.placementDone {
		return httpx.BusinessRule("placement already completed")
	}
	if party != *d.placementParty {
		return httpx.BusinessRule("placement is for the other party")
	}

	if d.initiatedBy == partyPublisher {
		if err := s.moveLeadToBuyer(ctx, tx, d, pipelineID, stageID); err != nil {
			return err
		}
	} else {
		if err := s.moveLeadToPublisher(ctx, tx, d, pipelineID, stageID); err != nil {
			return err
		}
	}
	if _, err := tx.Exec(ctx,
		`UPDATE disputes SET placement_pipeline_id=$2, placement_stage_id=$3, placement_completed_at=now() WHERE id=$1`,
		d.id, pipelineID, stageID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// ── Messages / attachments reads ───────────────────────────────────

func (s *Service) ListDisputeMessages(ctx context.Context, accountID, disputeID int64) ([]DisputeMessage, error) {
	d, err := s.loadDisputeState(ctx, s.pool, disputeID, false)
	if err != nil {
		return nil, err
	}
	if _, err := d.callerParty(accountID); err != nil {
		return nil, err
	}
	return s.loadMessages(ctx, s.pool, disputeID, 0)
}

// DisputeAttachment streams an attachment after verifying the caller is a party.
func (s *Service) DisputeAttachment(ctx context.Context, accountID, attachmentID int64) (io.ReadCloser, string, string, error) {
	var storageKey, filename, contentType string
	var disputeID int64
	err := s.pool.QueryRow(ctx,
		`SELECT a.storage_key, a.filename, a.content_type, m.dispute_id
		 FROM dispute_message_attachments a
		 JOIN dispute_messages m ON m.id = a.message_id
		 WHERE a.id = $1`, attachmentID).Scan(&storageKey, &filename, &contentType, &disputeID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, "", "", httpx.NotFound("attachment not found")
	}
	if err != nil {
		return nil, "", "", err
	}
	d, err := s.loadDisputeState(ctx, s.pool, disputeID, false)
	if err != nil {
		return nil, "", "", err
	}
	if _, err := d.callerParty(accountID); err != nil {
		return nil, "", "", err
	}
	if s.attachments == nil || !s.attachments.Enabled() {
		return nil, "", "", httpx.BusinessRule("attachment storage not configured")
	}
	reader, ctype, err := s.attachments.Get(ctx, storageKey)
	if err != nil {
		return nil, "", "", err
	}
	if ctype == "" {
		ctype = contentType
	}
	return reader, ctype, filename, nil
}

// ── Deadline auto-resolution ───────────────────────────────────────

// RunDisputeDeadlineWorker periodically auto-resolves disputes whose deadline
// elapsed without a response (the non-responder loses).
func (s *Service) RunDisputeDeadlineWorker(ctx context.Context) {
	ticker := time.NewTicker(15 * time.Minute)
	defer ticker.Stop()
	for {
		s.processDisputeDeadlines(ctx)
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (s *Service) processDisputeDeadlines(ctx context.Context) {
	rows, err := s.pool.Query(ctx,
		`SELECT id FROM disputes WHERE status='open' AND response_deadline_at IS NOT NULL AND response_deadline_at < now()`)
	if err != nil {
		log.Printf("dispute deadline worker: query: %v", err)
		return
	}
	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			log.Printf("dispute deadline worker: scan: %v", err)
			return
		}
		ids = append(ids, id)
	}
	rows.Close()
	for _, id := range ids {
		if err := s.autoResolveDispute(ctx, id); err != nil {
			log.Printf("dispute deadline worker: auto-resolve %d: %v", id, err)
		}
	}
}

func (s *Service) autoResolveDispute(ctx context.Context, disputeID int64) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	d, err := s.loadDisputeState(ctx, tx, disputeID, true)
	if err != nil {
		return err
	}
	if d.status != "open" {
		return nil
	}
	var stillDue bool
	if err := tx.QueryRow(ctx,
		`SELECT response_deadline_at IS NOT NULL AND response_deadline_at < now() FROM disputes WHERE id=$1`,
		disputeID).Scan(&stillDue); err != nil {
		return err
	}
	if !stillDue {
		return nil
	}

	loser := d.awaitingParty
	winner := otherParty(loser)
	cp := closeParams{status: "accepted", outcome: "auto_accepted", winner: winner}

	switch {
	case d.initiatedBy == partyPublisher && loser == partyBuyer:
		// Buyer did not respond: charge stands, lead must move to buyer (placement pending).
		cp.placementParty = partyBuyer
	case d.initiatedBy == partyPublisher && loser == partyPublisher:
		// Publisher did not respond: refund buyer, lead stays returned on publisher.
		if err := s.refundBuyer(ctx, tx, d.buyerID, d.amount); err != nil {
			return err
		}
		if d.leadID != nil {
			if err := contracts.ReverseEarningPublisherDispute(ctx, tx, *d.leadID, d.contractID); err != nil {
				return err
			}
			if err := setLeadStatus(ctx, tx, *d.leadID, "returned", "Dispute auto-accepted"); err != nil {
				return err
			}
		}
	case d.initiatedBy == partyBuyer && loser == partyPublisher:
		// Publisher did not respond: refund buyer, lead must move to publisher (placement pending).
		if err := s.refundBuyer(ctx, tx, d.buyerID, d.amount); err != nil {
			return err
		}
		if err := contracts.RecordEarningDispute(ctx, tx, d.transactionID, d.leadID, d.contractID, d.amount); err != nil {
			return err
		}
		cp.placementParty = partyPublisher
	default:
		// Buyer did not respond on their own dispute: publisher wins, no money/move.
		if d.leadID != nil {
			if err := setLeadStatus(ctx, tx, *d.leadID, "distributed", "Dispute auto-rejected"); err != nil {
				return err
			}
		}
		cp.status = "rejected"
		cp.outcome = "auto_rejected"
	}

	if err := closeDispute(ctx, tx, d.id, cp); err != nil {
		return err
	}
	if _, err := insertDisputeMessage(ctx, tx, d.id, 0, s.partyAccountID(d, winner), winner, "system", "Auto-resolved: response deadline elapsed"); err != nil {
		return err
	}
	var emails []notifications.EmailJob
	for _, acc := range []int64{d.buyerID, d.publisherID} {
		if acc == 0 {
			continue
		}
		e, err := s.notifyDisputeAccount(ctx, tx, acc, "dispute_deadline", map[string]any{"dispute_id": d.id, "outcome": cp.outcome})
		if err != nil {
			return err
		}
		emails = append(emails, e...)
	}
	if err := tx.Commit(ctx); err != nil {
		return err
	}
	s.notif.SendEmails(emails)
	return nil
}

// ── Helpers ────────────────────────────────────────────────────────

type closeParams struct {
	status              string
	outcome             string
	winner              string
	placementParty      string
	placementPipelineID int64
	placementStageID    int64
	placed              bool
	resolvedBy          int64
}

func closeDispute(ctx context.Context, q database.Querier, id int64, p closeParams) error {
	var placedAt *time.Time
	if p.placed {
		now := time.Now()
		placedAt = &now
	}
	_, err := q.Exec(ctx,
		`UPDATE disputes SET status=$2::dispute_status, outcome=$3, winner_party=$4, awaiting_party=NULL,
		     placement_party=NULLIF($5,''), placement_pipeline_id=NULLIF($6,0), placement_stage_id=NULLIF($7,0),
		     placement_completed_at=$8, resolved_by=NULLIF($9,0), resolved_at=now()
		 WHERE id=$1`,
		id, p.status, p.outcome, p.winner, p.placementParty, p.placementPipelineID, p.placementStageID, placedAt, p.resolvedBy)
	return err
}

func (s *Service) partyAccountID(d *disputeState, party string) int64 {
	if party == partyBuyer {
		return d.buyerID
	}
	return d.publisherID
}

// moveLeadToBuyer places a disputed lead into the buyer's chosen pipeline and
// marks it distributed. Distributed leads live only on the buyer board.
func (s *Service) moveLeadToBuyer(ctx context.Context, q database.Querier, d *disputeState, pipelineID, stageID int64) error {
	if d.leadID == nil || d.contractID == nil {
		return httpx.BusinessRule("dispute has no lead or contract")
	}
	return moveLead(ctx, q, *d.leadID, d.buyerID, pipelineID, stageID, d.contractID, "distributed", "Dispute resolved")
}

// moveLeadToPublisher returns a disputed lead to the publisher's chosen pipeline.
func (s *Service) moveLeadToPublisher(ctx context.Context, q database.Querier, d *disputeState, pipelineID, stageID int64) error {
	if d.leadID == nil {
		return httpx.BusinessRule("dispute has no lead")
	}
	return moveLead(ctx, q, *d.leadID, d.publisherID, pipelineID, stageID, nil, "returned", "Dispute resolved")
}

func moveLead(ctx context.Context, q database.Querier, leadID, ownerID, pipelineID, stageID int64, contractID *int64, status, label string) error {
	var pipeOwner int64
	if err := q.QueryRow(ctx, `SELECT account_id FROM pipelines WHERE id=$1`, pipelineID).Scan(&pipeOwner); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return httpx.BusinessRule("pipeline not found")
		}
		return err
	}
	if pipeOwner != ownerID {
		return httpx.BusinessRule("pipeline does not belong to this account")
	}
	var ok bool
	if err := q.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pipeline_stages WHERE id=$1 AND pipeline_id=$2)`, stageID, pipelineID).Scan(&ok); err != nil {
		return err
	}
	if !ok {
		return httpx.BusinessRule("stage does not belong to pipeline")
	}
	var pipelineName, stageName string
	_ = q.QueryRow(ctx, `SELECT p.name, ps.name FROM pipelines p JOIN pipeline_stages ps ON ps.id=$2 WHERE p.id=$1`, pipelineID, stageID).Scan(&pipelineName, &stageName)
	if _, err := q.Exec(ctx,
		`INSERT INTO lead_change_log(lead_id, owner_account_id, actor_type, actor_label, change_kind, field_name, from_value, to_value)
		 VALUES ($1,$2,'system',$3,'pipeline_placed','Pipeline',$4,$5)`,
		leadID, ownerID, label, pipelineName, stageName); err != nil {
		return err
	}
	_, err := q.Exec(ctx,
		`UPDATE leads SET owner_account_id=$2, pipeline_id=$3, stage_id=$4, contract_id=$5, status=$6::lead_status,
		     publisher_pipeline_id=NULL, publisher_stage_id=NULL,
		     position=COALESCE((SELECT MAX(position)+1 FROM leads WHERE stage_id=$4),0)
		 WHERE id=$1`,
		leadID, ownerID, pipelineID, stageID, contractID, status)
	return err
}

func setLeadStatus(ctx context.Context, q database.Querier, leadID int64, status, label string) error {
	var owner int64
	var from string
	if err := q.QueryRow(ctx, `SELECT owner_account_id, status::text FROM leads WHERE id=$1`, leadID).Scan(&owner, &from); err != nil {
		return err
	}
	if from != status {
		if _, err := q.Exec(ctx,
			`INSERT INTO lead_change_log(lead_id, owner_account_id, actor_type, actor_label, change_kind, field_name, from_value, to_value)
			 VALUES ($1,$2,'system',$3,'status','Status',$4,$5)`,
			leadID, owner, label, from, status); err != nil {
			return err
		}
	}
	_, err := q.Exec(ctx, `UPDATE leads SET status=$2::lead_status WHERE id=$1`, leadID, status)
	return err
}

func (s *Service) refundBuyer(ctx context.Context, q database.Querier, buyerID int64, amount float64) error {
	if amount <= 0 {
		return nil
	}
	if err := EnsureBalance(ctx, q, buyerID); err != nil {
		return err
	}
	var balance float64
	if err := q.QueryRow(ctx, `SELECT balance::float8 FROM buyer_balances WHERE buyer_id=$1 FOR UPDATE`, buyerID).Scan(&balance); err != nil {
		return err
	}
	newBal := balance + amount
	if _, err := q.Exec(ctx, `UPDATE buyer_balances SET balance=$2 WHERE buyer_id=$1`, buyerID, newBal); err != nil {
		return err
	}
	_, err := q.Exec(ctx,
		`INSERT INTO transactions(buyer_id, type, amount, balance_after, description)
		 VALUES ($1,'dispute_credit',$2,$3,$4)`,
		buyerID, amount, newBal, "dispute accepted")
	return err
}

func returnSaleContext(ctx context.Context, q database.Querier, leadID int64) (buyerID int64, contractID int64, rate float64, err error) {
	err = q.QueryRow(ctx,
		`SELECT buyer_id, contract_id, abs(amount)::float8
		 FROM transactions
		 WHERE lead_id=$1 AND type='credit' AND description='lead returned' AND contract_id IS NOT NULL
		 ORDER BY created_at DESC LIMIT 1`, leadID).Scan(&buyerID, &contractID, &rate)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, 0, 0, httpx.BusinessRule("no refundable return was found for this lead")
	}
	if err != nil {
		return 0, 0, 0, err
	}
	if rate <= 0 {
		return 0, 0, 0, httpx.BusinessRule("return had no charge to dispute")
	}
	return buyerID, contractID, rate, nil
}

func latestTxnID(ctx context.Context, q database.Querier, buyerID, leadID, contractID int64, ttype, desc string) (int64, error) {
	var id int64
	err := q.QueryRow(ctx,
		`SELECT id FROM transactions
		 WHERE buyer_id=$1 AND lead_id=$2 AND contract_id=$3 AND type=$4::txn_type AND description=$5
		 ORDER BY created_at DESC, id DESC LIMIT 1`,
		buyerID, leadID, contractID, ttype, desc).Scan(&id)
	return id, err
}

func insertDisputeMessage(ctx context.Context, q database.Querier, disputeID, userID, accountID int64, party, kind, body string) (int64, error) {
	var id int64
	err := q.QueryRow(ctx,
		`INSERT INTO dispute_messages(dispute_id, user_id, account_id, author_party, kind, body)
		 VALUES ($1, NULLIF($2,0), $3, $4, $5, $6) RETURNING id`,
		disputeID, userID, accountID, party, kind, body).Scan(&id)
	return id, err
}

func (s *Service) storeAttachments(ctx context.Context, q database.Querier, msgID, disputeID int64, files []UploadFile) error {
	if len(files) == 0 {
		return nil
	}
	if s.attachments == nil || !s.attachments.Enabled() {
		return httpx.BusinessRule("attachment storage is not configured")
	}
	for _, f := range files {
		suffix := make([]byte, 6)
		_, _ = rand.Read(suffix)
		key := fmt.Sprintf("disputes/%d/%d/%s-%s", disputeID, msgID, hex.EncodeToString(suffix), safeFilename(f.Filename))
		if err := s.attachments.Put(ctx, key, f.ContentType, bytes.NewReader(f.Data)); err != nil {
			return err
		}
		if _, err := q.Exec(ctx,
			`INSERT INTO dispute_message_attachments(message_id, storage_key, filename, content_type, byte_size)
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

// loadMessages returns the dispute thread; when onlyID > 0 it returns just that message.
func (s *Service) loadMessages(ctx context.Context, q database.Querier, disputeID, onlyID int64) ([]DisputeMessage, error) {
	rows, err := q.Query(ctx,
		`SELECT m.id, m.dispute_id, m.author_party, COALESCE(u.full_name,''), m.kind, m.body, m.created_at
		 FROM dispute_messages m
		 LEFT JOIN users u ON u.id = m.user_id
		 WHERE m.dispute_id=$1 AND ($2=0 OR m.id=$2)
		 ORDER BY m.created_at, m.id`, disputeID, onlyID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []DisputeMessage
	byID := map[int64]int{}
	for rows.Next() {
		var m DisputeMessage
		if err := rows.Scan(&m.ID, &m.DisputeID, &m.AuthorParty, &m.AuthorName, &m.Kind, &m.Body, &m.CreatedAt); err != nil {
			return nil, err
		}
		byID[m.ID] = len(out)
		out = append(out, m)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(out) == 0 {
		return out, nil
	}
	arows, err := q.Query(ctx,
		`SELECT a.id, a.message_id, a.filename, a.content_type, a.byte_size
		 FROM dispute_message_attachments a
		 JOIN dispute_messages m ON m.id = a.message_id
		 WHERE m.dispute_id=$1 AND ($2=0 OR m.id=$2)
		 ORDER BY a.id`, disputeID, onlyID)
	if err != nil {
		return nil, err
	}
	defer arows.Close()
	for arows.Next() {
		var a DisputeAttachment
		if err := arows.Scan(&a.ID, &a.MessageID, &a.Filename, &a.ContentType, &a.ByteSize); err != nil {
			return nil, err
		}
		if idx, ok := byID[a.MessageID]; ok {
			out[idx].Attachments = append(out[idx].Attachments, a)
		}
	}
	return out, arows.Err()
}

func (s *Service) notifyDisputeAccount(ctx context.Context, q database.Querier, accountID int64, eventType string, payload map[string]any) ([]notifications.EmailJob, error) {
	if accountID == 0 {
		return nil, nil
	}
	ids, err := s.accounts.AccountUserIDs(ctx, q, accountID)
	if err != nil {
		return nil, err
	}
	return s.notif.Deliver(ctx, q, notifications.DeliverParams{
		AccountID: accountID,
		UserIDs:   ids,
		EventType: eventType,
		Payload:   payload,
	})
}

// disputeByID returns a single dispute for an account-scoped response.
func (s *Service) disputeByID(ctx context.Context, accountID, disputeID int64) (*Dispute, error) {
	q := `SELECT ` + disputeCols + `,
	             NULLIF(trim(ba.name), ''), NULLIF(trim(pub.name), ''), pub.type::text
	      FROM disputes d
	      JOIN transactions t ON t.id = d.transaction_id
	      JOIN accounts ba ON ba.id = d.buyer_id
	      LEFT JOIN leads l ON l.id = d.lead_id
	      LEFT JOIN contracts c ON c.id = COALESCE(d.contract_id, t.contract_id)
	      LEFT JOIN accounts pub ON pub.id = c.publisher_id
	      WHERE d.id = $1`
	list, err := s.queryDisputes(ctx, q, disputeID)
	if err != nil {
		return nil, err
	}
	if len(list) == 0 {
		return nil, httpx.NotFound("dispute not found")
	}
	return &list[0], nil
}
