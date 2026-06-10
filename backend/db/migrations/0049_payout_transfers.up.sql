ALTER TABLE compensation_payout_clears
  ADD COLUMN stripe_transfer_id TEXT,
  ADD COLUMN stripe_transfer_status TEXT NOT NULL DEFAULT 'pending';

CREATE UNIQUE INDEX idx_comp_payout_clears_stripe_transfer
  ON compensation_payout_clears(stripe_transfer_id)
  WHERE stripe_transfer_id IS NOT NULL;
