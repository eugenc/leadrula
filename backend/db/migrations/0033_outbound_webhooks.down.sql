DROP INDEX IF EXISTS idx_delivery_queue_trigger;
ALTER TABLE integration_delivery_queue DROP COLUMN IF EXISTS webhook_trigger_id;
DROP TABLE IF EXISTS webhook_outbound_triggers;
DROP TYPE IF EXISTS outbound_trigger_event;
ALTER TABLE webhooks
    DROP COLUMN IF EXISTS outbound_connection_id,
    DROP COLUMN IF EXISTS outbound_secret,
    DROP COLUMN IF EXISTS outbound_url,
    DROP COLUMN IF EXISTS outbound_enabled,
    DROP COLUMN IF EXISTS inbound_enabled;
