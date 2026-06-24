package calls

import (
	"context"
	"log"
	"net/http"
	"strconv"

	"github.com/echayko/leadrula/backend/internal/routing"
	"github.com/go-chi/chi/v5"
)

// RegisterWebhookRoutes mounts the public Twilio webhook endpoints. Each handler
// resolves the publisher's auth token (tracking number -> source -> integration)
// and validates the Twilio signature before any side effect.
func (s *Service) RegisterWebhookRoutes(r chi.Router) {
	r.Post("/webhooks/twilio/voice", s.webhookVoice)
	r.Post("/webhooks/twilio/continue", s.webhookContinue)
	r.Post("/webhooks/twilio/leg-status", s.webhookLegStatus)
	r.Post("/webhooks/twilio/recording", s.webhookRecording)
}

func writeTwiML(w http.ResponseWriter, twiml string) {
	w.Header().Set("Content-Type", "text/xml")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(twiml))
}

func (s *Service) webhookVoice(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		writeTwiML(w, twiMLHangup())
		return
	}
	to := r.PostForm.Get("To")
	from := r.PostForm.Get("From")
	sid := r.PostForm.Get("CallSid")
	token := r.URL.Query().Get("preload_token")

	src, err := routing.SourceByTrackingNumber(r.Context(), s.pool, to)
	if err != nil {
		writeTwiML(w, twiMLHangup())
		return
	}
	if src == nil {
		writeTwiML(w, twiMLHold())
		return
	}
	if !s.verifySignature(r, src.PublisherID, src.IntegrationConnectionID) {
		w.WriteHeader(http.StatusForbidden)
		return
	}

	twiml, err := s.HandleInbound(r.Context(), to, from, sid, token)
	if err != nil {
		log.Printf("calls: inbound error: %v", err)
		writeTwiML(w, twiMLHold())
		return
	}
	writeTwiML(w, twiml)
}

func (s *Service) webhookContinue(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		writeTwiML(w, twiMLHangup())
		return
	}
	callID, _ := strconv.ParseInt(r.URL.Query().Get("call"), 10, 64)
	tier, _ := strconv.Atoi(r.URL.Query().Get("tier"))
	if callID == 0 {
		writeTwiML(w, twiMLHangup())
		return
	}
	if !s.verifySignatureForCall(r, callID) {
		w.WriteHeader(http.StatusForbidden)
		return
	}
	dialStatus := r.PostForm.Get("DialCallStatus")
	dialSID := r.PostForm.Get("DialCallSid")
	dialDuration, _ := strconv.Atoi(r.PostForm.Get("DialCallDuration"))

	twiml, err := s.ContinueWaterfall(r.Context(), callID, tier, dialStatus, dialDuration, dialSID)
	if err != nil {
		log.Printf("calls: continue error: %v", err)
		writeTwiML(w, twiMLHangup())
		return
	}
	writeTwiML(w, twiml)
}

func (s *Service) webhookLegStatus(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	legID, _ := strconv.ParseInt(r.URL.Query().Get("leg"), 10, 64)
	if legID == 0 {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	if !s.verifySignatureForLeg(r, legID) {
		w.WriteHeader(http.StatusForbidden)
		return
	}
	callStatus := r.PostForm.Get("CallStatus")
	childSID := r.PostForm.Get("CallSid")
	duration, _ := strconv.Atoi(r.PostForm.Get("CallDuration"))
	if err := s.HandleLegStatus(r.Context(), legID, callStatus, childSID, duration); err != nil {
		log.Printf("calls: leg status error: %v", err)
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Service) webhookRecording(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	callID, _ := strconv.ParseInt(r.URL.Query().Get("call"), 10, 64)
	if callID == 0 {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	if !s.verifySignatureForCall(r, callID) {
		w.WriteHeader(http.StatusForbidden)
		return
	}
	recURL := r.PostForm.Get("RecordingUrl")
	duration, _ := strconv.Atoi(r.PostForm.Get("RecordingDuration"))
	if recURL != "" {
		if err := s.HandleRecording(r.Context(), callID, recURL, duration); err != nil {
			log.Printf("calls: recording error: %v", err)
		}
	}
	w.WriteHeader(http.StatusNoContent)
}

// verifySignature validates the Twilio signature using the publisher's auth token.
// When no token is configured (e.g. local dev), validation is skipped.
func (s *Service) verifySignature(r *http.Request, publisherID int64, connID *int64) bool {
	token := s.twilioAuthToken(r.Context(), publisherID, connID)
	if token == "" {
		return true
	}
	fullURL := s.webhookBase + r.URL.RequestURI()
	return validateTwilioSignature(token, fullURL, r.PostForm, r.Header.Get("X-Twilio-Signature"))
}

func (s *Service) verifySignatureForCall(r *http.Request, callID int64) bool {
	publisherID, connID := s.callCredsRef(r.Context(), callID)
	return s.verifySignature(r, publisherID, connID)
}

func (s *Service) verifySignatureForLeg(r *http.Request, legID int64) bool {
	var callID int64
	if err := s.pool.QueryRow(r.Context(), `SELECT call_id FROM call_legs WHERE id=$1`, legID).Scan(&callID); err != nil {
		return false
	}
	return s.verifySignatureForCall(r, callID)
}

func (s *Service) callCredsRef(ctx context.Context, callID int64) (int64, *int64) {
	var publisherID int64
	var connID *int64
	_ = s.pool.QueryRow(ctx,
		`SELECT c.publisher_id, rs.integration_connection_id
		 FROM calls c LEFT JOIN routing_sources rs ON rs.id = c.source_id
		 WHERE c.id=$1`, callID).Scan(&publisherID, &connID)
	return publisherID, connID
}

func (s *Service) twilioAuthToken(ctx context.Context, publisherID int64, connID *int64) string {
	if connID == nil || s.creds == nil {
		return ""
	}
	creds, err := s.creds.DecryptedCredentials(ctx, publisherID, *connID)
	if err != nil {
		return ""
	}
	return creds["auth_token"]
}
