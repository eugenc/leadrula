-- Extend webhooks table for outbound direction
ALTER TABLE webhooks
    ADD COLUMN IF NOT EXISTS inbound_enabled  BOOLEAN NOT NULL DEFAULT true,
    ADD COLUMN IF NOT EXISTS outbound_enabled BOOLEAN NOT NULL DEFAULT false,
    ADD COLUMN IF NOT EXISTS outbound_url     TEXT,
    ADD COLUMN IF NOT EXISTS outbound_secret  BYTEA,
    ADD COLUMN IF NOT EXISTS outbound_connection_id BIGINT REFERENCES integration_connections(id) ON DELETE SET NULL;

-- Outbound trigger events
CREATE TYPE outbound_trigger_event AS ENUM (
    'lead.create',
    'lead.update',
    'lead.delete',
    'pipeline.move_stage',
    'pipeline.place',
    'pipeline.stage_rule_applied'
);

CREATE TABLE webhook_outbound_triggers (
    id               BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    webhook_id       BIGINT NOT NULL REFERENCES webhooks(id) ON DELETE CASCADE,
    trigger_event    outbound_trigger_event NOT NULL,
    condition_logic  TEXT NOT NULL DEFAULT 'and',
    conditions       JSONB NOT NULL DEFAULT '[]',
    payload_template TEXT NOT NULL DEFAULT '{}',
    position         INT NOT NULL DEFAULT 0,
    is_active        BOOLEAN NOT NULL DEFAULT true,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_outbound_triggers_webhook ON webhook_outbound_triggers(webhook_id);
CREATE INDEX idx_outbound_triggers_event ON webhook_outbound_triggers(trigger_event) WHERE is_active;

-- Extend delivery queue to track webhook trigger source
ALTER TABLE integration_delivery_queue
    ADD COLUMN IF NOT EXISTS webhook_trigger_id BIGINT REFERENCES webhook_outbound_triggers(id) ON DELETE SET NULL;
CREATE INDEX IF NOT EXISTS idx_delivery_queue_trigger ON integration_delivery_queue(webhook_trigger_id)
    WHERE webhook_trigger_id IS NOT NULL;
