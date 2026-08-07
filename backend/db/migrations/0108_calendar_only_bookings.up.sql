ALTER TABLE lead_appointment_bookings DROP CONSTRAINT IF EXISTS lead_appointment_bookings_contract_id_lead_id_key;

ALTER TABLE lead_appointment_bookings ALTER COLUMN contract_id DROP NOT NULL;

ALTER TABLE lead_appointment_bookings
    ADD COLUMN publisher_calendar_id BIGINT REFERENCES publisher_booking_calendars(id) ON DELETE CASCADE,
    ADD COLUMN buyer_calendar_id BIGINT REFERENCES buyer_booking_calendars(id) ON DELETE CASCADE;

ALTER TABLE lead_appointment_bookings
    ADD CONSTRAINT lead_appointment_bookings_lead_id_key UNIQUE (lead_id);

ALTER TABLE lead_appointment_bookings
    ADD CONSTRAINT lead_appointment_bookings_context_check CHECK (
        (contract_id IS NOT NULL AND publisher_calendar_id IS NULL AND buyer_calendar_id IS NULL)
        OR (contract_id IS NULL AND publisher_calendar_id IS NOT NULL AND buyer_calendar_id IS NULL AND publisher_slot_id IS NOT NULL)
        OR (contract_id IS NULL AND buyer_calendar_id IS NOT NULL AND publisher_calendar_id IS NULL AND buyer_slot_id IS NOT NULL)
    );

CREATE INDEX lead_appointment_bookings_publisher_calendar_slot_idx
    ON lead_appointment_bookings(publisher_calendar_id, publisher_slot_id, slot_start)
    WHERE publisher_calendar_id IS NOT NULL;

CREATE INDEX lead_appointment_bookings_buyer_calendar_slot_idx
    ON lead_appointment_bookings(buyer_calendar_id, buyer_slot_id, slot_start)
    WHERE buyer_calendar_id IS NOT NULL;
