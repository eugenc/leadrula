DROP INDEX IF EXISTS idx_leads_preassigned_buyer;
ALTER TABLE leads DROP COLUMN IF EXISTS preassigned_buyer_id;
