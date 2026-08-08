ALTER TABLE routing_sources
  DROP COLUMN IF EXISTS phone_match_mode,
  DROP COLUMN IF EXISTS publisher_stage_id,
  DROP COLUMN IF EXISTS publisher_pipeline_id,
  DROP COLUMN IF EXISTS delivery_mode,
  DROP COLUMN IF EXISTS contract_id;

-- PostgreSQL does not support removing enum values; appointment sources must be deleted first.
