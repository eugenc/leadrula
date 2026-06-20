DROP TABLE IF EXISTS dispute_message_attachments;
DROP TABLE IF EXISTS dispute_messages;
DROP INDEX IF EXISTS idx_disputes_deadline;
ALTER TABLE disputes
  DROP COLUMN IF EXISTS initiated_by,
  DROP COLUMN IF EXISTS lead_id,
  DROP COLUMN IF EXISTS contract_id,
  DROP COLUMN IF EXISTS amount,
  DROP COLUMN IF EXISTS deadline_days,
  DROP COLUMN IF EXISTS response_deadline_at,
  DROP COLUMN IF EXISTS awaiting_party,
  DROP COLUMN IF EXISTS outcome,
  DROP COLUMN IF EXISTS winner_party,
  DROP COLUMN IF EXISTS placement_party,
  DROP COLUMN IF EXISTS placement_pipeline_id,
  DROP COLUMN IF EXISTS placement_stage_id,
  DROP COLUMN IF EXISTS placement_completed_at;
-- enum values added to lead_status / notification_type are left in place
-- (Postgres cannot drop individual enum values).
