ALTER TABLE routing_sources
  ADD COLUMN calendar_id BIGINT REFERENCES publisher_booking_calendars(id) ON DELETE SET NULL;

ALTER TABLE routing_sources DROP CONSTRAINT routing_sources_delivery_mode_check;
ALTER TABLE routing_sources ADD CONSTRAINT routing_sources_delivery_mode_check
  CHECK (delivery_mode IN ('contract', 'publisher_pipeline', 'publisher'));

ALTER TABLE lead_appointment_bookings DROP CONSTRAINT lead_appointment_bookings_delivery_check;
ALTER TABLE lead_appointment_bookings ADD CONSTRAINT lead_appointment_bookings_delivery_check
  CHECK (delivery_mode IN ('contract', 'publisher_pipeline', 'publisher'));

UPDATE routing_sources rs
SET calendar_id = c.publisher_appointment_calendar_id,
    contract_id = NULL
FROM contracts c
WHERE rs.type = 'appointment'
  AND rs.delivery_mode IN ('publisher_pipeline', 'publisher')
  AND rs.contract_id = c.id
  AND rs.calendar_id IS NULL
  AND c.publisher_appointment_calendar_id IS NOT NULL;
