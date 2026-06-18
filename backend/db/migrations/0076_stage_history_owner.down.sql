DROP INDEX IF EXISTS idx_stage_history_owner;
ALTER TABLE lead_stage_history DROP COLUMN IF EXISTS owner_account_id;
