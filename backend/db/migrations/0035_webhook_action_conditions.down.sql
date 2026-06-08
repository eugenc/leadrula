ALTER TABLE webhook_events ADD COLUMN event_key TEXT;

UPDATE webhook_events
SET event_key = COALESCE(conditions->0->>'value', '')
WHERE jsonb_array_length(conditions) > 0 AND conditions->0->>'field' = 'event';

UPDATE webhook_events SET event_key = '' WHERE event_key IS NULL;
ALTER TABLE webhook_events ALTER COLUMN event_key SET NOT NULL;
ALTER TABLE webhook_events ADD CONSTRAINT webhook_events_webhook_id_event_key_key UNIQUE (webhook_id, event_key);

ALTER TABLE webhook_events DROP COLUMN lookup_source_key;
ALTER TABLE webhook_events DROP COLUMN conditions;
ALTER TABLE webhook_events DROP COLUMN condition_logic;

-- phone/email enum values cannot be removed in PostgreSQL without recreating the type
