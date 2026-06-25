CREATE TABLE buyer_availability (
    account_id  BIGINT PRIMARY KEY REFERENCES accounts(id) ON DELETE CASCADE,
    schedule    JSONB NOT NULL DEFAULT '{}',
    timezone    TEXT NOT NULL DEFAULT 'UTC',
    buffer_min  INT NOT NULL DEFAULT 0,
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT buyer_availability_buffer_check CHECK (buffer_min >= 0 AND buffer_min <= 60)
);

INSERT INTO buyer_availability (account_id, schedule, timezone, buffer_min, updated_at)
SELECT DISTINCT ON (account_id) account_id, schedule, timezone, buffer_min, updated_at
FROM buyer_booking_calendars
ORDER BY account_id, id;

ALTER TABLE contracts DROP COLUMN IF EXISTS appointment_calendar_id;

ALTER TABLE buyer_appointment_slots DROP CONSTRAINT IF EXISTS buyer_appointment_slots_calendar_weekday_start_key;
ALTER TABLE buyer_appointment_slots ADD UNIQUE (account_id, weekday, start_time);

ALTER TABLE buyer_appointment_slots DROP COLUMN IF EXISTS calendar_id;

DROP TABLE IF EXISTS buyer_booking_calendars;
