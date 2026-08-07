CREATE TABLE publisher_booking_calendars (
    id          BIGSERIAL PRIMARY KEY,
    account_id  BIGINT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    name        TEXT NOT NULL,
    schedule    JSONB NOT NULL DEFAULT '{}',
    timezone    TEXT NOT NULL DEFAULT 'UTC',
    buffer_min  INT NOT NULL DEFAULT 0,
    location    TEXT NOT NULL DEFAULT '',
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT publisher_booking_calendars_buffer_check CHECK (buffer_min >= 0 AND buffer_min <= 60)
);

CREATE INDEX publisher_booking_calendars_account_idx ON publisher_booking_calendars(account_id);

CREATE TABLE publisher_appointment_slots (
    id           BIGSERIAL PRIMARY KEY,
    account_id   BIGINT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    calendar_id  BIGINT NOT NULL REFERENCES publisher_booking_calendars(id) ON DELETE CASCADE,
    weekday      SMALLINT NOT NULL,
    start_time   TIME NOT NULL,
    duration_min INT NOT NULL DEFAULT 30,
    capacity     INT NOT NULL DEFAULT 1,
    disabled_at  TIMESTAMPTZ,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT publisher_appointment_slots_weekday_check CHECK (weekday >= 0 AND weekday <= 6),
    CONSTRAINT publisher_appointment_slots_duration_check CHECK (duration_min >= 15 AND duration_min <= 180),
    CONSTRAINT publisher_appointment_slots_capacity_check CHECK (capacity >= 1 AND capacity <= 20),
    UNIQUE (calendar_id, weekday, start_time)
);

CREATE INDEX publisher_appointment_slots_calendar_idx ON publisher_appointment_slots(calendar_id);

ALTER TABLE contracts ADD COLUMN publisher_appointment_calendar_id BIGINT REFERENCES publisher_booking_calendars(id);
ALTER TABLE contracts ADD COLUMN appointment_calendar_source TEXT
    CHECK (appointment_calendar_source IS NULL OR appointment_calendar_source IN ('buyer', 'publisher'));

UPDATE contracts
SET appointment_calendar_source = 'buyer'
WHERE appointment_calendar_id IS NOT NULL;

CREATE TABLE contract_publisher_appointment_slots (
    contract_id             BIGINT NOT NULL REFERENCES contracts(id) ON DELETE CASCADE,
    publisher_slot_id       BIGINT NOT NULL REFERENCES publisher_appointment_slots(id) ON DELETE CASCADE,
    duration_min_override   INT,
    capacity_override       INT,
    enabled                 BOOLEAN NOT NULL DEFAULT true,
    PRIMARY KEY (contract_id, publisher_slot_id),
    CONSTRAINT contract_publisher_appointment_slots_duration_check
        CHECK (duration_min_override IS NULL OR (duration_min_override >= 15 AND duration_min_override <= 180)),
    CONSTRAINT contract_publisher_appointment_slots_capacity_check
        CHECK (capacity_override IS NULL OR (capacity_override >= 1 AND capacity_override <= 20))
);

ALTER TABLE lead_appointment_bookings ADD COLUMN publisher_slot_id BIGINT REFERENCES publisher_appointment_slots(id);
ALTER TABLE lead_appointment_bookings ALTER COLUMN buyer_slot_id DROP NOT NULL;

ALTER TABLE lead_appointment_bookings ADD CONSTRAINT lead_appointment_bookings_slot_check
    CHECK (
        (buyer_slot_id IS NOT NULL AND publisher_slot_id IS NULL)
        OR (buyer_slot_id IS NULL AND publisher_slot_id IS NOT NULL)
    );
