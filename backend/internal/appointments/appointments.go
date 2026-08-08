package appointments

import (
	"context"
	"encoding/json"
	"time"

	"github.com/echayko/leadrula/backend/internal/accounts"
	"github.com/echayko/leadrula/backend/internal/leads"
	"github.com/echayko/leadrula/backend/internal/notifications"
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
	CalendarID  int64      `json:"calendar_id"`
	Weekday     int        `json:"weekday"`
	StartTime   string     `json:"start_time"`
	DurationMin int        `json:"duration_min"`
	Capacity    int        `json:"capacity"`
	DisabledAt  *time.Time `json:"disabled_at,omitempty"`
}

type PublisherSlot struct {
	ID          int64      `json:"id"`
	AccountID   int64      `json:"account_id"`
	CalendarID  int64      `json:"calendar_id"`
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
	BuyerSlotID       int64     `json:"buyer_slot_id,omitempty"`
	PublisherSlotID   int64     `json:"publisher_slot_id,omitempty"`
	SlotStart         time.Time `json:"slot_start"`
	DurationMin       int       `json:"duration_min"`
	Capacity          int       `json:"capacity"`
	RemainingCapacity int       `json:"remaining_capacity"`
}

type CalendarDayMarker struct {
	Date          string `json:"date"`
	HasBookable   bool   `json:"has_bookable"`
	HasBookings   bool   `json:"has_bookings"`
}

type BookingRow struct {
	ID             int64      `json:"id"`
	ContractID     int64      `json:"contract_id"`
	ContractName   string     `json:"contract_name,omitempty"`
	LeadID         int64      `json:"lead_id"`
	LeadName       string     `json:"lead_name"`
	Phone          string     `json:"phone,omitempty"`
	Email          string     `json:"email,omitempty"`
	BookedAt       time.Time  `json:"booked_at"`
	AppointmentAt  *time.Time `json:"appointment_at,omitempty"`
	DurationMin    int        `json:"duration_min,omitempty"`
	DeliveryMode   string     `json:"delivery_mode,omitempty"`
	DeliveryStatus string     `json:"delivery_status,omitempty"`
	BuyerName      string     `json:"buyer_name,omitempty"`
	PublisherName  string     `json:"publisher_name,omitempty"`
	CalendarName   string     `json:"calendar_name,omitempty"`
	LeadStatus       string     `json:"lead_status,omitempty"`
	ExternalEventID  string     `json:"external_event_id,omitempty"`
	IsRoute          bool       `json:"-"`
}

type AppointmentContract struct {
	ContractID             int64  `json:"contract_id"`
	ContractName           string `json:"contract_name"`
	BuyerID                int64  `json:"buyer_id"`
	BuyerName              string `json:"buyer_name"`
	Timezone               string `json:"timezone"`
	Location               string `json:"location,omitempty"`
	Configured             bool   `json:"configured"`
	OwnConfigured          bool   `json:"own_configured"`
	CounterpartyConfigured bool   `json:"counterparty_configured"`
	CalendarSource         string `json:"calendar_source,omitempty"`
	LeadDelivery           string `json:"lead_delivery,omitempty"`
}

func (s *Service) getAccountTimezone(ctx context.Context, accountID int64) (string, error) {
	if s.accounts != nil {
		acct, err := s.accounts.GetAccount(ctx, accountID)
		if err != nil {
			return "", err
		}
		if acct.Timezone != "" {
			return acct.Timezone, nil
		}
		return "UTC", nil
	}
	var tz string
	if err := s.pool.QueryRow(ctx, `SELECT COALESCE(NULLIF(timezone,''), 'UTC') FROM accounts WHERE id=$1`, accountID).Scan(&tz); err != nil {
		return "UTC", nil
	}
	return tz, nil
}

func (s *Service) loadAvailability(ctx context.Context, buyerID int64) (*Availability, error) {
	calID, err := s.firstCalendarID(ctx, buyerID)
	if err != nil {
		return nil, err
	}
	if calID == 0 {
		tz, tzErr := s.getAccountTimezone(ctx, buyerID)
		if tzErr != nil {
			return nil, tzErr
		}
		return &Availability{AccountID: buyerID, Timezone: tz, Schedule: json.RawMessage(`{}`)}, nil
	}
	cal, err := s.GetBookingCalendar(ctx, buyerID, calID)
	if err != nil {
		return nil, err
	}
	return &Availability{
		AccountID:  buyerID,
		Schedule:   cal.Schedule,
		Timezone:   cal.Timezone,
		BufferMin:  cal.BufferMin,
		Configured: cal.Configured,
		UpdatedAt:  cal.UpdatedAt,
	}, nil
}

func (s *Service) buyerConfigured(ctx context.Context, buyerID int64) (bool, error) {
	var ok bool
	err := s.pool.QueryRow(ctx,
		`SELECT EXISTS(
			SELECT 1 FROM buyer_booking_calendars c
			WHERE c.account_id = $1 AND c.schedule::text NOT IN ('{}', 'null')
		) AND EXISTS(
			SELECT 1 FROM buyer_appointment_slots s
			JOIN buyer_booking_calendars c ON c.id = s.calendar_id
			WHERE c.account_id = $1 AND s.disabled_at IS NULL
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
