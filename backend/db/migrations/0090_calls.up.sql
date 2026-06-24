-- Call leads: contract-owned live call routing, billing, and telemetry.
-- Source owns the tracking number + Twilio integration connection. Call settings
-- and per-buyer call targets hang off the existing contract/participation tables.

-- ── Source: tracking number + Twilio integration + optional preload payload ──
ALTER TABLE routing_sources
  ADD COLUMN integration_connection_id BIGINT REFERENCES integration_connections(id) ON DELETE SET NULL,
  ADD COLUMN tracking_number           TEXT,
  ADD COLUMN twilio_sid                TEXT,
  ADD COLUMN payload_enabled           BOOLEAN NOT NULL DEFAULT false,
  ADD COLUMN require_preload           BOOLEAN NOT NULL DEFAULT false;

-- One tracking number maps to exactly one source.
CREATE UNIQUE INDEX idx_routing_sources_tracking_number
  ON routing_sources(tracking_number) WHERE tracking_number IS NOT NULL;

-- ── Allow per_connected_call compensation trigger ──
ALTER TABLE contract_compensations DROP CONSTRAINT contract_compensations_trigger_check;
ALTER TABLE contract_compensations ADD CONSTRAINT contract_compensations_trigger_check
  CHECK (trigger IN ('per_lead', 'buyer_stage', 'manual', 'per_connected_call'));

-- ── Per-contract call policy (1:1 with contract) ──
CREATE TABLE contract_call_settings (
    contract_id            BIGINT PRIMARY KEY REFERENCES contracts(id) ON DELETE CASCADE,
    duration_threshold_sec INT NOT NULL DEFAULT 30,
    tier_timeout_sec       INT NOT NULL DEFAULT 20,
    duplicate_window_hours INT NOT NULL DEFAULT 72,
    mask_caller_id         BOOLEAN NOT NULL DEFAULT false,
    expose_caller_id       BOOLEAN NOT NULL DEFAULT true,
    pass_inbound_payload   BOOLEAN NOT NULL DEFAULT false,
    recording_enabled      BOOLEAN NOT NULL DEFAULT true,
    vertical               TEXT NOT NULL DEFAULT '',
    allowed_states         TEXT[] NOT NULL DEFAULT '{}',
    caller_geo_mode        TEXT NOT NULL DEFAULT 'none'
        CHECK (caller_geo_mode IN ('twilio_lookup', 'area_code', 'none')),
    created_at             TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at             TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- ── Per-buyer call routing target (1:1 with participation) ──
-- Buyer owns target_type/destination/RTB; publisher owns priority/weight/schedule/caps.
CREATE TABLE participation_call_targets (
    participation_id   BIGINT PRIMARY KEY REFERENCES contract_participations(id) ON DELETE CASCADE,
    target_type        TEXT NOT NULL DEFAULT 'static'
        CHECK (target_type IN ('static', 'dynamic')),
    destination_number TEXT,
    rtb_endpoint       TEXT,
    rtb_headers        BYTEA,
    priority           INT NOT NULL DEFAULT 1,
    weight             INT NOT NULL DEFAULT 1,
    rate_override      NUMERIC(14,2),
    schedule           JSONB NOT NULL DEFAULT '{}',
    daily_cap          INT,
    monthly_cap        INT,
    concurrency_cap    INT,
    calls_today        INT NOT NULL DEFAULT 0,
    calls_this_month   INT NOT NULL DEFAULT 0,
    concurrent_calls   INT NOT NULL DEFAULT 0,
    last_cap_reset     DATE,
    created_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at         TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- ── Calls (one row per inbound call) ──
CREATE TABLE calls (
    id                      BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    public_id               UUID NOT NULL DEFAULT gen_random_uuid() UNIQUE,
    publisher_id            BIGINT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    source_id               BIGINT REFERENCES routing_sources(id) ON DELETE SET NULL,
    contract_id             BIGINT REFERENCES contracts(id) ON DELETE SET NULL,
    lead_id                 BIGINT REFERENCES leads(id) ON DELETE SET NULL,
    winner_participation_id BIGINT REFERENCES contract_participations(id) ON DELETE SET NULL,
    twilio_call_sid         TEXT UNIQUE,
    caller_number           TEXT,
    caller_phone_hash       TEXT,
    caller_state            TEXT,
    tracking_number         TEXT,
    status                  TEXT NOT NULL DEFAULT 'inbound'
        CHECK (status IN ('inbound', 'ringing', 'connected', 'completed', 'no_answer', 'blocked', 'failed')),
    disposition             TEXT,
    disposition_note        TEXT,
    billable                BOOLEAN NOT NULL DEFAULT false,
    duration_sec            INT NOT NULL DEFAULT 0,
    billable_duration_sec   INT NOT NULL DEFAULT 0,
    price_cents             INT NOT NULL DEFAULT 0,
    recording_url           TEXT,
    connected_at            TIMESTAMPTZ,
    ended_at                TIMESTAMPTZ,
    created_at              TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at              TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_calls_publisher ON calls(publisher_id, created_at DESC);
CREATE INDEX idx_calls_contract ON calls(contract_id, created_at DESC);
CREATE INDEX idx_calls_lead ON calls(lead_id);

-- ── Call legs (one per dialed buyer attempt; drives waterfall + billing) ──
CREATE TABLE call_legs (
    id                 BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    call_id            BIGINT NOT NULL REFERENCES calls(id) ON DELETE CASCADE,
    participation_id   BIGINT REFERENCES contract_participations(id) ON DELETE SET NULL,
    buyer_id           BIGINT REFERENCES accounts(id) ON DELETE SET NULL,
    tier               INT NOT NULL DEFAULT 1,
    destination_number TEXT,
    twilio_call_sid    TEXT,
    leg_status         TEXT NOT NULL DEFAULT 'dialing'
        CHECK (leg_status IN ('dialing', 'ringing', 'in_progress', 'completed', 'no_answer', 'busy', 'failed', 'canceled')),
    rate               NUMERIC(14,2) NOT NULL DEFAULT 0,
    billed             BOOLEAN NOT NULL DEFAULT false,
    answered_at        TIMESTAMPTZ,
    ended_at           TIMESTAMPTZ,
    duration_sec       INT NOT NULL DEFAULT 0,
    created_at         TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_call_legs_call ON call_legs(call_id);
CREATE UNIQUE INDEX idx_call_legs_sid ON call_legs(twilio_call_sid) WHERE twilio_call_sid IS NOT NULL;

-- ── Duplicate suppression (per source/tracking number, billable connects only) ──
CREATE TABLE call_suppression (
    id                BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    source_id         BIGINT NOT NULL REFERENCES routing_sources(id) ON DELETE CASCADE,
    caller_phone_hash TEXT NOT NULL,
    call_id           BIGINT REFERENCES calls(id) ON DELETE SET NULL,
    expires_at        TIMESTAMPTZ NOT NULL,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_call_suppression_lookup ON call_suppression(source_id, caller_phone_hash, expires_at);

-- ── RTB ping audit (dynamic targets) ──
CREATE TABLE rtb_pings (
    id                 BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    call_id            BIGINT NOT NULL REFERENCES calls(id) ON DELETE CASCADE,
    participation_id   BIGINT REFERENCES contract_participations(id) ON DELETE SET NULL,
    endpoint           TEXT NOT NULL,
    accepted           BOOLEAN NOT NULL DEFAULT false,
    bid_amount         NUMERIC(14,2),
    destination_number TEXT,
    response_status    INT,
    response_body      TEXT,
    reason             TEXT,
    created_at         TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_rtb_pings_call ON rtb_pings(call_id);

-- ── Preloaded caller payload (payload only, no lead; merged on inbound) ──
CREATE TABLE call_preloads (
    id                BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    source_id         BIGINT NOT NULL REFERENCES routing_sources(id) ON DELETE CASCADE,
    caller_phone_hash TEXT,
    preload_token     TEXT NOT NULL,
    raw_payload       JSONB NOT NULL DEFAULT '{}',
    consumed_at       TIMESTAMPTZ,
    expires_at        TIMESTAMPTZ NOT NULL,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_call_preloads_phone ON call_preloads(source_id, caller_phone_hash, expires_at);
CREATE UNIQUE INDEX idx_call_preloads_token ON call_preloads(preload_token);

-- ── Link call charges to the ledger (basis for the "Call" transaction label) ──
ALTER TABLE transactions ADD COLUMN call_id BIGINT REFERENCES calls(id) ON DELETE SET NULL;

-- ── Twilio integration provider (publisher BYO account for call sources) ──
INSERT INTO integration_providers (slug, name, description, auth_type, direction, config_schema)
VALUES (
  'twilio',
  'Twilio',
  'Connect your Twilio account to provision call tracking numbers and route live calls.',
  'api_key',
  'both',
  '{"type":"object","properties":{}}'
)
ON CONFLICT (slug) DO NOTHING;
