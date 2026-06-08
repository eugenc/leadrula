ALTER TABLE webhook_outbound_triggers
    ADD COLUMN payload_template TEXT NOT NULL DEFAULT '{}',
    ADD COLUMN response_map JSONB NOT NULL DEFAULT '[]';

UPDATE webhook_outbound_triggers t SET
    payload_template = w.outbound_payload_template,
    response_map = w.outbound_response_map
FROM webhooks w
WHERE t.webhook_id = w.id;

ALTER TABLE webhooks
    DROP COLUMN outbound_payload_template,
    DROP COLUMN outbound_response_map;
