CREATE TABLE IF NOT EXISTS crm_inbound_stage_sync_retries (
    id              BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    account_id      BIGINT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    lead_id         BIGINT NOT NULL REFERENCES leads(id) ON DELETE CASCADE,
    connection_id   BIGINT NOT NULL REFERENCES integration_connections(id) ON DELETE CASCADE,
    webhook_id      BIGINT NOT NULL REFERENCES webhooks(id) ON DELETE CASCADE,
    payload         JSONB NOT NULL,
    attempts        INT NOT NULL DEFAULT 0,
    max_attempts    INT NOT NULL DEFAULT 3,
    next_attempt_at TIMESTAMPTZ NOT NULL DEFAULT now() + interval '5 seconds',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (lead_id, connection_id)
);

CREATE INDEX IF NOT EXISTS idx_crm_inbound_sync_retry_due
    ON crm_inbound_stage_sync_retries (next_attempt_at)
    WHERE attempts < max_attempts;
