ALTER TABLE lead_appointment_bookings DROP CONSTRAINT IF EXISTS lead_appointment_bookings_slot_check;
DELETE FROM lead_appointment_bookings WHERE buyer_slot_id IS NULL;
ALTER TABLE lead_appointment_bookings DROP COLUMN IF EXISTS publisher_slot_id;
ALTER TABLE lead_appointment_bookings ALTER COLUMN buyer_slot_id SET NOT NULL;

DROP TABLE IF EXISTS contract_publisher_appointment_slots;

ALTER TABLE contracts DROP COLUMN IF EXISTS appointment_calendar_source;
ALTER TABLE contracts DROP COLUMN IF EXISTS publisher_appointment_calendar_id;

DROP TABLE IF EXISTS publisher_appointment_slots;
DROP TABLE IF EXISTS publisher_booking_calendars;
