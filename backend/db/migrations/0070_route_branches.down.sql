ALTER TABLE route_integrations DROP CONSTRAINT IF EXISTS route_integrations_route_conn_branch_key;

ALTER TABLE route_integrations
  ADD CONSTRAINT route_integrations_route_id_connection_id_key UNIQUE (route_id, connection_id);

ALTER TABLE route_integrations DROP COLUMN IF EXISTS branch_position;

ALTER TABLE routes
  ADD COLUMN condition_logic TEXT NOT NULL DEFAULT 'and',
  ADD COLUMN conditions JSONB NOT NULL DEFAULT '[]',
  ADD COLUMN position INT NOT NULL DEFAULT 0;

CREATE INDEX idx_routes_origin_priority ON routes(origin, position);

UPDATE routes SET
  condition_logic = COALESCE(branches->0->>'condition_logic', 'and'),
  conditions = COALESCE(branches->0->'conditions', '[]'::jsonb),
  position = COALESCE((branches->0->>'position')::int, 0)
WHERE jsonb_array_length(branches) > 0;

ALTER TABLE routes DROP COLUMN IF EXISTS branches;
