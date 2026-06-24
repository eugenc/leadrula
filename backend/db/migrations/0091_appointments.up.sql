CREATE TABLE buyer_availability (
    account_id  BIGINT PRIMARY KEY REFERENCES accounts(id) ON DELETE CASCADE,
    schedule    JSONB NOT NULL DEFAULT '{}',
    timezone    TEXT NOT NULL DEFAULT 'UTC',
    buffer_min  INT NOT NULL DEFAULT 0,
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT buyer_availability_buffer_check CHECK (buffer_min >= 0 AND buffer_min <= 60)
);

CREATE TABLE buyer_appointment_slots (
    id           BIGSERIAL PRIMARY KEY,
    account_id   BIGINT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    weekday      SMALLINT NOT NULL,
    start_time   TIME NOT NULL,
    duration_min INT NOT NULL DEFAULT 30,
    capacity     INT NOT NULL DEFAULT 1,
    disabled_at  TIMESTAMPTZ,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT buyer_appointment_slots_weekday_check CHECK (weekday >= 0 AND weekday <= 6),
    CONSTRAINT buyer_appointment_slots_duration_check CHECK (duration_min >= 15 AND duration_min <= 240),
    CONSTRAINT buyer_appointment_slots_capacity_check CHECK (capacity >= 1 AND capacity <= 20),
    UNIQUE (account_id, weekday, start_time)
);

CREATE INDEX buyer_appointment_slots_account_idx ON buyer_appointment_slots(account_id);

CREATE TABLE contract_appointment_slots (
    contract_id            BIGINT NOT NULL REFERENCES contracts(id) ON DELETE CASCADE,
    buyer_slot_id          BIGINT NOT NULL REFERENCES buyer_appointment_slots(id) ON DELETE CASCADE,
    duration_min_override  INT,
    capacity_override      INT,
    enabled                BOOLEAN NOT NULL DEFAULT true,
    PRIMARY KEY (contract_id, buyer_slot_id),
    CONSTRAINT contract_appointment_slots_duration_check
        CHECK (duration_min_override IS NULL OR (duration_min_override >= 15 AND duration_min_override <= 240)),
    CONSTRAINT contract_appointment_slots_capacity_check
        CHECK (capacity_override IS NULL OR (capacity_override >= 1 AND capacity_override <= 20))
);

CREATE TABLE lead_appointment_bookings (
    id               BIGSERIAL PRIMARY KEY,
    contract_id      BIGINT NOT NULL REFERENCES contracts(id) ON DELETE CASCADE,
    lead_id          BIGINT NOT NULL REFERENCES leads(id) ON DELETE CASCADE,
    buyer_slot_id    BIGINT NOT NULL REFERENCES buyer_appointment_slots(id),
    slot_start       TIMESTAMPTZ NOT NULL,
    duration_min     INT NOT NULL,
    booked_by_user_id BIGINT NOT NULL REFERENCES users(id),
    delivery_mode    TEXT NOT NULL,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT lead_appointment_bookings_delivery_check
        CHECK (delivery_mode IN ('contract', 'publisher_pipeline')),
    UNIQUE (contract_id, lead_id)
);

CREATE INDEX lead_appointment_bookings_contract_slot_idx
    ON lead_appointment_bookings(contract_id, slot_start);

CREATE INDEX lead_appointment_bookings_lead_idx ON lead_appointment_bookings(lead_id);
