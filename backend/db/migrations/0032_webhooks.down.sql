DROP TABLE IF EXISTS webhook_deliveries;
DROP TABLE IF EXISTS webhook_event_field_map;
DROP TABLE IF EXISTS webhook_events;
DROP TABLE IF EXISTS webhooks;
DROP TYPE IF EXISTS webhook_delivery_status;
DROP TYPE IF EXISTS webhook_lookup_by;
DROP TYPE IF EXISTS webhook_duplicate_mode;
DROP TYPE IF EXISTS webhook_action;

DROP INDEX IF EXISTS idx_leads_not_deleted;
DROP INDEX IF EXISTS idx_leads_external_id;
ALTER TABLE leads DROP COLUMN IF EXISTS deleted_at;
ALTER TABLE leads DROP COLUMN IF EXISTS external_id;
