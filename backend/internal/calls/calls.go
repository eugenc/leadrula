// Package calls implements contract-owned live call routing: inbound Twilio
// webhooks create a publisher lead + call, route across priority tiers
// (simuldial, RTB, waterfall), bill on billable connect or no-answer, and
// assign the lead to the winning buyer only after a successful debit.
package calls

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strings"
	"time"

	"github.com/echayko/leadrula/backend/internal/accounts"
	"github.com/echayko/leadrula/backend/internal/database"
	"github.com/echayko/leadrula/backend/internal/leads"
	"github.com/echayko/leadrula/backend/internal/notifications"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// CredentialProvider decrypts a publisher's stored Twilio credentials.
type CredentialProvider interface {
	DecryptedCredentials(ctx context.Context, accountID, connectionID int64) (map[string]string, error)
}

type Service struct {
	pool         *pgxpool.Pool
	encKey       []byte
	leads        *leads.Repository
	accounts     *accounts.Repository
	notif        *notifications.Service
	integrations leads.IntegrationEnqueuer
	creds        CredentialProvider
	webhookBase  string
}

func NewService(pool *pgxpool.Pool, encKey []byte, leadRepo *leads.Repository, acc *accounts.Repository, notif *notifications.Service, integrations leads.IntegrationEnqueuer, creds CredentialProvider, webhookBase string) *Service {
	return &Service{
		pool:         pool,
		encKey:       encKey,
		leads:        leadRepo,
		accounts:     acc,
		notif:        notif,
		integrations: integrations,
		creds:        creds,
		webhookBase:  strings.TrimRight(webhookBase, "/"),
	}
}

func (s *Service) routeDeps() leads.RouteApplyDeps {
	return leads.RouteApplyDeps{Repo: s.leads, Accounts: s.accounts, Notif: s.notif, Integrations: s.integrations}
}

// ── Models ────────────────────────────────────────────────────────

type Call struct {
	ID                    int64           `json:"id"`
	PublicID              string          `json:"public_id"`
	PublisherID           int64           `json:"-"`
	SourceID              *int64          `json:"source_id,omitempty"`
	ContractID            *int64          `json:"contract_id,omitempty"`
	LeadID                *int64          `json:"lead_id,omitempty"`
	WinnerParticipationID *int64          `json:"winner_participation_id,omitempty"`
	TwilioCallSID         *string         `json:"twilio_call_sid,omitempty"`
	CallerNumber          *string         `json:"caller_number,omitempty"`
	CallerState           *string         `json:"caller_state,omitempty"`
	TrackingNumber        *string         `json:"tracking_number,omitempty"`
	Status                string          `json:"status"`
	Disposition           *string         `json:"disposition,omitempty"`
	DispositionNote       *string         `json:"disposition_note,omitempty"`
	Billable              bool            `json:"billable"`
	DurationSec           int             `json:"duration_sec"`
	BillableDurationSec   int             `json:"billable_duration_sec"`
	PriceCents            int             `json:"price_cents"`
	RecordingURL          *string         `json:"recording_url,omitempty"`
	ConnectedAt           *time.Time      `json:"connected_at,omitempty"`
	EndedAt               *time.Time      `json:"ended_at,omitempty"`
	CreatedAt             time.Time       `json:"created_at"`
	// Enriched (call log / detail).
	ContractName  *string   `json:"contract_name,omitempty"`
	WinnerName    *string   `json:"winner_buyer_name,omitempty"`
	LeadName      *string   `json:"lead_name,omitempty"`
	LeadPhone     *string   `json:"lead_phone,omitempty"`
	PublisherName *string   `json:"publisher_name,omitempty"`
	Legs          []CallLeg `json:"legs,omitempty"`
	Pings         []RTBPing `json:"rtb_pings,omitempty"`
}

type CallLeg struct {
	ID                int64      `json:"id"`
	CallID            int64      `json:"call_id"`
	ParticipationID   *int64     `json:"participation_id,omitempty"`
	BuyerID           *int64     `json:"buyer_id,omitempty"`
	BuyerName         *string    `json:"buyer_name,omitempty"`
	Tier              int        `json:"tier"`
	DestinationNumber *string    `json:"destination_number,omitempty"`
	TwilioCallSID     *string    `json:"twilio_call_sid,omitempty"`
	LegStatus         string     `json:"leg_status"`
	Rate              float64    `json:"rate"`
	Billed            bool       `json:"billed"`
	AnsweredAt        *time.Time `json:"answered_at,omitempty"`
	EndedAt           *time.Time `json:"ended_at,omitempty"`
	DurationSec       int        `json:"duration_sec"`
	CreatedAt         time.Time  `json:"created_at"`
}

type RTBPing struct {
	ID                int64     `json:"id"`
	CallID            int64     `json:"call_id"`
	ParticipationID   *int64    `json:"participation_id,omitempty"`
	Endpoint          string    `json:"endpoint"`
	Accepted          bool      `json:"accepted"`
	BidAmount         *float64  `json:"bid_amount,omitempty"`
	DestinationNumber *string   `json:"destination_number,omitempty"`
	ResponseStatus    *int      `json:"response_status,omitempty"`
	ResponseBody      *string   `json:"response_body,omitempty"`
	Reason            *string   `json:"reason,omitempty"`
	CreatedAt         time.Time `json:"created_at"`
}

// hashPhone normalizes a phone number to digits and returns its SHA-256 hex.
func hashPhone(phone string) string {
	var b strings.Builder
	for _, r := range phone {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	digits := b.String()
	if digits == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(digits))
	return hex.EncodeToString(sum[:])
}

// ── Call rows ─────────────────────────────────────────────────────

const callCols = `id, public_id::text, publisher_id, source_id, contract_id, lead_id,
	winner_participation_id, twilio_call_sid, caller_number, caller_state, tracking_number,
	status, disposition, disposition_note, billable, duration_sec, billable_duration_sec,
	price_cents, recording_url, connected_at, ended_at, created_at`

func scanCall(row pgx.Row) (*Call, error) {
	c := &Call{}
	err := row.Scan(&c.ID, &c.PublicID, &c.PublisherID, &c.SourceID, &c.ContractID, &c.LeadID,
		&c.WinnerParticipationID, &c.TwilioCallSID, &c.CallerNumber, &c.CallerState, &c.TrackingNumber,
		&c.Status, &c.Disposition, &c.DispositionNote, &c.Billable, &c.DurationSec, &c.BillableDurationSec,
		&c.PriceCents, &c.RecordingURL, &c.ConnectedAt, &c.EndedAt, &c.CreatedAt)
	if err != nil {
		return nil, err
	}
	return c, nil
}

func (s *Service) insertCall(ctx context.Context, q database.Querier, publisherID int64, sourceID, contractID, leadID *int64, caller, callerHash, callerState, trackingNumber, twilioSID string, status string) (*Call, error) {
	return scanCall(q.QueryRow(ctx,
		`INSERT INTO calls(publisher_id, source_id, contract_id, lead_id, caller_number, caller_phone_hash,
		   caller_state, tracking_number, twilio_call_sid, status)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
		 RETURNING `+callCols,
		publisherID, sourceID, contractID, leadID, nullStr(caller), nullStr(callerHash),
		nullStr(callerState), nullStr(trackingNumber), nullStr(twilioSID), status))
}

func (s *Service) getCallByID(ctx context.Context, q database.Querier, id int64) (*Call, error) {
	return scanCall(q.QueryRow(ctx, `SELECT `+callCols+` FROM calls WHERE id=$1`, id))
}

func (s *Service) getCallByPublicID(ctx context.Context, q database.Querier, publicID string) (*Call, error) {
	return scanCall(q.QueryRow(ctx, `SELECT `+callCols+` FROM calls WHERE public_id=$1`, publicID))
}

func (s *Service) legsForCall(ctx context.Context, q database.Querier, callID int64) ([]CallLeg, error) {
	rows, err := q.Query(ctx,
		`SELECT cl.id, cl.call_id, cl.participation_id, cl.buyer_id, a.name, cl.tier,
		        cl.destination_number, cl.twilio_call_sid, cl.leg_status, cl.rate::float8, cl.billed,
		        cl.answered_at, cl.ended_at, cl.duration_sec, cl.created_at
		 FROM call_legs cl
		 LEFT JOIN accounts a ON a.id = cl.buyer_id
		 WHERE cl.call_id=$1 ORDER BY cl.tier, cl.id`, callID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []CallLeg
	for rows.Next() {
		var l CallLeg
		if err := rows.Scan(&l.ID, &l.CallID, &l.ParticipationID, &l.BuyerID, &l.BuyerName, &l.Tier,
			&l.DestinationNumber, &l.TwilioCallSID, &l.LegStatus, &l.Rate, &l.Billed,
			&l.AnsweredAt, &l.EndedAt, &l.DurationSec, &l.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, l)
	}
	return out, rows.Err()
}

func (s *Service) pingsForCall(ctx context.Context, q database.Querier, callID int64) ([]RTBPing, error) {
	rows, err := q.Query(ctx,
		`SELECT id, call_id, participation_id, endpoint, accepted, bid_amount::float8,
		        destination_number, response_status, response_body, reason, created_at
		 FROM rtb_pings WHERE call_id=$1 ORDER BY id`, callID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []RTBPing
	for rows.Next() {
		var p RTBPing
		if err := rows.Scan(&p.ID, &p.CallID, &p.ParticipationID, &p.Endpoint, &p.Accepted, &p.BidAmount,
			&p.DestinationNumber, &p.ResponseStatus, &p.ResponseBody, &p.Reason, &p.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func nullStr(s string) interface{} {
	if s == "" {
		return nil
	}
	return s
}

func toJSON(v any) json.RawMessage {
	b, _ := json.Marshal(v)
	return b
}
