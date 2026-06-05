-- Lead external_id and soft delete
ALTER TABLE leads ADD COLUMN IF NOT EXISTS external_id TEXT;
ALTER TABLE leads ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMPTZ;

CREATE UNIQUE INDEX IF NOT EXISTS idx_leads_external_id
    ON leads (owner_account_id, external_id)
    WHERE deleted_at IS NULL AND external_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_leads_not_deleted ON leads (owner_account_id) WHERE deleted_at IS NULL;

-- Webhook action types
CREATE TYPE webhook_action AS ENUM ('create', 'update', 'delete', 'move_stage');
CREATE TYPE webhook_duplicate_mode AS ENUM ('update', 'duplicate', 'reject');
CREATE TYPE webhook_lookup_by AS ENUM ('external_id', 'public_id');
CREATE TYPE webhook_delivery_status AS ENUM ('success', 'error', 'skipped');

CREATE TABLE webhooks (
    id            BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    account_id    BIGINT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    name          TEXT NOT NULL,
    slug          TEXT NOT NULL UNIQUE,
    secret_hash   TEXT NOT NULL,
    secret_prefix TEXT NOT NULL,
    event_field   TEXT NOT NULL DEFAULT 'event',
    is_active     BOOLEAN NOT NULL DEFAULT true,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_webhooks_account ON webhooks(account_id);

CREATE TABLE webhook_events (
    id                   BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    webhook_id           BIGINT NOT NULL REFERENCES webhooks(id) ON DELETE CASCADE,
    event_key            TEXT NOT NULL,
    action               webhook_action NOT NULL,
    duplicate_mode       webhook_duplicate_mode,
    lookup_by            webhook_lookup_by,
    target_stage_id      BIGINT REFERENCES pipeline_stages(id),
    target_pipeline_id   BIGINT REFERENCES pipelines(id),
    position             INT NOT NULL DEFAULT 0,
    created_at           TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (webhook_id, event_key),
    CHECK (
        (action = 'create' AND duplicate_mode IS NOT NULL)
        OR (action != 'create' AND duplicate_mode IS NULL)
    ),
    CHECK (
        (action IN ('update', 'delete', 'move_stage') AND lookup_by IS NOT NULL)
        OR (action = 'create' AND lookup_by IS NULL)
    ),
    CHECK (
        (action = 'move_stage' AND target_stage_id IS NOT NULL)
        OR (action != 'move_stage' AND target_stage_id IS NULL)
    )
);
CREATE INDEX idx_webhook_events_webhook ON webhook_events(webhook_id);

CREATE TABLE webhook_event_field_map (
    id              BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    event_id        BIGINT NOT NULL REFERENCES webhook_events(id) ON DELETE CASCADE,
    source_key      TEXT NOT NULL,
    target_type     map_target NOT NULL,
    builtin_field   TEXT,
    custom_field_id BIGINT REFERENCES custom_fields(id),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK ( (target_type='builtin' AND builtin_field IS NOT NULL AND custom_field_id IS NULL)
         OR (target_type='custom'  AND custom_field_id IS NOT NULL AND builtin_field IS NULL) )
);
CREATE INDEX idx_webhook_event_fieldmap ON webhook_event_field_map(event_id);

CREATE TABLE webhook_deliveries (
    id              BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    webhook_id      BIGINT NOT NULL REFERENCES webhooks(id) ON DELETE CASCADE,
    event_id        BIGINT REFERENCES webhook_events(id) ON DELETE SET NULL,
    lead_id         BIGINT REFERENCES leads(id) ON DELETE SET NULL,
    status          webhook_delivery_status NOT NULL,
    request_payload JSONB NOT NULL DEFAULT '{}',
    error_message   TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_webhook_deliveries_webhook ON webhook_deliveries(webhook_id, created_at DESC);
