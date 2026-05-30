UPDATE routing_source_field_map
SET builtin_field = 'campaign_name'
WHERE builtin_field = 'source';

UPDATE stage_rules
SET actions = replace(actions::text, '"field":"source"', '"field":"campaign_name"')::jsonb
WHERE actions::text LIKE '%"field":"source"%';

UPDATE stage_rules
SET conditions = replace(conditions::text, '"field":"source"', '"field":"campaign_name"')::jsonb
WHERE conditions::text LIKE '%"field":"source"%';

UPDATE lead_saved_views
SET filters = replace(filters::text, '"field":"source"', '"field":"campaign_name"')::jsonb
WHERE filters::text LIKE '%"field":"source"%';

UPDATE lead_saved_views
SET columns = replace(columns::text, '"source"', '"campaign"')::jsonb
WHERE columns IS NOT NULL AND columns::text LIKE '%"source"%';

ALTER TABLE lead_intake_queue RENAME COLUMN source TO campaign_name;

ALTER INDEX idx_leads_source RENAME TO idx_leads_campaign;
ALTER TABLE leads RENAME COLUMN source TO campaign_name;
