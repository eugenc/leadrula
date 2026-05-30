ALTER TABLE leads RENAME COLUMN campaign_name TO source;
ALTER INDEX idx_leads_campaign RENAME TO idx_leads_source;

ALTER TABLE lead_intake_queue RENAME COLUMN campaign_name TO source;

-- Saved views: column id and filter field names
UPDATE lead_saved_views
SET columns = replace(columns::text, '"campaign"', '"source"')::jsonb
WHERE columns IS NOT NULL AND columns::text LIKE '%"campaign"%';

UPDATE lead_saved_views
SET filters = replace(filters::text, '"field":"campaign_name"', '"field":"source"')::jsonb
WHERE filters::text LIKE '%"field":"campaign_name"%';

-- Stage rules: condition/action field names
UPDATE stage_rules
SET conditions = replace(conditions::text, '"field":"campaign_name"', '"field":"source"')::jsonb
WHERE conditions::text LIKE '%"field":"campaign_name"%';

UPDATE stage_rules
SET actions = replace(actions::text, '"field":"campaign_name"', '"field":"source"')::jsonb
WHERE actions::text LIKE '%"field":"campaign_name"%';

UPDATE routing_source_field_map
SET builtin_field = 'source'
WHERE builtin_field = 'campaign_name';
