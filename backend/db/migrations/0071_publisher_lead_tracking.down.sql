DROP TABLE IF EXISTS contract_stage_maps;
ALTER TABLE leads
  DROP COLUMN IF EXISTS publisher_pipeline_id,
  DROP COLUMN IF EXISTS publisher_stage_id;
