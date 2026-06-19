ALTER TABLE lead_stage_history
  DROP COLUMN IF EXISTS actor_label,
  DROP COLUMN IF EXISTS actor_type;
