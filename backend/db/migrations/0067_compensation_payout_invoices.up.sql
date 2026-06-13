ALTER TABLE invoices
  DROP CONSTRAINT IF EXISTS invoices_kind_check;

ALTER TABLE invoices
  ADD CONSTRAINT invoices_kind_check
    CHECK (kind IN ('starting_balance', 'prepay_request', 'compensation_payout'));

ALTER TYPE txn_type ADD VALUE IF NOT EXISTS 'compensation_payout';

ALTER TABLE invoices
  ADD COLUMN compensation_payout_clear_id BIGINT REFERENCES compensation_payout_clears(id) ON DELETE SET NULL;

CREATE UNIQUE INDEX idx_invoices_compensation_payout_clear
  ON invoices(compensation_payout_clear_id)
  WHERE compensation_payout_clear_id IS NOT NULL;

ALTER TABLE compensation_payout_clears
  ADD COLUMN invoice_id BIGINT REFERENCES invoices(id) ON DELETE SET NULL;

CREATE UNIQUE INDEX idx_comp_payout_clears_invoice
  ON compensation_payout_clears(invoice_id)
  WHERE invoice_id IS NOT NULL;
