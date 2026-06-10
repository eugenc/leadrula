DROP INDEX IF EXISTS idx_comp_payout_clears_stripe_transfer;

ALTER TABLE compensation_payout_clears
  DROP COLUMN IF EXISTS stripe_transfer_id,
  DROP COLUMN IF EXISTS stripe_transfer_status;
