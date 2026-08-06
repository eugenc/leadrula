package integrations

import (
	"context"
	"errors"
	"strings"

	"github.com/jackc/pgx/v5"
)

func (s *Service) ghlAppointmentEventID(ctx context.Context, leadID, connID int64) (string, error) {
	if s == nil || s.pool == nil || leadID == 0 || connID == 0 {
		return "", nil
	}
	var eventID string
	err := s.pool.QueryRow(ctx,
		`SELECT event_id FROM lead_integration_appointment_events
		 WHERE lead_id = $1 AND connection_id = $2`,
		leadID, connID).Scan(&eventID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", nil
		}
		return "", err
	}
	return strings.TrimSpace(eventID), nil
}

func (s *Service) setGHLAppointmentEventID(ctx context.Context, leadID, connID int64, eventID string) error {
	eventID = strings.TrimSpace(eventID)
	if s == nil || s.pool == nil || leadID == 0 || connID == 0 || eventID == "" {
		return nil
	}
	_, err := s.pool.Exec(ctx,
		`INSERT INTO lead_integration_appointment_events (lead_id, connection_id, event_id, updated_at)
		 VALUES ($1, $2, $3, now())
		 ON CONFLICT (lead_id, connection_id) DO UPDATE
		   SET event_id = EXCLUDED.event_id, updated_at = now()`,
		leadID, connID, eventID)
	return err
}
