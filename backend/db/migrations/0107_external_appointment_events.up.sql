ALTER TABLE lead_integration_appointment_events RENAME TO lead_external_appointment_events;
ALTER TABLE lead_external_appointment_events RENAME COLUMN event_id TO external_event_id;
ALTER TABLE lead_external_appointment_events ADD COLUMN provider_slug TEXT NOT NULL DEFAULT 'ghl';

ALTER INDEX lead_integration_appointment_events_connection_idx
    RENAME TO lead_external_appointment_events_connection_idx;

ALTER TABLE lead_appointment_bookings ADD COLUMN IF NOT EXISTS external_event_id TEXT;
ALTER TABLE lead_appointment_bookings ADD COLUMN IF NOT EXISTS external_provider_slug TEXT;

CREATE UNIQUE INDEX IF NOT EXISTS lead_appointment_bookings_external_uidx
    ON lead_appointment_bookings (external_provider_slug, external_event_id)
    WHERE external_event_id IS NOT NULL AND external_event_id <> '';
