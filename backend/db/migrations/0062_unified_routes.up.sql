-- Unified routing: extend origins/destinations, buyer-owned routes.

ALTER TYPE route_origin ADD VALUE IF NOT EXISTS 'webhook';
ALTER TYPE route_origin ADD VALUE IF NOT EXISTS 'integration';

ALTER TYPE route_destination ADD VALUE IF NOT EXISTS 'contract';
ALTER TYPE route_destination ADD VALUE IF NOT EXISTS 'pipeline';
ALTER TYPE route_destination ADD VALUE IF NOT EXISTS 'webhook';
ALTER TYPE route_destination ADD VALUE IF NOT EXISTS 'integration';

UPDATE routes SET destination = 'contract' WHERE destination = 'buyer';
UPDATE routes SET destination = 'pipeline' WHERE destination = 'publisher';

ALTER TABLE routes ADD COLUMN IF NOT EXISTS buyer_id BIGINT REFERENCES accounts(id) ON DELETE CASCADE;
ALTER TABLE routes ADD COLUMN IF NOT EXISTS origin_webhook_id BIGINT REFERENCES webhooks(id) ON DELETE CASCADE;
ALTER TABLE routes ADD COLUMN IF NOT EXISTS origin_connection_id BIGINT REFERENCES integration_connections(id) ON DELETE CASCADE;
ALTER TABLE routes ADD COLUMN IF NOT EXISTS dest_webhook_id BIGINT REFERENCES webhooks(id) ON DELETE CASCADE;

ALTER TABLE routes ALTER COLUMN publisher_id DROP NOT NULL;

ALTER TABLE routes DROP CONSTRAINT IF EXISTS routes_check;
ALTER TABLE routes DROP CONSTRAINT IF EXISTS routes_check1;
ALTER TABLE routes DROP CONSTRAINT IF EXISTS routes_check2;
ALTER TABLE routes DROP CONSTRAINT IF EXISTS routes_check3;

ALTER TABLE routes ADD CONSTRAINT routes_one_owner CHECK (
    (publisher_id IS NOT NULL AND buyer_id IS NULL)
    OR (buyer_id IS NOT NULL AND publisher_id IS NULL)
);

ALTER TABLE routes ADD CONSTRAINT routes_origin_shape CHECK (
    (origin = 'source' AND source_id IS NOT NULL AND origin_pipeline_id IS NULL AND origin_stage_id IS NULL
        AND origin_webhook_id IS NULL AND origin_connection_id IS NULL)
    OR (origin = 'pipeline' AND source_id IS NULL AND origin_pipeline_id IS NOT NULL AND origin_stage_id IS NOT NULL
        AND origin_webhook_id IS NULL AND origin_connection_id IS NULL)
    OR (origin = 'webhook' AND origin_webhook_id IS NOT NULL AND source_id IS NULL
        AND origin_pipeline_id IS NULL AND origin_stage_id IS NULL AND origin_connection_id IS NULL)
    OR (origin = 'integration' AND origin_connection_id IS NOT NULL AND source_id IS NULL
        AND origin_pipeline_id IS NULL AND origin_stage_id IS NULL AND origin_webhook_id IS NULL)
);

ALTER TABLE routes ADD CONSTRAINT routes_destination_shape CHECK (
    (destination = 'contract' AND contract_id IS NOT NULL AND publisher_id IS NOT NULL)
    OR (destination = 'pipeline' AND contract_id IS NULL
        AND target_pipeline_id IS NOT NULL AND target_stage_id IS NOT NULL)
    OR (destination = 'webhook' AND dest_webhook_id IS NOT NULL AND contract_id IS NULL)
    OR (destination = 'integration' AND contract_id IS NULL
        AND dest_webhook_id IS NULL AND target_pipeline_id IS NULL AND target_stage_id IS NULL)
);

CREATE INDEX IF NOT EXISTS idx_routes_buyer_stage
    ON routes(buyer_id, origin_stage_id) WHERE buyer_id IS NOT NULL AND origin = 'pipeline';

CREATE INDEX IF NOT EXISTS idx_routes_origin_webhook
    ON routes(origin_webhook_id) WHERE origin = 'webhook';

CREATE INDEX IF NOT EXISTS idx_routes_origin_connection
    ON routes(origin_connection_id) WHERE origin = 'integration';
