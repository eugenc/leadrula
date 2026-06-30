package appointments

import (
	"context"
	"errors"

	"github.com/echayko/leadrula/backend/pkg/httpx"
	"github.com/jackc/pgx/v5"
)

type BuyerAppointmentContract struct {
	ContractID    int64  `json:"contract_id"`
	ContractName  string `json:"contract_name"`
	PublisherID   int64  `json:"publisher_id"`
	PublisherName string `json:"publisher_name"`
	Timezone      string `json:"timezone"`
	Configured    bool   `json:"configured"`
}

func (s *Service) ListBuyerAppointmentContracts(ctx context.Context, buyerID int64) ([]BuyerAppointmentContract, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT c.id, c.name, c.publisher_id, COALESCE(p.name, ''),
		        COALESCE(bc.timezone, b.timezone, 'UTC'),
		        (bc.id IS NOT NULL AND bc.schedule::text NOT IN ('{}', 'null')
		         AND EXISTS(SELECT 1 FROM buyer_appointment_slots sl
		                    WHERE sl.calendar_id = bc.id AND sl.disabled_at IS NULL))
		 FROM contracts c
		 JOIN accounts b ON b.id = c.buyer_id
		 JOIN accounts p ON p.id = c.publisher_id
		 JOIN buyer_booking_calendars bc ON bc.id = c.appointment_calendar_id
		 WHERE c.buyer_id = $1 AND c.lead_type = 'Appointment' AND c.status = 'active'
		   AND c.deleted_at IS NULL
		   AND c.appointment_calendar_id IS NOT NULL
		   AND bc.schedule::text NOT IN ('{}', 'null')
		   AND EXISTS(
		     SELECT 1 FROM buyer_appointment_slots sl
		     WHERE sl.calendar_id = bc.id AND sl.disabled_at IS NULL
		   )
		 ORDER BY p.name, c.name`, buyerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []BuyerAppointmentContract
	for rows.Next() {
		var c BuyerAppointmentContract
		if err := rows.Scan(&c.ContractID, &c.ContractName, &c.PublisherID, &c.PublisherName, &c.Timezone, &c.Configured); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (s *Service) contractOwnedByBuyer(ctx context.Context, buyerID, contractID int64) error {
	var id int64
	err := s.pool.QueryRow(ctx,
		`SELECT id FROM contracts
		 WHERE id=$1 AND buyer_id=$2 AND lead_type='Appointment' AND status='active' AND deleted_at IS NULL`,
		contractID, buyerID).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return httpx.NotFound("appointment contract not found")
	}
	return err
}

func (s *Service) ListContractSlotsForBuyer(ctx context.Context, buyerID, contractID int64) ([]ContractSlot, error) {
	var leadType string
	var calendarID *int64
	err := s.pool.QueryRow(ctx,
		`SELECT COALESCE(lead_type,''), appointment_calendar_id FROM contracts
		 WHERE id=$1 AND buyer_id=$2 AND deleted_at IS NULL`,
		contractID, buyerID).Scan(&leadType, &calendarID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, httpx.NotFound("contract not found")
	}
	if err != nil {
		return nil, err
	}
	if leadType != "Appointment" {
		return nil, httpx.Validation("contract is not appointment type")
	}
	if calendarID == nil || *calendarID == 0 {
		return nil, httpx.Validation("contract has no appointment calendar")
	}
	if err := s.ensureContractSlots(ctx, contractID); err != nil {
		return nil, err
	}
	rows, err := s.pool.Query(ctx,
		`SELECT sl.id, sl.weekday, sl.start_time::text, sl.duration_min, sl.capacity,
		        COALESCE(cs.enabled, false), cs.duration_min_override, cs.capacity_override,
		        sl.disabled_at IS NOT NULL
		 FROM buyer_appointment_slots sl
		 LEFT JOIN contract_appointment_slots cs ON cs.buyer_slot_id = sl.id AND cs.contract_id = $1
		 WHERE sl.calendar_id = $2
		 ORDER BY sl.weekday, sl.start_time`, contractID, *calendarID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ContractSlot
	for rows.Next() {
		var cs ContractSlot
		var enabled bool
		if err := rows.Scan(&cs.BuyerSlotID, &cs.Weekday, &cs.StartTime, &cs.DurationMin, &cs.Capacity,
			&enabled, &cs.DurationMinOverride, &cs.CapacityOverride, &cs.Disabled); err != nil {
			return nil, err
		}
		cs.Enabled = enabled
		out = append(out, cs)
	}
	return out, rows.Err()
}

func (s *Service) ListFreeSlotsForBuyer(ctx context.Context, buyerID, contractID int64, dateStr string) ([]FreeSlot, error) {
	return s.listFreeSlots(ctx, buyerID, contractID, dateStr, true)
}

func (s *Service) ListCalendarMarkersForBuyer(ctx context.Context, buyerID, contractID int64, fromStr, toStr string) ([]CalendarDayMarker, error) {
	return s.listCalendarMarkers(ctx, buyerID, contractID, fromStr, toStr, true)
}
