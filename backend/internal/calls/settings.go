package calls

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"time"

	"github.com/echayko/leadrula/backend/internal/database"
	"github.com/echayko/leadrula/backend/pkg/httpx"
	"github.com/jackc/pgx/v5"
)

// ── Per-contract call settings ────────────────────────────────────

type CallSettings struct {
	ContractID           int64    `json:"contract_id"`
	DurationThresholdSec int      `json:"duration_threshold_sec"`
	TierTimeoutSec       int      `json:"tier_timeout_sec"`
	DuplicateWindowHours int      `json:"duplicate_window_hours"`
	MaskCallerID         bool     `json:"mask_caller_id"`
	ExposeCallerID       bool     `json:"expose_caller_id"`
	PassInboundPayload   bool     `json:"pass_inbound_payload"`
	RecordingEnabled     bool     `json:"recording_enabled"`
	Vertical             string   `json:"vertical"`
	AllowedStates        []string `json:"allowed_states"`
	CallerGeoMode        string   `json:"caller_geo_mode"`
}

const callSettingsCols = `contract_id, duration_threshold_sec, tier_timeout_sec, duplicate_window_hours,
	mask_caller_id, expose_caller_id, pass_inbound_payload, recording_enabled, vertical, allowed_states, caller_geo_mode`

func scanCallSettings(row pgx.Row) (*CallSettings, error) {
	cs := &CallSettings{}
	err := row.Scan(&cs.ContractID, &cs.DurationThresholdSec, &cs.TierTimeoutSec, &cs.DuplicateWindowHours,
		&cs.MaskCallerID, &cs.ExposeCallerID, &cs.PassInboundPayload, &cs.RecordingEnabled, &cs.Vertical,
		&cs.AllowedStates, &cs.CallerGeoMode)
	if err != nil {
		return nil, err
	}
	return cs, nil
}

// GetCallSettings returns saved settings or sane defaults for a contract.
func (s *Service) GetCallSettings(ctx context.Context, q database.Querier, contractID int64) (*CallSettings, error) {
	cs, err := scanCallSettings(q.QueryRow(ctx, `SELECT `+callSettingsCols+` FROM contract_call_settings WHERE contract_id=$1`, contractID))
	if errors.Is(err, pgx.ErrNoRows) {
		return &CallSettings{
			ContractID:           contractID,
			DurationThresholdSec: 30,
			TierTimeoutSec:       20,
			DuplicateWindowHours: 72,
			ExposeCallerID:       true,
			RecordingEnabled:     true,
			AllowedStates:        []string{},
			CallerGeoMode:        "none",
		}, nil
	}
	if err != nil {
		return nil, err
	}
	if cs.AllowedStates == nil {
		cs.AllowedStates = []string{}
	}
	return cs, nil
}

func validGeoMode(m string) bool {
	return m == "twilio_lookup" || m == "area_code" || m == "none"
}

// UpsertCallSettings saves call settings for a publisher-owned contract.
func (s *Service) UpsertCallSettings(ctx context.Context, publisherID int64, cs CallSettings) (*CallSettings, error) {
	if err := s.requireContractOwner(ctx, publisherID, cs.ContractID); err != nil {
		return nil, err
	}
	if cs.DurationThresholdSec < 0 || cs.TierTimeoutSec <= 0 {
		return nil, httpx.Validation("tier timeout must be positive")
	}
	if cs.CallerGeoMode == "" {
		cs.CallerGeoMode = "none"
	}
	if !validGeoMode(cs.CallerGeoMode) {
		return nil, httpx.Validation("invalid caller_geo_mode")
	}
	if cs.AllowedStates == nil {
		cs.AllowedStates = []string{}
	}
	return scanCallSettings(s.pool.QueryRow(ctx,
		`INSERT INTO contract_call_settings(`+callSettingsCols+`)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)
		 ON CONFLICT (contract_id) DO UPDATE SET
		   duration_threshold_sec=$2, tier_timeout_sec=$3, duplicate_window_hours=$4,
		   mask_caller_id=$5, expose_caller_id=$6, pass_inbound_payload=$7, recording_enabled=$8,
		   vertical=$9, allowed_states=$10, caller_geo_mode=$11, updated_at=now()
		 RETURNING `+callSettingsCols,
		cs.ContractID, cs.DurationThresholdSec, cs.TierTimeoutSec, cs.DuplicateWindowHours,
		cs.MaskCallerID, cs.ExposeCallerID, cs.PassInboundPayload, cs.RecordingEnabled, cs.Vertical,
		cs.AllowedStates, cs.CallerGeoMode))
}

func (s *Service) requireContractOwner(ctx context.Context, publisherID, contractID int64) error {
	var ok bool
	if err := s.pool.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM contracts WHERE id=$1 AND publisher_id=$2 AND deleted_at IS NULL)`,
		contractID, publisherID).Scan(&ok); err != nil {
		return err
	}
	if !ok {
		return httpx.NotFound("contract not found")
	}
	return nil
}

// ── Per-participation call target ─────────────────────────────────

type CallTarget struct {
	ParticipationID   int64             `json:"participation_id"`
	TargetType        string            `json:"target_type"`
	DestinationNumber *string           `json:"destination_number,omitempty"`
	RTBEndpoint       *string           `json:"rtb_endpoint,omitempty"`
	RTBHeaders        map[string]string `json:"rtb_headers,omitempty"`
	Priority          int               `json:"priority"`
	Weight            int               `json:"weight"`
	RateOverride      *float64          `json:"rate_override,omitempty"`
	Schedule          json.RawMessage   `json:"schedule,omitempty"`
	DailyCap          *int              `json:"daily_cap,omitempty"`
	MonthlyCap        *int              `json:"monthly_cap,omitempty"`
	ConcurrencyCap    *int              `json:"concurrency_cap,omitempty"`
	CallsToday        int               `json:"calls_today"`
	CallsThisMonth    int               `json:"calls_this_month"`
	ConcurrentCalls   int               `json:"concurrent_calls"`
	Configured        bool              `json:"configured"`
}

// targetRow is the internal scan shape (RTB headers stay encrypted).
type targetRow struct {
	CallTarget
	rtbHeadersEnc []byte
}

func (s *Service) loadTarget(ctx context.Context, q database.Querier, participationID int64) (*targetRow, error) {
	t := &targetRow{}
	t.ParticipationID = participationID
	err := q.QueryRow(ctx,
		`SELECT target_type, destination_number, rtb_endpoint, rtb_headers, priority, weight,
		        rate_override::float8, schedule, daily_cap, monthly_cap, concurrency_cap,
		        calls_today, calls_this_month, concurrent_calls
		 FROM participation_call_targets WHERE participation_id=$1`, participationID).
		Scan(&t.TargetType, &t.DestinationNumber, &t.RTBEndpoint, &t.rtbHeadersEnc, &t.Priority, &t.Weight,
			&t.RateOverride, &t.Schedule, &t.DailyCap, &t.MonthlyCap, &t.ConcurrencyCap,
			&t.CallsToday, &t.CallsThisMonth, &t.ConcurrentCalls)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	t.Configured = targetConfigured(t.TargetType, t.DestinationNumber, t.RTBEndpoint)
	return t, nil
}

func targetConfigured(targetType string, dest, rtb *string) bool {
	if targetType == "dynamic" {
		return rtb != nil && strings.TrimSpace(*rtb) != ""
	}
	return dest != nil && strings.TrimSpace(*dest) != ""
}

// GetCallTarget returns the target for a participation, decrypting RTB headers
// for the owner view. Returns nil when not configured.
func (s *Service) GetCallTarget(ctx context.Context, q database.Querier, participationID int64) (*CallTarget, error) {
	t, err := s.loadTarget(ctx, q, participationID)
	if err != nil || t == nil {
		return nil, err
	}
	out := t.CallTarget
	if len(t.rtbHeadersEnc) > 0 {
		if headers, err := s.decryptHeaders(t.rtbHeadersEnc); err == nil {
			out.RTBHeaders = headers
		}
	}
	return &out, nil
}

// UpsertCallTargetBuyer saves the buyer-owned routing destination (static or dynamic).
func (s *Service) UpsertCallTargetBuyer(ctx context.Context, buyerID, participationID int64, t CallTarget) (*CallTarget, error) {
	if err := s.requireParticipationBuyer(ctx, buyerID, participationID); err != nil {
		return nil, err
	}
	if t.TargetType != "static" && t.TargetType != "dynamic" {
		return nil, httpx.Validation("target_type must be static or dynamic")
	}
	if t.TargetType == "static" && (t.DestinationNumber == nil || strings.TrimSpace(*t.DestinationNumber) == "") {
		return nil, httpx.Validation("destination_number is required for a static target")
	}
	if t.TargetType == "dynamic" && (t.RTBEndpoint == nil || strings.TrimSpace(*t.RTBEndpoint) == "") {
		return nil, httpx.Validation("rtb_endpoint is required for a dynamic target")
	}
	var headersEnc []byte
	if len(t.RTBHeaders) > 0 {
		enc, err := s.encryptHeaders(t.RTBHeaders)
		if err != nil {
			return nil, err
		}
		headersEnc = enc
	}
	_, err := s.pool.Exec(ctx,
		`INSERT INTO participation_call_targets(participation_id, target_type, destination_number, rtb_endpoint, rtb_headers)
		 VALUES ($1,$2,$3,$4,$5)
		 ON CONFLICT (participation_id) DO UPDATE SET
		   target_type=$2, destination_number=$3, rtb_endpoint=$4,
		   rtb_headers=COALESCE($5, participation_call_targets.rtb_headers), updated_at=now()`,
		participationID, t.TargetType, t.DestinationNumber, t.RTBEndpoint, headersEnc)
	if err != nil {
		return nil, err
	}
	return s.GetCallTarget(ctx, s.pool, participationID)
}

// UpsertCallTargetPublisher saves the publisher-owned routing knobs (priority, weight, schedule, caps).
func (s *Service) UpsertCallTargetPublisher(ctx context.Context, publisherID, participationID int64, t CallTarget) (*CallTarget, error) {
	if err := s.requireParticipationPublisher(ctx, publisherID, participationID); err != nil {
		return nil, err
	}
	if t.Priority <= 0 {
		t.Priority = 1
	}
	if t.Weight < 0 {
		t.Weight = 0
	}
	schedule := t.Schedule
	if len(schedule) == 0 {
		schedule = json.RawMessage(`{}`)
	}
	_, err := s.pool.Exec(ctx,
		`INSERT INTO participation_call_targets(participation_id, priority, weight, rate_override, schedule, daily_cap, monthly_cap, concurrency_cap)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
		 ON CONFLICT (participation_id) DO UPDATE SET
		   priority=$2, weight=$3, rate_override=$4, schedule=$5,
		   daily_cap=$6, monthly_cap=$7, concurrency_cap=$8, updated_at=now()`,
		participationID, t.Priority, t.Weight, t.RateOverride, schedule, t.DailyCap, t.MonthlyCap, t.ConcurrencyCap)
	if err != nil {
		return nil, err
	}
	return s.GetCallTarget(ctx, s.pool, participationID)
}

func (s *Service) requireParticipationBuyer(ctx context.Context, buyerID, participationID int64) error {
	var ok bool
	if err := s.pool.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM contract_participations WHERE id=$1 AND buyer_id=$2)`,
		participationID, buyerID).Scan(&ok); err != nil {
		return err
	}
	if !ok {
		return httpx.NotFound("participation not found")
	}
	return nil
}

func (s *Service) requireParticipationPublisher(ctx context.Context, publisherID, participationID int64) error {
	var ok bool
	if err := s.pool.QueryRow(ctx,
		`SELECT EXISTS(
		   SELECT 1 FROM contract_participations p
		   JOIN contracts c ON c.id = p.contract_id
		   WHERE p.id=$1 AND c.publisher_id=$2)`,
		participationID, publisherID).Scan(&ok); err != nil {
		return err
	}
	if !ok {
		return httpx.NotFound("participation not found")
	}
	return nil
}

// ── Cap window reset + counters ───────────────────────────────────

// resetStaleCaps zeroes daily/monthly counters whose last_cap_reset has rolled
// over. Called lazily at routing and by the nightly worker.
func (s *Service) resetStaleCaps(ctx context.Context, q database.Querier, participationID int64) error {
	today := time.Now().UTC().Format("2006-01-02")
	_, err := q.Exec(ctx,
		`UPDATE participation_call_targets
		 SET calls_today = CASE WHEN last_cap_reset IS DISTINCT FROM $2::date THEN 0 ELSE calls_today END,
		     calls_this_month = CASE WHEN date_trunc('month', COALESCE(last_cap_reset, '1970-01-01'::date))
		                              <> date_trunc('month', $2::date) THEN 0 ELSE calls_this_month END,
		     last_cap_reset = $2::date
		 WHERE participation_id = $1`,
		participationID, today)
	return err
}

// ResetAllStaleCaps is the nightly worker backstop across all targets.
func (s *Service) ResetAllStaleCaps(ctx context.Context) error {
	today := time.Now().UTC().Format("2006-01-02")
	_, err := s.pool.Exec(ctx,
		`UPDATE participation_call_targets
		 SET calls_today = CASE WHEN last_cap_reset IS DISTINCT FROM $1::date THEN 0 ELSE calls_today END,
		     calls_this_month = CASE WHEN date_trunc('month', COALESCE(last_cap_reset, '1970-01-01'::date))
		                              <> date_trunc('month', $1::date) THEN 0 ELSE calls_this_month END,
		     last_cap_reset = $1::date
		 WHERE last_cap_reset IS DISTINCT FROM $1::date`,
		today)
	return err
}

func (s *Service) incrementCallCounters(ctx context.Context, q database.Querier, participationID int64) error {
	_, err := q.Exec(ctx,
		`UPDATE participation_call_targets
		 SET calls_today = calls_today + 1, calls_this_month = calls_this_month + 1, updated_at=now()
		 WHERE participation_id=$1`, participationID)
	return err
}

func (s *Service) adjustConcurrency(ctx context.Context, q database.Querier, participationID int64, delta int) error {
	_, err := q.Exec(ctx,
		`UPDATE participation_call_targets
		 SET concurrent_calls = GREATEST(0, concurrent_calls + $2), updated_at=now()
		 WHERE participation_id=$1`, participationID, delta)
	return err
}

// ── RTB header encryption (AES-GCM, same scheme as integrations) ──

func (s *Service) encryptHeaders(headers map[string]string) ([]byte, error) {
	if len(s.encKey) != 32 {
		return nil, httpx.BusinessRule("encryption key not configured")
	}
	plain, _ := json.Marshal(headers)
	block, err := aes.NewCipher(s.encKey)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}
	return gcm.Seal(nonce, nonce, plain, nil), nil
}

func (s *Service) decryptHeaders(ciphertext []byte) (map[string]string, error) {
	if len(s.encKey) != 32 {
		return nil, httpx.BusinessRule("encryption key not configured")
	}
	block, err := aes.NewCipher(s.encKey)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	if len(ciphertext) < gcm.NonceSize() {
		return nil, errors.New("ciphertext too short")
	}
	nonce, ct := ciphertext[:gcm.NonceSize()], ciphertext[gcm.NonceSize():]
	plain, err := gcm.Open(nil, nonce, ct, nil)
	if err != nil {
		return nil, err
	}
	out := map[string]string{}
	if err := json.Unmarshal(plain, &out); err != nil {
		return nil, err
	}
	return out, nil
}
