package calls

import (
	"context"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/echayko/leadrula/backend/internal/auth"
	"github.com/echayko/leadrula/backend/pkg/httpx"
	"github.com/go-chi/chi/v5"
)

type Handler struct{ svc *Service }

func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

var validDispositions = map[string]bool{
	"converted": true, "not_interested": true, "callback": true, "wrong_number": true, "no_answer": true,
}

// ── Route registration ────────────────────────────────────────────

// RegisterPublicRoutes mounts the API-key authenticated preload + read endpoints.
func (h *Handler) RegisterPublicRoutes(r chi.Router) {
	r.Post("/api/v1/calls/preload", h.preload)
	r.Get("/api/v1/calls/{public_id}", h.publicGetCall)
}

func (h *Handler) RegisterPublisher(r chi.Router) {
	r.Get("/contracts/{id}/call-settings", h.getCallSettings)
	r.Patch("/contracts/{id}/call-settings", h.putCallSettings)
	r.Get("/participations/{id}/call-target", h.getCallTargetPublisher)
	r.Patch("/participations/{id}/call-target", h.putCallTargetPublisher)
	r.Get("/calls", h.listCalls)
	r.Get("/calls/{id}", h.getCall)
	r.Get("/calls/{id}/recording", h.publisherRecording)
}

func (h *Handler) RegisterBuyer(r chi.Router) {
	r.Get("/participations/{id}/call-target", h.getCallTargetBuyer)
	r.Patch("/participations/{id}/call-target", h.putCallTargetBuyer)
	r.Get("/calls", h.listCallsBuyer)
	// Static path must be registered before the `{id}` param route.
	r.Get("/calls/by-lead/{lead_id}", h.buyerCallByLead)
	r.Get("/calls/{id}", h.getCallBuyer)
	r.Post("/calls/{id}/disposition", h.setDisposition)
	r.Get("/calls/{id}/recording", h.buyerRecording)
}

// ── Public: preload + call read ───────────────────────────────────

func (h *Handler) preload(w http.ResponseWriter, r *http.Request) {
	acct := auth.APIKeyAccountFromContext(r.Context())
	if acct == nil || acct.AccountType != "publisher" {
		httpx.Err(w, http.StatusForbidden, httpx.CodeForbidden, "publisher API key required")
		return
	}
	var body struct {
		Source      string         `json:"source"`
		CallerPhone string         `json:"caller_phone"`
		Payload     map[string]any `json:"payload"`
	}
	if !httpx.DecodeJSON(w, r, &body) {
		return
	}
	res, err := h.svc.CreatePreload(r.Context(), acct.AccountID, body.Source, body.CallerPhone, body.Payload)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, res)
}

func (h *Handler) publicGetCall(w http.ResponseWriter, r *http.Request) {
	acct := auth.APIKeyAccountFromContext(r.Context())
	if acct == nil {
		httpx.Err(w, http.StatusForbidden, httpx.CodeForbidden, "API key required")
		return
	}
	call, err := h.svc.getCallByPublicID(r.Context(), h.svc.pool, chi.URLParam(r, "public_id"))
	if err != nil {
		httpx.WriteError(w, httpx.NotFound("call not found"))
		return
	}
	if call.PublisherID != acct.AccountID {
		httpx.WriteError(w, httpx.NotFound("call not found"))
		return
	}
	httpx.JSON(w, http.StatusOK, call)
}

// ── Publisher: call settings ──────────────────────────────────────

func (h *Handler) getCallSettings(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	contractID := pathInt(r, "id")
	if err := h.svc.requireContractOwner(r.Context(), p.AccountID, contractID); err != nil {
		httpx.WriteError(w, err)
		return
	}
	cs, err := h.svc.GetCallSettings(r.Context(), h.svc.pool, contractID)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, cs)
}

func (h *Handler) putCallSettings(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	var cs CallSettings
	if !httpx.DecodeJSON(w, r, &cs) {
		return
	}
	cs.ContractID = pathInt(r, "id")
	saved, err := h.svc.UpsertCallSettings(r.Context(), p.AccountID, cs)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, saved)
}

// ── Call targets (buyer destination + publisher routing knobs) ────

func (h *Handler) getCallTargetPublisher(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	partID := pathInt(r, "id")
	if err := h.svc.requireParticipationPublisher(r.Context(), p.AccountID, partID); err != nil {
		httpx.WriteError(w, err)
		return
	}
	h.respondTarget(w, r, partID)
}

func (h *Handler) putCallTargetPublisher(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	var t CallTarget
	if !httpx.DecodeJSON(w, r, &t) {
		return
	}
	saved, err := h.svc.UpsertCallTargetPublisher(r.Context(), p.AccountID, pathInt(r, "id"), t)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, saved)
}

func (h *Handler) getCallTargetBuyer(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	partID := pathInt(r, "id")
	if err := h.svc.requireParticipationBuyer(r.Context(), p.AccountID, partID); err != nil {
		httpx.WriteError(w, err)
		return
	}
	h.respondTarget(w, r, partID)
}

func (h *Handler) putCallTargetBuyer(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	var t CallTarget
	if !httpx.DecodeJSON(w, r, &t) {
		return
	}
	saved, err := h.svc.UpsertCallTargetBuyer(r.Context(), p.AccountID, pathInt(r, "id"), t)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, saved)
}

func (h *Handler) respondTarget(w http.ResponseWriter, r *http.Request, partID int64) {
	t, err := h.svc.GetCallTarget(r.Context(), h.svc.pool, partID)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	if t == nil {
		httpx.JSON(w, http.StatusOK, map[string]any{"participation_id": partID, "configured": false})
		return
	}
	httpx.JSON(w, http.StatusOK, t)
}

// ── Publisher: call log + detail ──────────────────────────────────

func (h *Handler) listCalls(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	f := CallLogFilter{
		Status:     r.URL.Query().Get("status"),
		ContractID: queryInt(r, "contract_id"),
		LeadID:     queryInt(r, "lead_id"),
		Q:          strings.TrimSpace(r.URL.Query().Get("q")),
		Limit:      int(queryInt(r, "limit")),
	}
	if b := r.URL.Query().Get("billable"); b != "" {
		v := b == "true"
		f.Billable = &v
	}
	items, err := h.svc.ListCalls(r.Context(), p.AccountID, f)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *Handler) getCall(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	call, err := h.svc.CallDetailForPublisher(r.Context(), p.AccountID, pathInt(r, "id"))
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, call)
}

// ── Buyer: call log + detail (billable winning calls only) ────────

func (h *Handler) listCallsBuyer(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	f := CallLogFilter{
		Status:     r.URL.Query().Get("status"),
		ContractID: queryInt(r, "contract_id"),
		LeadID:     queryInt(r, "lead_id"),
		Q:          strings.TrimSpace(r.URL.Query().Get("q")),
		Limit:      int(queryInt(r, "limit")),
	}
	items, err := h.svc.ListCallsForBuyer(r.Context(), p.AccountID, f)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *Handler) getCallBuyer(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	call, err := h.svc.CallDetailForBuyer(r.Context(), p.AccountID, pathInt(r, "id"))
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, call)
}

// ── Buyer: call for an assigned lead ──────────────────────────────

func (h *Handler) buyerCallByLead(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	leadID := pathInt(r, "lead_id")
	call, err := h.svc.BuyerCallForLead(r.Context(), p.AccountID, leadID)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, call)
}

// ── Buyer: disposition ────────────────────────────────────────────

func (h *Handler) setDisposition(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	var body struct {
		Disposition string `json:"disposition"`
		Note        string `json:"note"`
	}
	if !httpx.DecodeJSON(w, r, &body) {
		return
	}
	if !validDispositions[body.Disposition] {
		httpx.Err(w, http.StatusBadRequest, httpx.CodeValidation, "invalid disposition")
		return
	}
	if err := h.svc.SetDisposition(r.Context(), p.AccountID, pathInt(r, "id"), body.Disposition, body.Note); err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"ok": true})
}

// ── Recording proxy ───────────────────────────────────────────────

func (h *Handler) publisherRecording(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	h.streamRecording(w, r, p.AccountID, "publisher")
}

func (h *Handler) buyerRecording(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	h.streamRecording(w, r, p.AccountID, "buyer")
}

func (h *Handler) streamRecording(w http.ResponseWriter, r *http.Request, accountID int64, role string) {
	resp, err := h.svc.RecordingStream(r.Context(), accountID, role, pathInt(r, "id"))
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	defer resp.Body.Close()
	w.Header().Set("Content-Type", "audio/mpeg")
	w.WriteHeader(resp.StatusCode)
	_, _ = io.Copy(w, resp.Body)
}

// ── Service methods backing the handlers ──────────────────────────

type CallLogFilter struct {
	Status     string
	ContractID int64
	LeadID     int64
	Billable   *bool
	Q          string
	Limit      int
}

func (s *Service) ListCalls(ctx context.Context, publisherID int64, f CallLogFilter) ([]Call, error) {
	where := "c.publisher_id = $1"
	args := []any{publisherID}
	add := func(v any) string {
		args = append(args, v)
		return "$" + strconv.Itoa(len(args))
	}
	if f.Status != "" {
		where += " AND c.status = " + add(f.Status)
	}
	if f.ContractID != 0 {
		where += " AND c.contract_id = " + add(f.ContractID)
	}
	if f.LeadID != 0 {
		where += " AND c.lead_id = " + add(f.LeadID)
	}
	if f.Billable != nil {
		where += " AND c.billable = " + add(*f.Billable)
	}
	where = appendCallSearch(where, f.Q, "publisher", add)
	rows, err := s.pool.Query(ctx,
		`SELECT `+callColsPrefixed+`, ct.name, wa.name, `+leadNameExpr+`
		 FROM calls c
		 LEFT JOIN contracts ct ON ct.id = c.contract_id
		 LEFT JOIN contract_participations wp ON wp.id = c.winner_participation_id
		 LEFT JOIN accounts wa ON wa.id = wp.buyer_id
		 LEFT JOIN leads l ON l.id = c.lead_id
		 WHERE `+where+`
		 ORDER BY c.created_at DESC LIMIT `+add(callLimit(f.Limit)), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Call{}
	for rows.Next() {
		var c Call
		var name, winner, leadName *string
		if err := rows.Scan(&c.ID, &c.PublicID, &c.PublisherID, &c.SourceID, &c.ContractID, &c.LeadID,
			&c.WinnerParticipationID, &c.TwilioCallSID, &c.CallerNumber, &c.CallerState, &c.TrackingNumber,
			&c.Status, &c.Disposition, &c.DispositionNote, &c.Billable, &c.DurationSec, &c.BillableDurationSec,
			&c.PriceCents, &c.RecordingURL, &c.ConnectedAt, &c.EndedAt, &c.CreatedAt,
			&name, &winner, &leadName); err != nil {
			return nil, err
		}
		c.ContractName = name
		c.WinnerName = winner
		c.LeadName = leadName
		out = append(out, c)
	}
	return out, rows.Err()
}

// leadNameExpr builds a display name for a linked lead, falling back through
// phone/email/ID when name parts are empty.
const leadNameExpr = `COALESCE(
	NULLIF(trim(coalesce(l.first_name,'') || ' ' || coalesce(l.last_name,'')), ''),
	NULLIF(l.phone, ''),
	l.email::text)`

// ListCallsForBuyer returns billable calls won by this buyer, enriched with the
// publisher and linked lead name. Caller numbers are masked.
func (s *Service) ListCallsForBuyer(ctx context.Context, buyerID int64, f CallLogFilter) ([]Call, error) {
	where := "wp.buyer_id = $1 AND c.billable"
	args := []any{buyerID}
	add := func(v any) string {
		args = append(args, v)
		return "$" + strconv.Itoa(len(args))
	}
	if f.Status != "" {
		where += " AND c.status = " + add(f.Status)
	}
	if f.ContractID != 0 {
		where += " AND c.contract_id = " + add(f.ContractID)
	}
	if f.LeadID != 0 {
		where += " AND c.lead_id = " + add(f.LeadID)
	}
	where = appendCallSearch(where, f.Q, "buyer", add)
	rows, err := s.pool.Query(ctx,
		`SELECT `+callColsPrefixed+`, ct.name, pa.name, `+leadNameExpr+`, l.phone
		 FROM calls c
		 JOIN contract_participations wp ON wp.id = c.winner_participation_id
		 LEFT JOIN contracts ct ON ct.id = c.contract_id
		 LEFT JOIN accounts pa ON pa.id = c.publisher_id
		 LEFT JOIN leads l ON l.id = c.lead_id
		 WHERE `+where+`
		 ORDER BY c.created_at DESC LIMIT `+add(callLimit(f.Limit)), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Call{}
	for rows.Next() {
		var c Call
		var contractName, publisherName, leadName *string
		if err := rows.Scan(&c.ID, &c.PublicID, &c.PublisherID, &c.SourceID, &c.ContractID, &c.LeadID,
			&c.WinnerParticipationID, &c.TwilioCallSID, &c.CallerNumber, &c.CallerState, &c.TrackingNumber,
			&c.Status, &c.Disposition, &c.DispositionNote, &c.Billable, &c.DurationSec, &c.BillableDurationSec,
			&c.PriceCents, &c.RecordingURL, &c.ConnectedAt, &c.EndedAt, &c.CreatedAt,
			&contractName, &publisherName, &leadName, &c.LeadPhone); err != nil {
			return nil, err
		}
		c.ContractName = contractName
		c.PublisherName = publisherName
		c.LeadName = leadName
		c.CallerNumber = maskNumber(c.CallerNumber)
		out = append(out, c)
	}
	return out, rows.Err()
}

// CallDetailForBuyer returns a billable call won by this buyer, with legs.
// RTB pings are omitted from the buyer view.
func (s *Service) CallDetailForBuyer(ctx context.Context, buyerID, callID int64) (*Call, error) {
	var winnerBuyer *int64
	var billable bool
	if err := s.pool.QueryRow(ctx,
		`SELECT c.billable, wp.buyer_id
		 FROM calls c
		 LEFT JOIN contract_participations wp ON wp.id = c.winner_participation_id
		 WHERE c.id=$1`, callID).Scan(&billable, &winnerBuyer); err != nil {
		return nil, httpx.NotFound("call not found")
	}
	if !billable || winnerBuyer == nil || *winnerBuyer != buyerID {
		return nil, httpx.NotFound("call not found")
	}
	call, err := s.getCallByID(ctx, s.pool, callID)
	if err != nil {
		return nil, httpx.NotFound("call not found")
	}
	legs, err := s.legsForCall(ctx, s.pool, callID)
	if err != nil {
		return nil, err
	}
	call.Legs = legs
	call.CallerNumber = maskNumber(call.CallerNumber)
	return call, nil
}

func (s *Service) CallDetailForPublisher(ctx context.Context, publisherID, callID int64) (*Call, error) {
	call, err := s.getCallByID(ctx, s.pool, callID)
	if err != nil {
		return nil, httpx.NotFound("call not found")
	}
	if call.PublisherID != publisherID {
		return nil, httpx.NotFound("call not found")
	}
	legs, err := s.legsForCall(ctx, s.pool, callID)
	if err != nil {
		return nil, err
	}
	pings, err := s.pingsForCall(ctx, s.pool, callID)
	if err != nil {
		return nil, err
	}
	call.Legs = legs
	call.Pings = pings
	return call, nil
}

// BuyerCallForLead returns the billable call assigned to this buyer for a lead.
func (s *Service) BuyerCallForLead(ctx context.Context, buyerID, leadID int64) (*Call, error) {
	var callID int64
	err := s.pool.QueryRow(ctx,
		`SELECT c.id FROM calls c
		 JOIN contract_participations wp ON wp.id = c.winner_participation_id
		 WHERE c.lead_id=$1 AND c.billable AND wp.buyer_id=$2
		 ORDER BY c.created_at DESC LIMIT 1`, leadID, buyerID).Scan(&callID)
	if err != nil {
		return nil, httpx.NotFound("call not found")
	}
	call, err := s.getCallByID(ctx, s.pool, callID)
	if err != nil {
		return nil, httpx.NotFound("call not found")
	}
	// Buyer never sees the caller's real number.
	call.CallerNumber = maskNumber(call.CallerNumber)
	return call, nil
}

// maskNumber keeps only the last 4 digits visible (e.g. "+1 (***) ***-1234").
func maskNumber(n *string) *string {
	if n == nil || len(*n) < 4 {
		return n
	}
	masked := "***-" + (*n)[len(*n)-4:]
	return &masked
}

func (s *Service) SetDisposition(ctx context.Context, buyerID, callID int64, disposition, note string) error {
	// Buyer may only set disposition on a billable call assigned to them.
	var winnerBuyer *int64
	var billable bool
	err := s.pool.QueryRow(ctx,
		`SELECT c.billable, wp.buyer_id
		 FROM calls c
		 LEFT JOIN contract_participations wp ON wp.id = c.winner_participation_id
		 WHERE c.id=$1`, callID).Scan(&billable, &winnerBuyer)
	if err != nil {
		return httpx.NotFound("call not found")
	}
	if !billable || winnerBuyer == nil || *winnerBuyer != buyerID {
		return httpx.NotFound("call not found")
	}
	_, err = s.pool.Exec(ctx,
		`UPDATE calls SET disposition=$2, disposition_note=$3, updated_at=now() WHERE id=$1`,
		callID, disposition, strings.TrimSpace(note))
	return err
}

// RecordingStream returns a Twilio recording stream for an authorized viewer.
// Buyer access requires a billable call assigned to them; publisher always.
func (s *Service) RecordingStream(ctx context.Context, accountID int64, role string, callID int64) (*http.Response, error) {
	var publisherID int64
	var billable bool
	var recURL, accountSID *string
	var connID *int64
	var winnerBuyer *int64
	err := s.pool.QueryRow(ctx,
		`SELECT c.publisher_id, c.billable, c.recording_url, rs.twilio_sid, rs.integration_connection_id, wp.buyer_id
		 FROM calls c
		 LEFT JOIN routing_sources rs ON rs.id = c.source_id
		 LEFT JOIN contract_participations wp ON wp.id = c.winner_participation_id
		 WHERE c.id=$1`, callID).Scan(&publisherID, &billable, &recURL, &accountSID, &connID, &winnerBuyer)
	if err != nil {
		return nil, httpx.NotFound("call not found")
	}
	if recURL == nil || *recURL == "" {
		return nil, httpx.NotFound("recording not available")
	}
	switch role {
	case "publisher":
		if publisherID != accountID {
			return nil, httpx.NotFound("call not found")
		}
	case "buyer":
		if !billable || winnerBuyer == nil || *winnerBuyer != accountID {
			return nil, httpx.NotFound("call not found")
		}
	default:
		return nil, httpx.NotFound("call not found")
	}
	token := s.twilioAuthToken(ctx, publisherID, connID)
	sid := ""
	if accountSID != nil {
		sid = *accountSID
	}
	return fetchRecording(ctx, sid, token, *recURL)
}

// scanCallRow / scanCallWith helpers for the enriched list query.
const callColsPrefixed = `c.id, c.public_id::text, c.publisher_id, c.source_id, c.contract_id, c.lead_id,
	c.winner_participation_id, c.twilio_call_sid, c.caller_number, c.caller_state, c.tracking_number,
	c.status, c.disposition, c.disposition_note, c.billable, c.duration_sec, c.billable_duration_sec,
	c.price_cents, c.recording_url, c.connected_at, c.ended_at, c.created_at`

func pathInt(r *http.Request, key string) int64 {
	v, _ := strconv.ParseInt(chi.URLParam(r, key), 10, 64)
	return v
}

func queryInt(r *http.Request, key string) int64 {
	v, _ := strconv.ParseInt(r.URL.Query().Get(key), 10, 64)
	return v
}

func callLimit(n int) int {
	if n <= 0 || n > 500 {
		return 500
	}
	return n
}

// appendCallSearch adds a free-text filter over linked lead, caller number, and
// counterparty/contract names. Publisher searches the winning buyer (wa); buyer
// searches the publisher (pa). add binds each value and returns its placeholder.
func appendCallSearch(where, term, role string, add func(any) string) string {
	term = strings.TrimSpace(term)
	if term == "" {
		return where
	}
	p := add("%" + term + "%")
	leadFields := `l.first_name ILIKE ` + p + ` OR
		l.last_name ILIKE ` + p + ` OR
		(coalesce(l.first_name,'') || ' ' || coalesce(l.last_name,'')) ILIKE ` + p + ` OR
		l.phone ILIKE ` + p + ` OR
		l.email ILIKE ` + p + ` OR
		l.public_id::text ILIKE ` + p + ` OR
		c.public_id::text ILIKE ` + p
	counterparty := "wa.name"
	if role == "buyer" {
		counterparty = "pa.name"
	}
	return where + ` AND (
		` + leadFields + ` OR
		c.caller_number ILIKE ` + p + ` OR
		ct.name ILIKE ` + p + ` OR
		` + counterparty + ` ILIKE ` + p + `
	)`
}
