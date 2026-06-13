DROP INDEX IF EXISTS idx_webhooks_integration_connection;
ALTER TABLE webhooks DROP COLUMN IF EXISTS integration_connection_id;
