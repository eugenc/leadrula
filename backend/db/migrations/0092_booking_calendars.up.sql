CREATE TABLE buyer_booking_calendars (
    id          BIGSERIAL PRIMARY KEY,
    account_id  BIGINT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    name        TEXT NOT NULL,
    schedule    JSONB NOT NULL DEFAULT '{}',
    timezone    TEXT NOT NULL DEFAULT 'UTC',
    buffer_min  INT NOT NULL DEFAULT 0,
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT buyer_booking_calendars_buffer_check CHECK (buffer_min >= 0 AND buffer_min <= 60)
);

CREATE INDEX buyer_booking_calendars_account_idx ON buyer_booking_calendars(account_id);

INSERT INTO buyer_booking_calendars (account_id, name, schedule, timezone, buffer_min, updated_at)
SELECT account_id, 'Default', schedule::jsonb, timezone, buffer_min, updated_at
FROM buyer_availability;

INSERT INTO buyer_booking_calendars (account_id, name, schedule, timezone, buffer_min)
SELECT DISTINCT s.account_id, 'Default', '{}'::jsonb, COALESCE(a.timezone, 'UTC'), 0
FROM buyer_appointment_slots s
JOIN accounts a ON a.id = s.account_id
WHERE NOT EXISTS (SELECT 1 FROM buyer_booking_calendars bc WHERE bc.account_id = s.account_id);

ALTER TABLE buyer_appointment_slots ADD COLUMN calendar_id BIGINT REFERENCES buyer_booking_calendars(id) ON DELETE CASCADE;

UPDATE buyer_appointment_slots s
SET calendar_id = bc.id
FROM buyer_booking_calendars bc
WHERE bc.account_id = s.account_id;

ALTER TABLE buyer_appointment_slots ALTER COLUMN calendar_id SET NOT NULL;

ALTER TABLE buyer_appointment_slots DROP CONSTRAINT buyer_appointment_slots_account_id_weekday_start_time_key;
ALTER TABLE buyer_appointment_slots ADD CONSTRAINT buyer_appointment_slots_calendar_weekday_start_key
    UNIQUE (calendar_id, weekday, start_time);

ALTER TABLE contracts ADD COLUMN appointment_calendar_id BIGINT REFERENCES buyer_booking_calendars(id);

UPDATE contracts c
SET appointment_calendar_id = bc.id
FROM buyer_booking_calendars bc
WHERE c.buyer_id = bc.account_id AND c.lead_type = 'Appointment';

DROP TABLE buyer_availability;
