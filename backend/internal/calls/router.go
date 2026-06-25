package calls

import (
	"context"
	"encoding/json"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/echayko/leadrula/backend/internal/contracts"
	"github.com/echayko/leadrula/backend/internal/database"
	"github.com/echayko/leadrula/backend/internal/leads"
	"github.com/echayko/leadrula/backend/internal/routing"
)

// callParticipant is an eligible buyer destination loaded for routing.
type callParticipant struct {
	participationID int64
	buyerID         int64
	priority        int
	weight          int
	targetType      string
	destination     string
	rtbEndpoint     string
	rtbHeadersEnc   []byte
	rateOverride    *float64
	schedule        json.RawMessage
	dailyCap        *int
	monthlyCap      *int
	concurrencyCap  *int
	callsToday      int
	callsThisMonth  int
	concurrent      int
}

// HandleInbound processes a Twilio inbound voice webhook. It creates the publisher
// lead and call, merges any preload, resolves the contract, and returns TwiML to
// dial the first eligible tier (or hold/reject).
func (s *Service) HandleInbound(ctx context.Context, trackingNumber, caller, twilioSID, preloadToken string) (string, error) {
	src, err := routing.SourceByTrackingNumber(ctx, s.pool, trackingNumber)
	if err != nil {
		return "", err
	}
	if src == nil {
		// Unknown number: nobody to attribute the call to. Hold.
		return twiMLHold(), nil
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return "", err
	}
	defer tx.Rollback(ctx)

	callerHash := hashPhone(caller)
	merged := map[string]any{"source": src.Slug, "caller_number": caller}
	if preload, err := s.matchPreload(ctx, tx, src.ID, callerHash, preloadToken); err == nil && preload != nil {
		for k, v := range preload {
			merged[k] = v
		}
	}
	rawJSON, _ := json.Marshal(merged)

	leadID, _, err := s.leads.InsertLead(ctx, tx, src.PublisherID, src.PublisherID, src.Slug, rawJSON)
	if err != nil {
		return "", err
	}
	if caller != "" {
		if err := s.leads.SetBuiltinField(ctx, tx, leadID, "phone", caller); err != nil {
			return "", err
		}
	}
	if src.PayloadEnabled {
		if err := s.applyPreloadFieldMap(ctx, tx, src.ID, src.PublisherID, leadID, src.Name, merged); err != nil {
			return "", err
		}
	}

	call, err := s.insertCall(ctx, tx, src.PublisherID, &src.ID, nil, &leadID, caller, callerHash, "", trackingNumber, twilioSID, "inbound")
	if err != nil {
		return "", err
	}

	// Duplicate suppression (billable connects only, per source).
	if callerHash != "" {
		suppressed, err := s.isSuppressed(ctx, tx, src.ID, callerHash)
		if err != nil {
			return "", err
		}
		if suppressed {
			if err := s.markCallBlocked(ctx, tx, call.ID, leadID); err != nil {
				return "", err
			}
			if err := tx.Commit(ctx); err != nil {
				return "", err
			}
			return twiMLReject(), nil
		}
	}

	// Resolve the contract via the source's routes (conditions select which
	// Call contract handles this call).
	rt, err := routing.RouteForSource(ctx, tx, src.ID, leadID, merged)
	if err != nil {
		return "", err
	}
	if rt == nil || rt.Destination != "contract" || rt.ContractID == nil {
		// No matching contract: caller waits on hold, no billing.
		if err := tx.Commit(ctx); err != nil {
			return "", err
		}
		return twiMLHold(), nil
	}
	contractID := *rt.ContractID
	if _, err := tx.Exec(ctx, `UPDATE calls SET contract_id=$2, status='ringing' WHERE id=$1`, call.ID, contractID); err != nil {
		return "", err
	}
	call.ContractID = &contractID

	settings, err := s.GetCallSettings(ctx, tx, contractID)
	if err != nil {
		return "", err
	}

	twiml, dialed, err := s.dialTier(ctx, tx, call, trackingNumber, settings, 0)
	if err != nil {
		return "", err
	}
	if !dialed {
		// No eligible buyers: hold indefinitely.
		if err := tx.Commit(ctx); err != nil {
			return "", err
		}
		return twiMLHold(), nil
	}
	if err := tx.Commit(ctx); err != nil {
		return "", err
	}
	return twiml, nil
}

// dialTier builds the simuldial for the tier-th distinct priority. Returns
// (twiml, true) when at least one buyer is dialed, else ("", false).
func (s *Service) dialTier(ctx context.Context, tx database.Querier, call *Call, trackingNumber string, settings *CallSettings, tierIndex int) (string, bool, error) {
	if call.ContractID == nil {
		return "", false, nil
	}
	parts, err := s.loadCallParticipants(ctx, tx, *call.ContractID)
	if err != nil {
		return "", false, err
	}
	tiers := groupTiers(parts)
	if tierIndex >= len(tiers) {
		return "", false, nil
	}
	multiBuyer := countConfigured(parts) >= 2
	basePrice, _, err := s.contractBasePrice(ctx, tx, *call.ContractID)
	if err != nil {
		return "", false, err
	}

	var dialLegs []dialLeg
	now := time.Now()
	for _, p := range tiers[tierIndex] {
		if err := s.resetStaleCaps(ctx, tx, p.participationID); err != nil {
			return "", false, err
		}
		if !withinSchedule(p.schedule, now) {
			continue
		}
		// Resolve destination + price (RTB for dynamic targets).
		destination := p.destination
		price := resolvePrice(basePrice, p.rateOverride, nil)
		if p.targetType == "dynamic" {
			accepted, dest, bid := s.pingRTB(ctx, tx, call, &p, settings, caller(call))
			if !accepted || dest == "" {
				continue
			}
			destination = dest
			price = resolvePrice(basePrice, p.rateOverride, bid)
		}
		if destination == "" {
			continue
		}
		// Balance + cap gating.
		if ok, err := s.buyerHasBalance(ctx, tx, p.buyerID, price); err != nil {
			return "", false, err
		} else if !ok {
			continue
		}
		if multiBuyer {
			if !withinParticipationCaps(p) {
				continue
			}
		} else {
			if err := contracts.CheckCap(ctx, tx, *call.ContractID, 0); err != nil {
				continue
			}
		}
		legID, err := s.insertLeg(ctx, tx, call.ID, p.participationID, p.buyerID, tierIndex+1, destination, price)
		if err != nil {
			return "", false, err
		}
		if err := s.adjustConcurrency(ctx, tx, p.participationID, 1); err != nil {
			return "", false, err
		}
		dialLegs = append(dialLegs, dialLeg{LegID: legID, Destination: destination})
	}
	if len(dialLegs) == 0 {
		// Try the next tier within the same request (skip empty tiers).
		return s.dialTier(ctx, tx, call, trackingNumber, settings, tierIndex+1)
	}
	callerID := callerIDFor(call, trackingNumber, settings)
	return s.twiMLDial(call.ID, dialLegs, callerID, settings.TierTimeoutSec, settings.RecordingEnabled, tierIndex+1), true, nil
}

func caller(call *Call) string {
	if call.CallerNumber != nil {
		return *call.CallerNumber
	}
	return ""
}

// callerIDFor returns the outbound caller ID for the dial. Masking uses the
// tracking number; otherwise the real caller is passed.
func callerIDFor(call *Call, trackingNumber string, settings *CallSettings) string {
	if settings.MaskCallerID {
		return trackingNumber
	}
	if call.CallerNumber != nil {
		return *call.CallerNumber
	}
	return ""
}

func (s *Service) insertLeg(ctx context.Context, q database.Querier, callID, participationID, buyerID int64, tier int, destination string, rate float64) (int64, error) {
	var id int64
	err := q.QueryRow(ctx,
		`INSERT INTO call_legs(call_id, participation_id, buyer_id, tier, destination_number, leg_status, rate)
		 VALUES ($1,$2,$3,$4,$5,'dialing',$6) RETURNING id`,
		callID, participationID, buyerID, tier, destination, rate).Scan(&id)
	return id, err
}

func (s *Service) loadCallParticipants(ctx context.Context, q database.Querier, contractID int64) ([]callParticipant, error) {
	rows, err := q.Query(ctx,
		`SELECT p.id, p.buyer_id, t.priority, t.weight, t.target_type,
		        COALESCE(t.destination_number,''), COALESCE(t.rtb_endpoint,''), t.rtb_headers,
		        t.rate_override::float8, t.schedule, t.daily_cap, t.monthly_cap, t.concurrency_cap,
		        t.calls_today, t.calls_this_month, t.concurrent_calls
		 FROM contract_participations p
		 JOIN participation_call_targets t ON t.participation_id = p.id
		 WHERE p.contract_id = $1 AND p.status = 'active'
		 ORDER BY t.priority, t.id`, contractID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []callParticipant
	for rows.Next() {
		var p callParticipant
		var rate *float64
		if err := rows.Scan(&p.participationID, &p.buyerID, &p.priority, &p.weight, &p.targetType,
			&p.destination, &p.rtbEndpoint, &p.rtbHeadersEnc, &rate, &p.schedule,
			&p.dailyCap, &p.monthlyCap, &p.concurrencyCap,
			&p.callsToday, &p.callsThisMonth, &p.concurrent); err != nil {
			return nil, err
		}
		p.rateOverride = rate
		if targetConfiguredStrings(p.targetType, p.destination, p.rtbEndpoint) {
			out = append(out, p)
		}
	}
	return out, rows.Err()
}

func targetConfiguredStrings(targetType, dest, rtb string) bool {
	if targetType == "dynamic" {
		return strings.TrimSpace(rtb) != ""
	}
	return strings.TrimSpace(dest) != ""
}

func countConfigured(parts []callParticipant) int { return len(parts) }

// groupTiers splits participants (pre-sorted by priority) into tiers of equal priority.
func groupTiers(parts []callParticipant) [][]callParticipant {
	if len(parts) == 0 {
		return nil
	}
	sort.SliceStable(parts, func(i, j int) bool { return parts[i].priority < parts[j].priority })
	var tiers [][]callParticipant
	cur := []callParticipant{parts[0]}
	for _, p := range parts[1:] {
		if p.priority == cur[0].priority {
			cur = append(cur, p)
		} else {
			tiers = append(tiers, cur)
			cur = []callParticipant{p}
		}
	}
	return append(tiers, cur)
}

func withinParticipationCaps(p callParticipant) bool {
	if p.dailyCap != nil && p.callsToday >= *p.dailyCap {
		return false
	}
	if p.monthlyCap != nil && p.callsThisMonth >= *p.monthlyCap {
		return false
	}
	if p.concurrencyCap != nil && p.concurrent >= *p.concurrencyCap {
		return false
	}
	return true
}

func (s *Service) buyerHasBalance(ctx context.Context, q database.Querier, buyerID int64, price float64) (bool, error) {
	if price <= 0 {
		return true, nil
	}
	var balance float64
	err := q.QueryRow(ctx, `SELECT COALESCE(balance,0)::float8 FROM buyer_balances WHERE buyer_id=$1`, buyerID).Scan(&balance)
	if err != nil {
		// No balance row yet → treat as zero.
		return false, nil
	}
	return balance >= price, nil
}

// pingRTB calls a dynamic target's endpoint and logs the result. Returns
// (accepted, destination, bid).
func (s *Service) pingRTB(ctx context.Context, q database.Querier, call *Call, p *callParticipant, settings *CallSettings, callerNum string) (bool, string, *float64) {
	headers := map[string]string{}
	if len(p.rtbHeadersEnc) > 0 {
		if h, err := s.decryptHeaders(p.rtbHeadersEnc); err == nil {
			headers = h
		}
	}
	form := url.Values{}
	if settings.ExposeCallerID {
		form.Set("caller_id", callerNum)
	}
	form.Set("call_id", call.PublicID)
	if call.CallerState != nil {
		form.Set("state", *call.CallerState)
	}
	status, body, err := postForm(ctx, p.rtbEndpoint, headers, form)
	ping := RTBPing{Endpoint: p.rtbEndpoint, ParticipationID: &p.participationID}
	if err != nil {
		reason := err.Error()
		ping.Reason = &reason
		s.logPing(ctx, q, call.ID, ping)
		return false, "", nil
	}
	ping.ResponseStatus = &status
	bodyStr := string(body)
	ping.ResponseBody = &bodyStr
	var resp struct {
		Accept      bool    `json:"accept"`
		Bid         float64 `json:"bid"`
		Destination string  `json:"destination_number"`
	}
	_ = json.Unmarshal(body, &resp)
	if status < 200 || status >= 300 || !resp.Accept || resp.Destination == "" {
		reason := "rejected"
		ping.Reason = &reason
		s.logPing(ctx, q, call.ID, ping)
		return false, "", nil
	}
	ping.Accepted = true
	ping.BidAmount = &resp.Bid
	ping.DestinationNumber = &resp.Destination
	s.logPing(ctx, q, call.ID, ping)
	bid := resp.Bid
	return true, resp.Destination, &bid
}

func (s *Service) logPing(ctx context.Context, q database.Querier, callID int64, p RTBPing) {
	_, _ = q.Exec(ctx,
		`INSERT INTO rtb_pings(call_id, participation_id, endpoint, accepted, bid_amount, destination_number, response_status, response_body, reason)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)`,
		callID, p.ParticipationID, p.Endpoint, p.Accepted, p.BidAmount, p.DestinationNumber, p.ResponseStatus, p.ResponseBody, p.Reason)
}

func (s *Service) applyPreloadFieldMap(ctx context.Context, tx database.Querier, sourceID, publisherID, leadID int64, authorName string, flat map[string]any) error {
	maps, err := routing.SourceFieldMap(ctx, tx, sourceID)
	if err != nil {
		return err
	}
	for _, m := range maps {
		v, ok := flat[m.SourceKey]
		if !ok {
			continue
		}
		if m.TargetType == "builtin" && m.BuiltinField != nil {
			if *m.BuiltinField == "note" {
				if err := s.leads.AddInboundNoteFromValue(ctx, tx, leadID, authorName, v); err != nil {
					return err
				}
				continue
			}
			if err := leads.ApplyMappedField(ctx, tx, s.leads, publisherID, leadID, *m.BuiltinField, v); err != nil {
				return err
			}
		} else if m.TargetType == "custom" && m.CustomFieldID != nil {
			valJSON, _ := json.Marshal(v)
			if err := s.leads.UpsertCustomValue(ctx, tx, leadID, *m.CustomFieldID, valJSON); err != nil {
				return err
			}
		}
	}
	return nil
}

// withinSchedule evaluates an optional daypart grid. Empty schedule = always on.
// schedule shape: {"tz":"America/New_York","mon":{"start":"09:00","end":"17:00"}, ...}
func withinSchedule(schedule json.RawMessage, now time.Time) bool {
	if len(schedule) == 0 || string(schedule) == "{}" || string(schedule) == "null" {
		return true
	}
	var grid map[string]json.RawMessage
	if err := json.Unmarshal(schedule, &grid); err != nil {
		return true
	}
	loc := time.UTC
	if tzRaw, ok := grid["tz"]; ok {
		var tz string
		if json.Unmarshal(tzRaw, &tz) == nil && tz != "" {
			if l, err := time.LoadLocation(tz); err == nil {
				loc = l
			}
		}
	}
	t := now.In(loc)
	day := strings.ToLower(t.Weekday().String()[:3]) // mon, tue, ...
	dayRaw, ok := grid[day]
	if !ok {
		return true // no rule for this day → open
	}
	var window struct {
		Start string `json:"start"`
		End   string `json:"end"`
	}
	if json.Unmarshal(dayRaw, &window) != nil || window.Start == "" || window.End == "" {
		return true
	}
	cur := t.Format("15:04")
	return cur >= window.Start && cur <= window.End
}
