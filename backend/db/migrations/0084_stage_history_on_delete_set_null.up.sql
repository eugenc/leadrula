ALTER TABLE lead_stage_history ALTER COLUMN to_stage_id DROP NOT NULL;

ALTER TABLE lead_stage_history DROP CONSTRAINT IF EXISTS lead_stage_history_from_stage_id_fkey;
ALTER TABLE lead_stage_history DROP CONSTRAINT IF EXISTS lead_stage_history_to_stage_id_fkey;

ALTER TABLE lead_stage_history
  ADD CONSTRAINT lead_stage_history_from_stage_id_fkey
    FOREIGN KEY (from_stage_id) REFERENCES pipeline_stages(id) ON DELETE SET NULL;

ALTER TABLE lead_stage_history
  ADD CONSTRAINT lead_stage_history_to_stage_id_fkey
    FOREIGN KEY (to_stage_id) REFERENCES pipeline_stages(id) ON DELETE SET NULL;
