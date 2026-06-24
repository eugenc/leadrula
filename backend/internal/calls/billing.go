package calls

import (
	"context"

	"github.com/echayko/leadrula/backend/internal/billing"
	"github.com/echayko/leadrula/backend/internal/contracts"
	"github.com/echayko/leadrula/backend/internal/database"
	"github.com/echayko/leadrula/backend/internal/leads"
)

// ContinueWaterfall handles the Twilio <Dial> action callback that fires when a
// tier finishes. It bills + assigns on a billable connect, advances to the next
// tier on no-answer, or runs the no-answer multi-debit when all tiers are spent.
func (s *Service) ContinueWaterfall(ctx context.Context, callID int64, tier int, dialStatus string, dialDuration int, dialCallSID string) (string, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return "", err
	}
	defer tx.Rollback(ctx)

	call, err := s.getCallByID(ctx, tx, callID)
	if err != nil {
		return "", err
	}
	// Idempotency: Twilio retries must not re-bill a finalized call.
	if isTerminal(call.Status) {
		return twiMLHangup(), nil
	}

	if dialStatus == "completed" {
		twiml, err := s.finalizeConnect(ctx, tx, call, dialDuration, dialCallSID)
		if err != nil {
			return "", err
		}
		if err := tx.Commit(ctx); err != nil {
			return "", err
		}
		return twiml, nil
	}

	// Tier had no answer: mark this tier's open legs as no-answer.
	if _, err := tx.Exec(ctx,
		`UPDATE call_legs SET leg_status='no_answer', ended_at=now()
		 WHERE call_id=$1 AND tier=$2 AND leg_status IN ('dialing','ringing')`,
		callID, tier); err != nil {
		return "", err
	}

	// Try the next tier.
	trackingNumber, err := s.trackingNumberForCall(ctx, tx, call)
	if err != nil {
		return "", err
	}
	settings, err := s.GetCallSettings(ctx, tx, deref(call.ContractID))
	if err != nil {
		return "", err
	}
	twiml, dialed, err := s.dialTier(ctx, tx, call, trackingNumber, settings, tier) // tier is 1-based; tier == next index
	if err != nil {
		return "", err
	}
	if dialed {
		if err := tx.Commit(ctx); err != nil {
			return "", err
		}
		return twiml, nil
	}

	// No more tiers: bill every rung buyer at their own rate, no lead assigned.
	if err := s.noAnswerMultiDebit(ctx, tx, call); err != nil {
		return "", err
	}
	if _, err := tx.Exec(ctx, `UPDATE calls SET status='no_answer', ended_at=now() WHERE id=$1`, callID); err != nil {
		return "", err
	}
	if err := tx.Commit(ctx); err != nil {
		return "", err
	}
	return twiMLHangup(), nil
}

// finalizeConnect handles a tier where a buyer answered. Bills + assigns when the
// connected duration meets the threshold; otherwise no bill, no buyer lead.
func (s *Service) finalizeConnect(ctx context.Context, tx database.Querier, call *Call, dialDuration int, dialCallSID string) (string, error) {
	winner, err := s.winningLeg(ctx, tx, call.ID, dialCallSID)
	if err != nil {
		return "", err
	}
	settings, err := s.GetCallSettings(ctx, tx, deref(call.ContractID))
	if err != nil {
		return "", err
	}
	billable := winner != nil && dialDuration >= settings.DurationThresholdSec

	if !billable || winner == nil {
		// Sub-threshold (or no resolvable winner): publisher keeps the lead.
		if _, err := tx.Exec(ctx,
			`UPDATE calls SET status='completed', duration_sec=$2, billable=false, connected_at=COALESCE(connected_at, now()), ended_at=now()
			 WHERE id=$1`, call.ID, dialDuration); err != nil {
			return "", err
		}
		return twiMLHangup(), nil
	}

	price := winner.Rate
	contractID := deref(call.ContractID)
	_, compID, err := s.contractBasePrice(ctx, tx, contractID)
	if err != nil {
		return "", err
	}
	if call.LeadID == nil {
		return "", nil
	}
	leadID := *call.LeadID

	if err := billing.DebitCall(ctx, tx, *winner.BuyerID, price, leadID, contractID, call.ID, "call connected"); err != nil {
		return "", err
	}
	if compID != 0 {
		if err := contracts.RecordEarningDistribute(ctx, tx, compID, leadID, price, nil); err != nil {
			return "", err
		}
	}
	// Count cap usage on the billable event.
	if err := s.incrementCallCounters(ctx, tx, deref(winner.ParticipationID)); err != nil {
		return "", err
	}
	if err := s.markLegBilled(ctx, tx, winner.ID); err != nil {
		return "", err
	}

	emails, err := leads.AssignCallLeadAfterBill(ctx, tx, s.routeDeps(), contractID, deref(winner.ParticipationID), leadID)
	if err != nil {
		return "", err
	}

	priceCents := int(price*100 + 0.5)
	if _, err := tx.Exec(ctx,
		`UPDATE calls SET status='completed', billable=true, duration_sec=$2, billable_duration_sec=$2,
		   price_cents=$3, winner_participation_id=$4, connected_at=COALESCE(connected_at, now()), ended_at=now()
		 WHERE id=$1`,
		call.ID, dialDuration, priceCents, deref(winner.ParticipationID)); err != nil {
		return "", err
	}

	// Duplicate suppression on this source after a billable connect.
	if call.SourceID != nil {
		if err := s.addSuppression(ctx, tx, *call.SourceID, callerHashOf(call), call.ID, settings.DuplicateWindowHours); err != nil {
			return "", err
		}
	}

	s.notif.SendEmails(emails)
	return twiMLHangup(), nil
}

// noAnswerMultiDebit charges every buyer rung across all tiers (excluding legs a
// peer answer canceled) at their own rate. No lead is assigned. Publisher earns
// the total as a single hold entry.
func (s *Service) noAnswerMultiDebit(ctx context.Context, tx database.Querier, call *Call) error {
	rows, err := tx.Query(ctx,
		`SELECT id, participation_id, buyer_id, rate::float8
		 FROM call_legs
		 WHERE call_id=$1 AND billed=false
		   AND leg_status IN ('dialing','ringing','no_answer','busy','failed')`,
		call.ID)
	if err != nil {
		return err
	}
	type rung struct {
		legID, partID, buyerID int64
		rate                   float64
	}
	var rungs []rung
	for rows.Next() {
		var r rung
		var partID, buyerID *int64
		if err := rows.Scan(&r.legID, &partID, &buyerID, &r.rate); err != nil {
			rows.Close()
			return err
		}
		if partID != nil {
			r.partID = *partID
		}
		if buyerID != nil {
			r.buyerID = *buyerID
		}
		rungs = append(rungs, r)
	}
	rows.Close()

	contractID := deref(call.ContractID)
	_, compID, err := s.contractBasePrice(ctx, tx, contractID)
	if err != nil {
		return err
	}
	leadID := deref(call.LeadID)
	total := 0.0
	for _, r := range rungs {
		if r.buyerID == 0 || r.rate <= 0 {
			continue
		}
		// Skip the debit if the buyer can no longer cover it.
		if ok, err := s.buyerHasBalance(ctx, tx, r.buyerID, r.rate); err != nil {
			return err
		} else if !ok {
			continue
		}
		if err := billing.DebitCall(ctx, tx, r.buyerID, r.rate, leadID, contractID, call.ID, "call no-answer fee"); err != nil {
			return err
		}
		if err := s.incrementCallCounters(ctx, tx, r.partID); err != nil {
			return err
		}
		if err := s.markLegBilled(ctx, tx, r.legID); err != nil {
			return err
		}
		total += r.rate
	}
	if compID != 0 && total > 0 && leadID != 0 {
		if err := contracts.RecordEarningDistribute(ctx, tx, compID, leadID, total, nil); err != nil {
			return err
		}
	}
	return nil
}

// winningLeg returns the leg that connected: prefer the one matching the Twilio
// child SID, else any leg that was answered.
func (s *Service) winningLeg(ctx context.Context, q database.Querier, callID int64, dialCallSID string) (*CallLeg, error) {
	legs, err := s.legsForCall(ctx, q, callID)
	if err != nil {
		return nil, err
	}
	if dialCallSID != "" {
		for i := range legs {
			if legs[i].TwilioCallSID != nil && *legs[i].TwilioCallSID == dialCallSID {
				return &legs[i], nil
			}
		}
	}
	for i := range legs {
		if legs[i].AnsweredAt != nil || legs[i].LegStatus == "in_progress" || legs[i].LegStatus == "completed" {
			return &legs[i], nil
		}
	}
	return nil, nil
}

func (s *Service) markLegBilled(ctx context.Context, q database.Querier, legID int64) error {
	_, err := q.Exec(ctx, `UPDATE call_legs SET billed=true WHERE id=$1`, legID)
	return err
}

// HandleLegStatus updates a per-leg status callback: telemetry, concurrency
// decrement on terminal status, answered marker. Idempotent on the leg id.
func (s *Service) HandleLegStatus(ctx context.Context, legID int64, callStatus, childSID string, duration int) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	var participationID *int64
	var prevStatus string
	err = tx.QueryRow(ctx, `SELECT participation_id, leg_status FROM call_legs WHERE id=$1 FOR UPDATE`, legID).
		Scan(&participationID, &prevStatus)
	if err != nil {
		return err
	}
	if isLegTerminal(prevStatus) {
		return tx.Commit(ctx) // already finalized
	}

	legStatus := mapTwilioLegStatus(callStatus)
	answered := callStatus == "answered" || callStatus == "in-progress"
	if answered {
		if _, err := tx.Exec(ctx,
			`UPDATE call_legs SET leg_status='in_progress', twilio_call_sid=COALESCE($2, twilio_call_sid),
			   answered_at=COALESCE(answered_at, now()) WHERE id=$1`,
			legID, nullStr(childSID)); err != nil {
			return err
		}
		return tx.Commit(ctx)
	}

	if _, err := tx.Exec(ctx,
		`UPDATE call_legs SET leg_status=$2, twilio_call_sid=COALESCE($3, twilio_call_sid),
		   duration_sec=$4, ended_at=now() WHERE id=$1`,
		legID, legStatus, nullStr(childSID), duration); err != nil {
		return err
	}
	if isLegTerminal(legStatus) && participationID != nil {
		if err := s.adjustConcurrency(ctx, tx, *participationID, -1); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

// HandleRecording stores the Twilio recording URL on the call.
func (s *Service) HandleRecording(ctx context.Context, callID int64, recordingURL string, duration int) error {
	_, err := s.pool.Exec(ctx,
		`UPDATE calls SET recording_url=$2 WHERE id=$1`, callID, recordingURL)
	return err
}

func (s *Service) trackingNumberForCall(ctx context.Context, q database.Querier, call *Call) (string, error) {
	if call.SourceID == nil {
		return "", nil
	}
	var tn string
	err := q.QueryRow(ctx,
		`SELECT COALESCE(tracking_number,'') FROM routing_sources WHERE id=$1`, *call.SourceID).Scan(&tn)
	return tn, err
}

func isTerminal(status string) bool {
	switch status {
	case "completed", "no_answer", "blocked", "failed":
		return true
	}
	return false
}

func isLegTerminal(status string) bool {
	switch status {
	case "completed", "no_answer", "busy", "failed", "canceled":
		return true
	}
	return false
}

func mapTwilioLegStatus(s string) string {
	switch s {
	case "completed":
		return "completed"
	case "no-answer":
		return "no_answer"
	case "busy":
		return "busy"
	case "failed":
		return "failed"
	case "canceled":
		return "canceled"
	case "ringing", "initiated":
		return "ringing"
	default:
		return "dialing"
	}
}

func deref(p *int64) int64 {
	if p == nil {
		return 0
	}
	return *p
}

func callerHashOf(call *Call) string {
	if call.CallerNumber != nil {
		return hashPhone(*call.CallerNumber)
	}
	return ""
}
