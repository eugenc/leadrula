DELETE FROM routes WHERE buyer_id IS NOT NULL;

UPDATE routes SET destination = 'buyer' WHERE destination = 'contract';
UPDATE routes SET destination = 'publisher' WHERE destination = 'pipeline';

ALTER TABLE routes DROP CONSTRAINT IF EXISTS routes_destination_shape;
ALTER TABLE routes DROP CONSTRAINT IF EXISTS routes_origin_shape;
ALTER TABLE routes DROP CONSTRAINT IF EXISTS routes_one_owner;

ALTER TABLE routes DROP COLUMN IF EXISTS dest_webhook_id;
ALTER TABLE routes DROP COLUMN IF EXISTS origin_connection_id;
ALTER TABLE routes DROP COLUMN IF EXISTS origin_webhook_id;
ALTER TABLE routes DROP COLUMN IF EXISTS buyer_id;

DROP INDEX IF EXISTS idx_routes_buyer_stage;
DROP INDEX IF EXISTS idx_routes_origin_webhook;
DROP INDEX IF EXISTS idx_routes_origin_connection;

-- publisher_id must be set for remaining rows
UPDATE routes SET publisher_id = (SELECT MIN(id) FROM accounts WHERE type = 'publisher') WHERE publisher_id IS NULL;
ALTER TABLE routes ALTER COLUMN publisher_id SET NOT NULL;

ALTER TABLE routes ADD CONSTRAINT routes_check CHECK (
    (origin = 'source' AND source_id IS NOT NULL AND origin_pipeline_id IS NULL AND origin_stage_id IS NULL)
    OR (origin = 'pipeline' AND source_id IS NULL AND origin_pipeline_id IS NOT NULL AND origin_stage_id IS NOT NULL)
);
ALTER TABLE routes ADD CONSTRAINT routes_check1 CHECK (
    (destination = 'buyer' AND contract_id IS NOT NULL)
    OR (destination = 'publisher' AND contract_id IS NULL)
);
ALTER TABLE routes ADD CONSTRAINT routes_check2 CHECK (NOT (origin = 'pipeline' AND destination = 'publisher'));
ALTER TABLE routes ADD CONSTRAINT routes_check3 CHECK (
    (destination = 'publisher' AND delivery = 'leads')
    OR (destination = 'publisher' AND delivery = 'leads_pipeline' AND target_pipeline_id IS NOT NULL AND target_stage_id IS NOT NULL)
    OR (destination = 'buyer')
);
