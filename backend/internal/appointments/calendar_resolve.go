package appointments

import (
	"context"
	"errors"

	"github.com/echayko/leadrula/backend/pkg/httpx"
	"github.com/jackc/pgx/v5"
)

const (
	calendarSourceBuyer      = "buyer"
	calendarSourcePublisher  = "publisher"
	bookingTargetOwn         = "own"
	bookingTargetCross       = "cross"
	bookingTargetActive      = "active"
)

func parseBookingTarget(s string) string {
	switch s {
	case bookingTargetCross:
		return bookingTargetCross
	case bookingTargetActive:
		return bookingTargetActive
	default:
		return bookingTargetOwn
	}
}

type activeCalendar struct {
	Source     string
	CalendarID int64
	Timezone   string
	Location   string
}

type contractCalendarRow struct {
	Source                  *string
	BuyerCalendarID         *int64
	PublisherCalendarID     *int64
}

func (s *Service) loadContractCalendarRow(ctx context.Context, contractID int64) (contractCalendarRow, error) {
	var row contractCalendarRow
	err := s.pool.QueryRow(ctx,
		`SELECT appointment_calendar_source, appointment_calendar_id, publisher_appointment_calendar_id
		 FROM contracts WHERE id = $1`, contractID).Scan(
		&row.Source, &row.BuyerCalendarID, &row.PublisherCalendarID)
	if errors.Is(err, pgx.ErrNoRows) {
		return row, httpx.NotFound("contract not found")
	}
	return row, err
}

func (s *Service) resolveActiveCalendar(ctx context.Context, contractID int64) (activeCalendar, error) {
	row, err := s.loadContractCalendarRow(ctx, contractID)
	if err != nil {
		return activeCalendar{}, err
	}
	if row.Source == nil || *row.Source == "" {
		return activeCalendar{}, httpx.Validation("buyer has not selected an appointment calendar")
	}
	switch *row.Source {
	case calendarSourceBuyer:
		if row.BuyerCalendarID == nil || *row.BuyerCalendarID == 0 {
			return activeCalendar{}, httpx.Validation("contract has no buyer appointment calendar")
		}
		cal, err := s.loadCalendarByID(ctx, *row.BuyerCalendarID)
		if err != nil {
			return activeCalendar{}, err
		}
		return activeCalendar{
			Source: calendarSourceBuyer, CalendarID: cal.ID,
			Timezone: cal.Timezone, Location: cal.Location,
		}, nil
	case calendarSourcePublisher:
		if row.PublisherCalendarID == nil || *row.PublisherCalendarID == 0 {
			return activeCalendar{}, httpx.Validation("contract has no publisher appointment calendar")
		}
		cal, err := s.loadPublisherCalendarByID(ctx, *row.PublisherCalendarID)
		if err != nil {
			return activeCalendar{}, err
		}
		return activeCalendar{
			Source: calendarSourcePublisher, CalendarID: cal.ID,
			Timezone: cal.Timezone, Location: cal.Location,
		}, nil
	default:
		return activeCalendar{}, httpx.Validation("invalid appointment calendar source")
	}
}

func (s *Service) contractCalendarConfigured(ctx context.Context, contractID int64) (bool, error) {
	active, err := s.resolveActiveCalendar(ctx, contractID)
	if err != nil {
		var appErr *httpx.AppError
		if errors.As(err, &appErr) && appErr.Code == httpx.CodeValidation {
			return false, nil
		}
		return false, err
	}
	switch active.Source {
	case calendarSourceBuyer:
		return s.calendarConfigured(ctx, active.CalendarID)
	case calendarSourcePublisher:
		return s.publisherCalendarConfigured(ctx, active.CalendarID)
	default:
		return false, nil
	}
}

func (s *Service) contractAccepted(ctx context.Context, contractID int64) (bool, error) {
	var status string
	var buyerID *int64
	err := s.pool.QueryRow(ctx,
		`SELECT status, buyer_id FROM contracts WHERE id=$1 AND deleted_at IS NULL`,
		contractID).Scan(&status, &buyerID)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, httpx.NotFound("contract not found")
	}
	if err != nil {
		return false, err
	}
	if status != "active" {
		return false, nil
	}
	if buyerID != nil && *buyerID > 0 {
		return true, nil
	}
	var ok bool
	err = s.pool.QueryRow(ctx,
		`SELECT EXISTS(
			SELECT 1 FROM contract_participations
			WHERE contract_id=$1 AND status='active')`, contractID).Scan(&ok)
	return ok, err
}

func (s *Service) resolveBookingCalendar(ctx context.Context, contractID int64, asBuyer bool, target string) (activeCalendar, error) {
	if target == bookingTargetActive {
		return s.resolveActiveCalendar(ctx, contractID)
	}
	if target == bookingTargetCross {
		return s.resolveCrossBookingCalendar(ctx, contractID, asBuyer)
	}
	return s.resolveOwnBookingCalendar(ctx, contractID, asBuyer)
}

func (s *Service) resolveOwnBookingCalendar(ctx context.Context, contractID int64, asBuyer bool) (activeCalendar, error) {
	row, err := s.loadContractCalendarRow(ctx, contractID)
	if err != nil {
		return activeCalendar{}, err
	}
	if asBuyer {
		if row.BuyerCalendarID == nil || *row.BuyerCalendarID == 0 {
			return activeCalendar{}, httpx.Validation("contract has no buyer appointment calendar")
		}
		cal, err := s.loadCalendarByID(ctx, *row.BuyerCalendarID)
		if err != nil {
			return activeCalendar{}, err
		}
		return activeCalendar{
			Source: calendarSourceBuyer, CalendarID: cal.ID,
			Timezone: cal.Timezone, Location: cal.Location,
		}, nil
	}
	if row.PublisherCalendarID == nil || *row.PublisherCalendarID == 0 {
		return activeCalendar{}, httpx.Validation("contract has no publisher appointment calendar")
	}
	cal, err := s.loadPublisherCalendarByID(ctx, *row.PublisherCalendarID)
	if err != nil {
		return activeCalendar{}, err
	}
	return activeCalendar{
		Source: calendarSourcePublisher, CalendarID: cal.ID,
		Timezone: cal.Timezone, Location: cal.Location,
	}, nil
}

func (s *Service) resolveCrossBookingCalendar(ctx context.Context, contractID int64, asBuyer bool) (activeCalendar, error) {
	ok, err := s.contractAccepted(ctx, contractID)
	if err != nil {
		return activeCalendar{}, err
	}
	if !ok {
		return activeCalendar{}, httpx.Validation("contract is not accepted yet")
	}
	row, err := s.loadContractCalendarRow(ctx, contractID)
	if err != nil {
		return activeCalendar{}, err
	}
	if asBuyer {
		if row.PublisherCalendarID == nil || *row.PublisherCalendarID == 0 {
			return activeCalendar{}, httpx.Validation("contract has no publisher appointment calendar")
		}
		cal, err := s.loadPublisherCalendarByID(ctx, *row.PublisherCalendarID)
		if err != nil {
			return activeCalendar{}, err
		}
		return activeCalendar{
			Source: calendarSourcePublisher, CalendarID: cal.ID,
			Timezone: cal.Timezone, Location: cal.Location,
		}, nil
	}
	if row.BuyerCalendarID == nil || *row.BuyerCalendarID == 0 {
		return activeCalendar{}, httpx.Validation("contract has no buyer appointment calendar")
	}
	cal, err := s.loadCalendarByID(ctx, *row.BuyerCalendarID)
	if err != nil {
		return activeCalendar{}, err
	}
	return activeCalendar{
		Source: calendarSourceBuyer, CalendarID: cal.ID,
		Timezone: cal.Timezone, Location: cal.Location,
	}, nil
}

func (s *Service) ownCalendarConfigured(ctx context.Context, contractID int64, asBuyer bool) (bool, error) {
	active, err := s.resolveOwnBookingCalendar(ctx, contractID, asBuyer)
	if err != nil {
		var appErr *httpx.AppError
		if errors.As(err, &appErr) && appErr.Code == httpx.CodeValidation {
			return false, nil
		}
		return false, err
	}
	switch active.Source {
	case calendarSourceBuyer:
		return s.calendarConfigured(ctx, active.CalendarID)
	case calendarSourcePublisher:
		return s.publisherCalendarConfigured(ctx, active.CalendarID)
	default:
		return false, nil
	}
}

func (s *Service) crossCalendarConfigured(ctx context.Context, contractID int64, asBuyer bool) (bool, error) {
	ok, err := s.contractAccepted(ctx, contractID)
	if err != nil || !ok {
		return false, err
	}
	active, err := s.resolveCrossBookingCalendar(ctx, contractID, asBuyer)
	if err != nil {
		var appErr *httpx.AppError
		if errors.As(err, &appErr) && appErr.Code == httpx.CodeValidation {
			return false, nil
		}
		return false, err
	}
	switch active.Source {
	case calendarSourceBuyer:
		return s.calendarConfigured(ctx, active.CalendarID)
	case calendarSourcePublisher:
		return s.publisherCalendarConfigured(ctx, active.CalendarID)
	default:
		return false, nil
	}
}

func (s *Service) bookingCalendarConfigured(ctx context.Context, contractID int64, asBuyer bool, target string) (bool, error) {
	if target == bookingTargetActive {
		return s.contractCalendarConfigured(ctx, contractID)
	}
	if target == bookingTargetCross {
		return s.crossCalendarConfigured(ctx, contractID, asBuyer)
	}
	return s.ownCalendarConfigured(ctx, contractID, asBuyer)
}

func (s *Service) contractHasBuyerSelectedSource(ctx context.Context, contractID int64) (bool, error) {
	row, err := s.loadContractCalendarRow(ctx, contractID)
	if err != nil {
		return false, err
	}
	return row.Source != nil && *row.Source != "", nil
}

func (s *Service) contractIDsForCalendar(ctx context.Context, accountID, calendarID int64, source string) ([]int64, error) {
	var query string
	switch source {
	case calendarSourcePublisher:
		query = `SELECT id FROM contracts
		         WHERE publisher_id=$1 AND appointment_calendar_source='publisher'
		           AND publisher_appointment_calendar_id=$2`
	case calendarSourceBuyer:
		query = `SELECT id FROM contracts
		         WHERE buyer_id=$1 AND appointment_calendar_source='buyer'
		           AND appointment_calendar_id=$2`
	default:
		return nil, nil
	}
	rows, err := s.pool.Query(ctx, query, accountID, calendarID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

func (s *Service) enrichBookingCalendarContracts(ctx context.Context, cal *BookingCalendar, accountID int64, source string) error {
	ids, err := s.contractIDsForCalendar(ctx, accountID, cal.ID, source)
	if err != nil {
		return err
	}
	cal.CalendarSource = source
	if len(ids) == 1 {
		id := ids[0]
		cal.ContractID = &id
	}
	if len(ids) > 0 {
		cal.ContractIDs = ids
	}
	return nil
}

func (s *Service) calendarDeleteBlocked(ctx context.Context, accountID, calendarID int64, source string) (string, error) {
	var contractQuery string
	var bookingQuery string
	switch source {
	case calendarSourceBuyer:
		contractQuery = `SELECT EXISTS(
			SELECT 1 FROM contracts
			WHERE buyer_id=$1 AND deleted_at IS NULL AND appointment_calendar_id=$2)`
		bookingQuery = `SELECT EXISTS(
			SELECT 1 FROM lead_appointment_bookings b
			WHERE b.buyer_calendar_id=$1
			   OR EXISTS (
			     SELECT 1 FROM buyer_appointment_slots sl
			     WHERE sl.id = b.buyer_slot_id AND sl.calendar_id = $1
			   ))`
	case calendarSourcePublisher:
		contractQuery = `SELECT EXISTS(
			SELECT 1 FROM contracts
			WHERE publisher_id=$1 AND deleted_at IS NULL AND publisher_appointment_calendar_id=$2)`
		bookingQuery = `SELECT EXISTS(
			SELECT 1 FROM lead_appointment_bookings b
			WHERE b.publisher_calendar_id=$1
			   OR EXISTS (
			     SELECT 1 FROM publisher_appointment_slots sl
			     WHERE sl.id = b.publisher_slot_id AND sl.calendar_id = $1
			   ))`
	default:
		return "", nil
	}
	var attached bool
	if err := s.pool.QueryRow(ctx, contractQuery, accountID, calendarID).Scan(&attached); err != nil {
		return "", err
	}
	if attached {
		return "calendar is attached to contracts; detach it first", nil
	}
	if err := s.pool.QueryRow(ctx, bookingQuery, calendarID).Scan(&attached); err != nil {
		return "", err
	}
	if attached {
		return "calendar has appointment bookings; cancel them first", nil
	}
	return "", nil
}
