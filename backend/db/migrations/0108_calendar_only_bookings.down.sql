DROP INDEX IF EXISTS lead_appointment_bookings_buyer_calendar_slot_idx;
DROP INDEX IF EXISTS lead_appointment_bookings_publisher_calendar_slot_idx;

ALTER TABLE lead_appointment_bookings DROP CONSTRAINT IF EXISTS lead_appointment_bookings_context_check;
ALTER TABLE lead_appointment_bookings DROP CONSTRAINT IF EXISTS lead_appointment_bookings_lead_id_key;

ALTER TABLE lead_appointment_bookings DROP COLUMN IF EXISTS buyer_calendar_id;
ALTER TABLE lead_appointment_bookings DROP COLUMN IF EXISTS publisher_calendar_id;

DELETE FROM lead_appointment_bookings WHERE contract_id IS NULL;

ALTER TABLE lead_appointment_bookings ALTER COLUMN contract_id SET NOT NULL;

ALTER TABLE lead_appointment_bookings
    ADD CONSTRAINT lead_appointment_bookings_contract_id_lead_id_key UNIQUE (contract_id, lead_id);
