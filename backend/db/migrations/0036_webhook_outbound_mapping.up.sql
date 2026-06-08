ALTER TABLE webhooks
    ADD COLUMN outbound_payload_template TEXT NOT NULL DEFAULT '{}',
    ADD COLUMN outbound_response_map JSONB NOT NULL DEFAULT '[]';

UPDATE webhooks w SET
    outbound_payload_template = t.payload_template,
    outbound_response_map = t.response_map
FROM (
    SELECT DISTINCT ON (webhook_id) webhook_id, payload_template, response_map
    FROM webhook_outbound_triggers
    ORDER BY webhook_id, id DESC
) t
WHERE w.id = t.webhook_id;

ALTER TABLE webhook_outbound_triggers
    DROP COLUMN payload_template,
    DROP COLUMN response_map;
