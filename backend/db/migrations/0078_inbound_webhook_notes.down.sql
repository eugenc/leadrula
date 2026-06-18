ALTER TABLE lead_notes DROP COLUMN IF EXISTS author_name;

ALTER TABLE webhook_events DROP COLUMN IF EXISTS note_source_key;
