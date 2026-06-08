ALTER TABLE webhook_events
  ADD COLUMN condition_logic TEXT NOT NULL DEFAULT 'and' CHECK (condition_logic IN ('and', 'or')),
  ADD COLUMN conditions JSONB NOT NULL DEFAULT '[]',
  ADD COLUMN lookup_source_key TEXT;

UPDATE webhook_events
SET conditions = jsonb_build_array(jsonb_build_object('field', 'event', 'op', 'eq', 'value', event_key))
WHERE event_key IS NOT NULL AND event_key != '' AND conditions = '[]'::jsonb;

ALTER TABLE webhook_events DROP CONSTRAINT webhook_events_webhook_id_event_key_key;
ALTER TABLE webhook_events DROP COLUMN event_key;

ALTER TYPE webhook_lookup_by ADD VALUE IF NOT EXISTS 'phone';
ALTER TYPE webhook_lookup_by ADD VALUE IF NOT EXISTS 'email';
