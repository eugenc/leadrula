package appointments

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/echayko/leadrula/backend/internal/accounts"
	"github.com/echayko/leadrula/backend/internal/leads"
	"github.com/echayko/leadrula/backend/internal/notifications"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Service struct {
	pool     *pgxpool.Pool
	leads    *leads.Repository
	accounts *accounts.Repository
	notif    *notifications.Service
}

func NewService(pool *pgxpool.Pool, leadsRepo *leads.Repository, accountsRepo *accounts.Repository, notif *notifications.Service) *Service {
	return &Service{
		pool:     pool,
		leads:    leadsRepo,
		accounts: accountsRepo,
		notif:    notif,
	}
}

type Availability struct {
	AccountID  int64           `json:"account_id"`
	Schedule   json.RawMessage `json:"schedule"`
	Timezone   string          `json:"timezone"`
	BufferMin  int             `json:"buffer_min"`
	Configured bool            `json:"configured"`
	UpdatedAt  time.Time       `json:"updated_at"`
}

type BuyerSlot struct {
	ID          int64      `json:"id"`
	AccountID   int64      `json:"account_id"`
	Weekday     int        `json:"weekday"`
	StartTime   string     `json:"start_time"`
	DurationMin int        `json:"duration_min"`
	Capacity    int        `json:"capacity"`
	DisabledAt  *time.Time `json:"disabled_at,omitempty"`
}

type ContractSlot struct {
	BuyerSlotID          int64  `json:"buyer_slot_id"`
	Weekday              int    `json:"weekday"`
	StartTime            string `json:"start_time"`
	DurationMin          int    `json:"duration_min"`
	Capacity             int    `json:"capacity"`
	Enabled              bool   `json:"enabled"`
	DurationMinOverride  *int   `json:"duration_min_override,omitempty"`
	CapacityOverride     *int   `json:"capacity_override,omitempty"`
	Disabled             bool   `json:"disabled"`
}

type FreeSlot struct {
	BuyerSlotID        int64     `json:"buyer_slot_id"`
	SlotStart          time.Time `json:"slot_start"`
	DurationMin        int       `json:"duration_min"`
	Capacity           int       `json:"capacity"`
	RemainingCapacity  int       `json:"remaining_capacity"`
}

type CalendarDayMarker struct {
	Date          string `json:"date"`
	HasBookable   bool   `json:"has_bookable"`
	HasBookings   bool   `json:"has_bookings"`
}

type BookingRow struct {
	ID             int64     `json:"id"`
	ContractID     int64     `json:"contract_id"`
	ContractName   string    `json:"contract_name,omitempty"`
	LeadID         int64     `json:"lead_id"`
	LeadName       string    `json:"lead_name"`
	Phone          string    `json:"phone,omitempty"`
	Email          string    `json:"email,omitempty"`
	SlotStart      time.Time `json:"slot_start"`
	DurationMin    int       `json:"duration_min"`
	DeliveryMode   string    `json:"delivery_mode"`
	DeliveryStatus string    `json:"delivery_status"`
	BuyerName      string    `json:"buyer_name,omitempty"`
	PublisherName  string    `json:"publisher_name,omitempty"`
	LeadStatus     string    `json:"lead_status,omitempty"`
	CreatedAt      time.Time `json:"created_at"`
}

type AppointmentContract struct {
	ContractID   int64  `json:"contract_id"`
	ContractName string `json:"contract_name"`
	BuyerID      int64  `json:"buyer_id"`
	BuyerName    string `json:"buyer_name"`
	Timezone     string `json:"timezone"`
	Configured   bool   `json:"configured"`
}

func (s *Service) getAccountTimezone(ctx context.Context, accountID int64) (string, error) {
	acct, err := s.accounts.GetAccount(ctx, accountID)
	if err != nil {
		return "", err
	}
	if acct.Timezone != "" {
		return acct.Timezone, nil
	}
	return "UTC", nil
}

func (s *Service) loadAvailability(ctx context.Context, buyerID int64) (*Availability, error) {
	a := &Availability{AccountID: buyerID}
	err := s.pool.QueryRow(ctx,
		`SELECT schedule, timezone, buffer_min, updated_at FROM buyer_availability WHERE account_id=$1`,
		buyerID).Scan(&a.Schedule, &a.Timezone, &a.BufferMin, &a.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		tz, tzErr := s.getAccountTimezone(ctx, buyerID)
		if tzErr != nil {
			return nil, tzErr
		}
		a.Timezone = tz
		a.Schedule = json.RawMessage(`{}`)
		return a, nil
	}
	if err != nil {
		return nil, err
	}
	a.Configured, _ = s.buyerConfigured(ctx, buyerID)
	return a, nil
}

func (s *Service) buyerConfigured(ctx context.Context, buyerID int64) (bool, error) {
	var ok bool
	err := s.pool.QueryRow(ctx,
		`SELECT EXISTS(
			SELECT 1 FROM buyer_availability ba
			WHERE ba.account_id = $1 AND ba.schedule::text NOT IN ('{}', 'null')
		) AND EXISTS(
			SELECT 1 FROM buyer_appointment_slots s
			WHERE s.account_id = $1 AND s.disabled_at IS NULL
		)`, buyerID).Scan(&ok)
	return ok, err
}

func effectiveCapacity(slot BuyerSlot, cs *ContractSlot) int {
	if cs != nil && cs.CapacityOverride != nil {
		return *cs.CapacityOverride
	}
	return slot.Capacity
}

func effectiveDuration(slot BuyerSlot, cs *ContractSlot) int {
	if cs != nil && cs.DurationMinOverride != nil {
		return *cs.DurationMinOverride
	}
	return slot.DurationMin
}
