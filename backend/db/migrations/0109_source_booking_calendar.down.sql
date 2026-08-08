ALTER TABLE lead_appointment_bookings DROP CONSTRAINT lead_appointment_bookings_delivery_check;
ALTER TABLE lead_appointment_bookings ADD CONSTRAINT lead_appointment_bookings_delivery_check
  CHECK (delivery_mode IN ('contract', 'publisher_pipeline'));

ALTER TABLE routing_sources DROP CONSTRAINT routing_sources_delivery_mode_check;
ALTER TABLE routing_sources ADD CONSTRAINT routing_sources_delivery_mode_check
  CHECK (delivery_mode IN ('contract', 'publisher_pipeline'));

ALTER TABLE routing_sources DROP COLUMN IF EXISTS calendar_id;
