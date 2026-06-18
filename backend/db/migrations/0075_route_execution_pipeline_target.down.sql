ALTER TABLE route_executions
  DROP COLUMN IF EXISTS target_pipeline_id,
  DROP COLUMN IF EXISTS target_stage_id,
  DROP COLUMN IF EXISTS target_pipeline_name,
  DROP COLUMN IF EXISTS target_stage_name,
  DROP COLUMN IF EXISTS delivery;
