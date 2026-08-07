ALTER TABLE lead_appointment_bookings
    ADD COLUMN custom_time BOOLEAN NOT NULL DEFAULT false;

ALTER TABLE lead_appointment_bookings DROP CONSTRAINT IF EXISTS lead_appointment_bookings_slot_check;
ALTER TABLE lead_appointment_bookings ADD CONSTRAINT lead_appointment_bookings_slot_check CHECK (
    custom_time = true
    OR (buyer_slot_id IS NOT NULL AND publisher_slot_id IS NULL)
    OR (buyer_slot_id IS NULL AND publisher_slot_id IS NOT NULL)
);

ALTER TABLE lead_appointment_bookings DROP CONSTRAINT IF EXISTS lead_appointment_bookings_context_check;
ALTER TABLE lead_appointment_bookings ADD CONSTRAINT lead_appointment_bookings_context_check CHECK (
    (custom_time = true AND contract_id IS NOT NULL AND publisher_calendar_id IS NULL AND buyer_calendar_id IS NULL)
    OR (custom_time = true AND contract_id IS NULL AND publisher_calendar_id IS NOT NULL AND buyer_calendar_id IS NULL)
    OR (custom_time = true AND contract_id IS NULL AND buyer_calendar_id IS NOT NULL AND publisher_calendar_id IS NULL)
    OR (custom_time = false AND contract_id IS NOT NULL AND publisher_calendar_id IS NULL AND buyer_calendar_id IS NULL)
    OR (custom_time = false AND contract_id IS NULL AND publisher_calendar_id IS NOT NULL AND buyer_calendar_id IS NULL AND publisher_slot_id IS NOT NULL)
    OR (custom_time = false AND contract_id IS NULL AND buyer_calendar_id IS NOT NULL AND publisher_calendar_id IS NULL AND buyer_slot_id IS NOT NULL)
);
