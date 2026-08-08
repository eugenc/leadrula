package appointments

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/echayko/leadrula/backend/internal/auth"
	"github.com/echayko/leadrula/backend/internal/leads"
	"github.com/echayko/leadrula/backend/pkg/httpx"
	"github.com/jackc/pgx/v5"
)

type BookParams struct {
	ContractID          int64
	CalendarID          int64
	BuyerSlotID         int64
	PublisherSlotID     int64
	SlotStart           time.Time
	DurationMin         int
	CustomTime          bool
	DeliveryMode        string
	LeadID              int64
	FirstName           string
	LastName            string
	Phone               string
	Email               string
	Source              string
	PublisherPipelineID int64
	PublisherStageID    int64
	ExternalEventID     string
	ExternalProvider    string
	BookingTarget       string
}

func (s *Service) Book(ctx context.Context, p *auth.Principal, params BookParams) (*BookingRow, error) {
	if params.CalendarID != 0 && params.ContractID != 0 {
		return nil, httpx.Validation("provide contract_id or calendar_id, not both")
	}
	if params.CalendarID != 0 {
		return s.bookPublisherCalendar(ctx, p, params)
	}
	return s.bookAppointment(ctx, p, params, false)
}

func (s *Service) BookAsBuyer(ctx context.Context, p *auth.Principal, params BookParams) (*BookingRow, error) {
	if params.CalendarID != 0 && params.ContractID != 0 {
		return nil, httpx.Validation("provide contract_id or calendar_id, not both")
	}
	if params.CalendarID != 0 {
		return s.bookBuyerCalendar(ctx, p, params)
	}
	params.DeliveryMode = "contract"
	return s.bookAppointment(ctx, p, params, true)
}

func (s *Service) bookAppointment(ctx context.Context, p *auth.Principal, params BookParams, asBuyer bool) (*BookingRow, error) {
	prep, err := s.prepareContractBooking(ctx, p, params, asBuyer)
	if err != nil {
		return nil, err
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	result, err := s.executeBookAppointmentTx(ctx, tx, p, params, asBuyer, prep, params.LeadID)
	if err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	s.notif.SendEmails(result.Emails)
	return s.getBookingByID(ctx, result.BookingID)
}

func (s *Service) validateBuyerBookingSlot(ctx context.Context, p *auth.Principal, params BookParams, asBuyer bool, slotStart time.Time, loc *time.Location) (int, int, error) {
	if params.BuyerSlotID == 0 {
		return 0, 0, httpx.Validation("buyer_slot_id is required")
	}
	target := parseBookingTarget(params.BookingTarget)
	var contractSlots []ContractSlot
	var err error
	switch target {
	case bookingTargetActive:
		contractSlots, err = s.listActiveContractBuyerSlots(ctx, params.ContractID)
	case bookingTargetCross:
		if asBuyer {
			return 0, 0, httpx.Validation("buyer cross-booking uses publisher calendar")
		}
		contractSlots, err = s.ListContractSlots(ctx, p.AccountID, params.ContractID)
	default:
		contractSlots, err = s.listOwnBuyerCalendarSlots(ctx, p.AccountID, params.ContractID, asBuyer)
	}
	if err != nil {
		return 0, 0, err
	}
	var match *ContractSlot
	for i := range contractSlots {
		cs := &contractSlots[i]
		if cs.BuyerSlotID == params.BuyerSlotID && cs.Enabled && !cs.Disabled {
			match = cs
			break
		}
	}
	if match == nil {
		return 0, 0, httpx.Validation("slot not available for this contract")
	}
	expected, err := combineDateAndTime(slotStart, match.StartTime, loc)
	if err != nil || !expected.Truncate(time.Minute).Equal(slotStart.Truncate(time.Minute)) {
		return 0, 0, httpx.Validation("slot_start does not match slot template")
	}
	buyerSlot := BuyerSlot{DurationMin: match.DurationMin, Capacity: match.Capacity}
	return effectiveDuration(buyerSlot, match), effectiveCapacity(buyerSlot, match), nil
}

func (s *Service) validatePublisherBookingSlot(ctx context.Context, p *auth.Principal, params BookParams, asBuyer bool, slotStart time.Time, loc *time.Location) (int, int, error) {
	if params.PublisherSlotID == 0 {
		return 0, 0, httpx.Validation("publisher_slot_id is required")
	}
	target := parseBookingTarget(params.BookingTarget)
	var contractSlots []ContractPublisherSlot
	var err error
	switch target {
	case bookingTargetActive:
		contractSlots, err = s.listActiveContractPublisherSlots(ctx, params.ContractID)
	case bookingTargetCross:
		if asBuyer {
			contractSlots, err = s.ListContractPublisherSlotsForBuyer(ctx, p.AccountID, params.ContractID)
		} else {
			return 0, 0, httpx.Validation("publisher cross-booking uses buyer calendar")
		}
	default:
		contractSlots, err = s.listOwnPublisherCalendarSlots(ctx, p.AccountID, params.ContractID, asBuyer)
	}
	if err != nil {
		return 0, 0, err
	}
	var match *ContractPublisherSlot
	for i := range contractSlots {
		cs := &contractSlots[i]
		if cs.PublisherSlotID == params.PublisherSlotID && cs.Enabled && !cs.Disabled {
			match = cs
			break
		}
	}
	if match == nil {
		return 0, 0, httpx.Validation("slot not available for this contract")
	}
	expected, err := combineDateAndTime(slotStart, match.StartTime, loc)
	if err != nil || !expected.Truncate(time.Minute).Equal(slotStart.Truncate(time.Minute)) {
		return 0, 0, httpx.Validation("slot_start does not match slot template")
	}
	pubSlot := PublisherSlot{DurationMin: match.DurationMin, Capacity: match.Capacity}
	return effectivePublisherDuration(pubSlot, match), effectivePublisherCapacity(pubSlot, match), nil
}

func (s *Service) countBuyerSlotOccupancyTx(ctx context.Context, tx pgx.Tx, buyerSlotID int64, slotStart time.Time) (int, error) {
	var n int
	err := tx.QueryRow(ctx, buyerSlotOccupancySQL, buyerSlotID, slotStart).Scan(&n)
	return n, err
}

func (s *Service) countPublisherSlotOccupancyTx(ctx context.Context, tx pgx.Tx, contractID, publisherSlotID int64, slotStart time.Time) (int, error) {
	var n int
	err := tx.QueryRow(ctx, publisherSlotOccupancySQL, contractID, publisherSlotID, slotStart).Scan(&n)
	return n, err
}

func (s *Service) createLeadForBooking(ctx context.Context, tx pgx.Tx, p *auth.Principal, params BookParams) (int64, error) {
	raw, _ := json.Marshal(map[string]string{
		"first_name": params.FirstName,
		"last_name":  params.LastName,
		"phone":      params.Phone,
		"email":      params.Email,
		"source":     params.Source,
	})
	source := strings.TrimSpace(params.Source)
	leadID, _, err := s.leads.InsertLead(ctx, tx, p.AccountID, p.AccountID, source, raw)
	if err != nil {
		return 0, err
	}
	for field, val := range map[string]string{
		"first_name": params.FirstName,
		"last_name":  params.LastName,
		"phone":      params.Phone,
		"email":      params.Email,
		"source":     source,
	} {
		if strings.TrimSpace(val) == "" {
			continue
		}
		if err := s.leads.SetBuiltinField(ctx, tx, leadID, field, val); err != nil {
			return 0, err
		}
	}
	if err := s.leads.LogLeadCreated(ctx, tx, leadID, p.AccountID, leads.ActorFromPrincipal(p), source); err != nil {
		return 0, err
	}
	return leadID, nil
}

func (s *Service) createLeadForBuyerBooking(ctx context.Context, tx pgx.Tx, p *auth.Principal, contractID int64, params BookParams) (int64, error) {
	var publisherID int64
	if err := tx.QueryRow(ctx, `SELECT publisher_id FROM contracts WHERE id=$1`, contractID).Scan(&publisherID); err != nil {
		return 0, err
	}
	raw, _ := json.Marshal(map[string]string{
		"first_name": params.FirstName,
		"last_name":  params.LastName,
		"phone":      params.Phone,
		"email":      params.Email,
		"source":     params.Source,
	})
	source := strings.TrimSpace(params.Source)
	leadID, _, err := s.leads.InsertLead(ctx, tx, p.AccountID, publisherID, source, raw)
	if err != nil {
		return 0, err
	}
	for field, val := range map[string]string{
		"first_name": params.FirstName,
		"last_name":  params.LastName,
		"phone":      params.Phone,
		"email":      params.Email,
		"source":     source,
	} {
		if strings.TrimSpace(val) == "" {
			continue
		}
		if err := s.leads.SetBuiltinField(ctx, tx, leadID, field, val); err != nil {
			return 0, err
		}
	}
	if err := s.leads.LogLeadCreated(ctx, tx, leadID, p.AccountID, leads.ActorFromPrincipal(p), source); err != nil {
		return 0, err
	}
	return leadID, nil
}

func (s *Service) getBookingByID(ctx context.Context, id int64) (*BookingRow, error) {
	rows, err := s.listBookingsQuery(ctx, `b.id=$1`, []any{id}, "b.created_at DESC", 0)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, httpx.NotFound("booking not found")
	}
	return &rows[0], nil
}

func nullStr(s string) any {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	return s
}

func validateCustomDurationMin(dur int) (int, error) {
	if dur < 15 || dur > 240 {
		return 0, httpx.Validation("duration_min must be between 15 and 240")
	}
	return dur, nil
}
