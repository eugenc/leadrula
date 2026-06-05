ALTER TABLE webhook_outbound_triggers
    ADD COLUMN IF NOT EXISTS response_map JSONB NOT NULL DEFAULT '[]';
