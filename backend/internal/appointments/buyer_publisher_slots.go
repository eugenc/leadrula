package appointments

import (
	"context"
	"errors"

	"github.com/echayko/leadrula/backend/pkg/httpx"
	"github.com/jackc/pgx/v5"
)

type SetContractCalendarSourceParams struct {
	Source                string
	AppointmentCalendarID int64
}

func (s *Service) SetContractAppointmentCalendarSource(ctx context.Context, buyerID, contractID int64, p SetContractCalendarSourceParams) error {
	if p.Source != calendarSourceBuyer && p.Source != calendarSourcePublisher {
		return httpx.Validation("source must be buyer or publisher")
	}
	var leadType string
	var pubCalID *int64
	err := s.pool.QueryRow(ctx,
		`SELECT COALESCE(lead_type,''), publisher_appointment_calendar_id FROM contracts
		 WHERE id=$1 AND buyer_id=$2 AND deleted_at IS NULL`,
		contractID, buyerID).Scan(&leadType, &pubCalID)
	if errors.Is(err, pgx.ErrNoRows) {
		return httpx.NotFound("contract not found")
	}
	if err != nil {
		return err
	}
	if leadType != "Appointment" {
		return httpx.Validation("contract is not appointment type")
	}
	switch p.Source {
	case calendarSourceBuyer:
		if p.AppointmentCalendarID == 0 {
			return httpx.Validation("appointment_calendar_id is required")
		}
		if _, err := s.loadCalendar(ctx, buyerID, p.AppointmentCalendarID); err != nil {
			return err
		}
		tag, err := s.pool.Exec(ctx,
			`UPDATE contracts SET appointment_calendar_source = 'buyer', appointment_calendar_id = $3
			 WHERE id = $1 AND buyer_id = $2 AND lead_type = 'Appointment' AND deleted_at IS NULL`,
			contractID, buyerID, p.AppointmentCalendarID)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return httpx.NotFound("contract not found")
		}
		return s.ensureContractSlots(ctx, contractID)
	case calendarSourcePublisher:
		if pubCalID == nil || *pubCalID == 0 {
			return httpx.Validation("publisher has not attached a calendar to this contract")
		}
		tag, err := s.pool.Exec(ctx,
			`UPDATE contracts SET appointment_calendar_source = 'publisher'
			 WHERE id = $1 AND buyer_id = $2 AND lead_type = 'Appointment' AND deleted_at IS NULL`,
			contractID, buyerID)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return httpx.NotFound("contract not found")
		}
		return s.ensureContractPublisherSlots(ctx, contractID)
	default:
		return httpx.Validation("invalid source")
	}
}

func (s *Service) ensureContractPublisherSlots(ctx context.Context, contractID int64) error {
	_, err := s.pool.Exec(ctx,
		`INSERT INTO contract_publisher_appointment_slots(contract_id, publisher_slot_id, enabled)
		 SELECT $1, sl.id, true
		 FROM publisher_appointment_slots sl
		 JOIN contracts c ON c.id = $1
		 WHERE sl.calendar_id = c.publisher_appointment_calendar_id AND sl.disabled_at IS NULL
		 ON CONFLICT DO NOTHING`, contractID)
	return err
}

func (s *Service) syncNewPublisherSlotToContracts(ctx context.Context, publisherID, calendarID, slotID int64) error {
	_, err := s.pool.Exec(ctx,
		`INSERT INTO contract_publisher_appointment_slots(contract_id, publisher_slot_id, enabled)
		 SELECT c.id, $3, true
		 FROM contracts c
		 WHERE c.publisher_id = $1 AND c.lead_type = 'Appointment' AND c.status = 'active'
		   AND c.deleted_at IS NULL AND c.publisher_appointment_calendar_id = $2
		 ON CONFLICT DO NOTHING`, publisherID, calendarID, slotID)
	return err
}

type ContractPublisherSlot struct {
	PublisherSlotID     int64  `json:"publisher_slot_id"`
	Weekday             int    `json:"weekday"`
	StartTime           string `json:"start_time"`
	DurationMin         int    `json:"duration_min"`
	Capacity            int    `json:"capacity"`
	Enabled             bool   `json:"enabled"`
	DurationMinOverride *int   `json:"duration_min_override,omitempty"`
	CapacityOverride    *int   `json:"capacity_override,omitempty"`
	Disabled            bool   `json:"disabled"`
}

func (s *Service) ListContractPublisherSlotsForBuyer(ctx context.Context, buyerID, contractID int64) ([]ContractPublisherSlot, error) {
	row, err := s.loadContractCalendarRow(ctx, contractID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, httpx.NotFound("contract not found")
	}
	if err != nil {
		return nil, err
	}
	ok, err := s.contractAccepted(ctx, contractID)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, httpx.Validation("contract is not accepted yet")
	}
	if row.PublisherCalendarID == nil || *row.PublisherCalendarID == 0 {
		return nil, httpx.Validation("contract has no publisher appointment calendar")
	}
	var ownerBuyerID int64
	err = s.pool.QueryRow(ctx, `SELECT buyer_id FROM contracts WHERE id=$1`, contractID).Scan(&ownerBuyerID)
	if err != nil {
		return nil, err
	}
	if ownerBuyerID != buyerID {
		return nil, httpx.NotFound("contract not found")
	}
	if err := s.ensureContractPublisherSlots(ctx, contractID); err != nil {
		return nil, err
	}
	rows, err := s.pool.Query(ctx,
		`SELECT sl.id, sl.weekday, sl.start_time::text, sl.duration_min, sl.capacity,
		        COALESCE(cs.enabled, false), cs.duration_min_override, cs.capacity_override,
		        sl.disabled_at IS NOT NULL
		 FROM publisher_appointment_slots sl
		 LEFT JOIN contract_publisher_appointment_slots cs ON cs.publisher_slot_id = sl.id AND cs.contract_id = $1
		 WHERE sl.calendar_id = $2
		 ORDER BY sl.weekday, sl.start_time`, contractID, *row.PublisherCalendarID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ContractPublisherSlot
	for rows.Next() {
		var cs ContractPublisherSlot
		var enabled bool
		if err := rows.Scan(&cs.PublisherSlotID, &cs.Weekday, &cs.StartTime, &cs.DurationMin, &cs.Capacity,
			&enabled, &cs.DurationMinOverride, &cs.CapacityOverride, &cs.Disabled); err != nil {
			return nil, err
		}
		cs.Enabled = enabled
		out = append(out, cs)
	}
	return out, rows.Err()
}

type PutContractPublisherSlotsParams struct {
	Slots []struct {
		PublisherSlotID     int64 `json:"publisher_slot_id"`
		Enabled             bool  `json:"enabled"`
		DurationMinOverride *int  `json:"duration_min_override"`
		CapacityOverride    *int  `json:"capacity_override"`
	} `json:"slots"`
}

func (s *Service) PutContractPublisherSlotsForBuyer(ctx context.Context, buyerID, contractID int64, p PutContractPublisherSlotsParams) ([]ContractPublisherSlot, error) {
	slots, err := s.ListContractPublisherSlotsForBuyer(ctx, buyerID, contractID)
	if err != nil {
		return nil, err
	}
	allowed := map[int64]bool{}
	for _, sl := range slots {
		allowed[sl.PublisherSlotID] = true
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	for _, item := range p.Slots {
		if !allowed[item.PublisherSlotID] {
			return nil, httpx.Validation("invalid publisher slot for contract")
		}
		if item.DurationMinOverride != nil && (*item.DurationMinOverride < minDurationMin || *item.DurationMinOverride > maxDurationMin) {
			return nil, httpx.Validation("duration_min_override out of range")
		}
		if item.CapacityOverride != nil && (*item.CapacityOverride < minCapacity || *item.CapacityOverride > maxCapacity) {
			return nil, httpx.Validation("capacity_override out of range")
		}
		_, err := tx.Exec(ctx,
			`INSERT INTO contract_publisher_appointment_slots(contract_id, publisher_slot_id, enabled, duration_min_override, capacity_override)
			 VALUES ($1,$2,$3,$4,$5)
			 ON CONFLICT (contract_id, publisher_slot_id) DO UPDATE SET
			   enabled = EXCLUDED.enabled,
			   duration_min_override = EXCLUDED.duration_min_override,
			   capacity_override = EXCLUDED.capacity_override`,
			contractID, item.PublisherSlotID, item.Enabled, item.DurationMinOverride, item.CapacityOverride)
		if err != nil {
			return nil, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return s.ListContractPublisherSlotsForBuyer(ctx, buyerID, contractID)
}

func effectivePublisherCapacity(slot PublisherSlot, cs *ContractPublisherSlot) int {
	if cs != nil && cs.CapacityOverride != nil {
		return *cs.CapacityOverride
	}
	return slot.Capacity
}

func effectivePublisherDuration(slot PublisherSlot, cs *ContractPublisherSlot) int {
	if cs != nil && cs.DurationMinOverride != nil {
		return *cs.DurationMinOverride
	}
	return slot.DurationMin
}
