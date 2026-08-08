ALTER TABLE contracts
    ADD COLUMN IF NOT EXISTS schedule_timezone TEXT NOT NULL DEFAULT 'UTC';

UPDATE contracts SET schedule_timezone = 'America/New_York'
WHERE schedule_timezone = 'UTC';

ALTER TABLE contract_return_rules
    ADD COLUMN IF NOT EXISTS return_schedule_mode TEXT NOT NULL DEFAULT 'immediate',
    ADD COLUMN IF NOT EXISTS return_delay_seconds INT,
    ADD COLUMN IF NOT EXISTS return_time TIME,
    ADD COLUMN IF NOT EXISTS return_weekdays SMALLINT[];

ALTER TABLE contract_return_rules
    ADD CONSTRAINT contract_return_rules_schedule_mode_check
    CHECK (return_schedule_mode IN ('immediate', 'delay', 'daily', 'weekly'));

CREATE TABLE scheduled_lead_returns (
    id              BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    lead_id         BIGINT NOT NULL REFERENCES leads(id) ON DELETE CASCADE,
    contract_id     BIGINT NOT NULL REFERENCES contracts(id) ON DELETE CASCADE,
    return_rule_id  BIGINT NOT NULL REFERENCES contract_return_rules(id) ON DELETE CASCADE,
    buyer_stage_id  BIGINT NOT NULL REFERENCES pipeline_stages(id),
    return_stage_id BIGINT NOT NULL REFERENCES pipeline_stages(id),
    execute_at      TIMESTAMPTZ NOT NULL,
    status          TEXT NOT NULL DEFAULT 'pending'
        CHECK (status IN ('pending', 'processing', 'completed', 'cancelled')),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_scheduled_lead_returns_pending
    ON scheduled_lead_returns(status, execute_at)
    WHERE status = 'pending';

CREATE INDEX idx_scheduled_lead_returns_lead_pending
    ON scheduled_lead_returns(lead_id)
    WHERE status = 'pending';
