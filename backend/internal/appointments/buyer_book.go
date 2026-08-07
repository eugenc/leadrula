package appointments

import (
	"context"
	"errors"

	"github.com/echayko/leadrula/backend/pkg/httpx"
	"github.com/jackc/pgx/v5"
)

type BuyerAppointmentContract struct {
	ContractID             int64  `json:"contract_id"`
	ContractName           string `json:"contract_name"`
	PublisherID            int64  `json:"publisher_id"`
	PublisherName          string `json:"publisher_name"`
	Timezone               string `json:"timezone"`
	Configured             bool   `json:"configured"`
	OwnConfigured          bool   `json:"own_configured"`
	CounterpartyConfigured bool   `json:"counterparty_configured"`
	CalendarSource         string `json:"calendar_source,omitempty"`
}

func (s *Service) ListBuyerAppointmentContracts(ctx context.Context, buyerID int64) ([]BuyerAppointmentContract, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT c.id, c.name, c.publisher_id, COALESCE(p.name, ''),
		        CASE
		          WHEN bc.id IS NOT NULL THEN COALESCE(bc.timezone, b.timezone, 'UTC')
		          WHEN pc.id IS NOT NULL THEN COALESCE(pc.timezone, p.timezone, 'UTC')
		          ELSE COALESCE(b.timezone, 'UTC')
		        END,
		        CASE WHEN bc.id IS NOT NULL THEN 'buyer' WHEN pc.id IS NOT NULL THEN 'publisher' ELSE '' END,
		        (bc.id IS NOT NULL AND bc.schedule::text NOT IN ('{}', 'null')
		         AND EXISTS(SELECT 1 FROM buyer_appointment_slots sl
		                    WHERE sl.calendar_id = bc.id AND sl.disabled_at IS NULL)),
		        (c.status = 'active'
		         AND pc.id IS NOT NULL AND pc.schedule::text NOT IN ('{}', 'null')
		         AND EXISTS(SELECT 1 FROM publisher_appointment_slots sl
		                    WHERE sl.calendar_id = pc.id AND sl.disabled_at IS NULL))
		 FROM contracts c
		 JOIN accounts b ON b.id = c.buyer_id
		 JOIN accounts p ON p.id = c.publisher_id
		 LEFT JOIN buyer_booking_calendars bc ON bc.id = c.appointment_calendar_id
		 LEFT JOIN publisher_booking_calendars pc ON pc.id = c.publisher_appointment_calendar_id
		 WHERE c.buyer_id = $1 AND c.lead_type = 'Appointment' AND c.status = 'active'
		   AND c.deleted_at IS NULL
		   AND (bc.id IS NOT NULL OR pc.id IS NOT NULL)
		 ORDER BY p.name, c.name`, buyerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []BuyerAppointmentContract
	for rows.Next() {
		var c BuyerAppointmentContract
		if err := rows.Scan(&c.ContractID, &c.ContractName, &c.PublisherID, &c.PublisherName,
			&c.Timezone, &c.CalendarSource, &c.OwnConfigured, &c.CounterpartyConfigured); err != nil {
			return nil, err
		}
		c.Configured = c.OwnConfigured || c.CounterpartyConfigured
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
	err := s.pool.QueryRow(ctx,
		`SELECT COALESCE(lead_type,'') FROM contracts
		 WHERE id=$1 AND buyer_id=$2 AND deleted_at IS NULL`,
		contractID, buyerID).Scan(&leadType)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, httpx.NotFound("contract not found")
	}
	if err != nil {
		return nil, err
	}
	if leadType != "Appointment" {
		return nil, httpx.Validation("contract is not appointment type")
	}
	return s.listOwnBuyerCalendarSlots(ctx, buyerID, contractID, true)
}

func (s *Service) ListFreeSlotsForBuyer(ctx context.Context, buyerID, contractID int64, dateStr, target string) ([]FreeSlot, error) {
	return s.listFreeSlots(ctx, buyerID, contractID, dateStr, true, target)
}

func (s *Service) ListCalendarMarkersForBuyer(ctx context.Context, buyerID, contractID int64, fromStr, toStr, target string) ([]CalendarDayMarker, error) {
	return s.listCalendarMarkers(ctx, buyerID, contractID, fromStr, toStr, true, target)
}
