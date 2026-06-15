DROP INDEX IF EXISTS idx_routes_origin_priority;

ALTER TABLE routes
  DROP COLUMN IF EXISTS position,
  DROP COLUMN IF EXISTS conditions,
  DROP COLUMN IF EXISTS condition_logic;
