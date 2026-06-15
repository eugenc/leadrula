ALTER TABLE routes ADD COLUMN branches JSONB NOT NULL DEFAULT '[]';

UPDATE routes SET branches = jsonb_build_array(
  jsonb_strip_nulls(jsonb_build_object(
    'position', 0,
    'condition_logic', condition_logic,
    'conditions', conditions,
    'destination', destination,
    'delivery', delivery,
    'target_pipeline_id', target_pipeline_id,
    'target_stage_id', target_stage_id,
    'contract_id', contract_id,
    'compensation_id', compensation_id,
    'dest_webhook_id', dest_webhook_id
  ))
);

DROP INDEX IF EXISTS idx_routes_origin_priority;

ALTER TABLE routes
  DROP COLUMN IF EXISTS condition_logic,
  DROP COLUMN IF EXISTS conditions,
  DROP COLUMN IF EXISTS position;

ALTER TABLE route_integrations ADD COLUMN branch_position INT NOT NULL DEFAULT 0;

ALTER TABLE route_integrations DROP CONSTRAINT route_integrations_route_id_connection_id_key;

ALTER TABLE route_integrations
  ADD CONSTRAINT route_integrations_route_conn_branch_key
  UNIQUE (route_id, connection_id, branch_position);
