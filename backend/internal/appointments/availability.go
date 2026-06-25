package appointments

import "context"

type CreateSlotParams struct {
	Weekday     int    `json:"weekday"`
	StartTime   string `json:"start_time"`
	DurationMin int    `json:"duration_min"`
	Capacity    int    `json:"capacity"`
}

type PatchSlotParams struct {
	StartTime   *string `json:"start_time"`
	DurationMin *int    `json:"duration_min"`
	Capacity    *int    `json:"capacity"`
	Disabled    *bool   `json:"disabled"`
}

func nullStrPtr(s *string) interface{} {
	if s == nil {
		return nil
	}
	return *s
}

func (s *Service) syncNewSlotToContracts(ctx context.Context, buyerID, calendarID, slotID int64) error {
	_, err := s.pool.Exec(ctx,
		`INSERT INTO contract_appointment_slots(contract_id, buyer_slot_id, enabled)
		 SELECT c.id, $3, true
		 FROM contracts c
		 WHERE c.buyer_id = $1 AND c.lead_type = 'Appointment' AND c.status = 'active'
		   AND c.deleted_at IS NULL AND c.appointment_calendar_id = $2
		 ON CONFLICT DO NOTHING`, buyerID, calendarID, slotID)
	return err
}
