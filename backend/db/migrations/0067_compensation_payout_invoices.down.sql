ALTER TABLE compensation_payout_clears DROP COLUMN IF EXISTS invoice_id;

ALTER TABLE invoices DROP COLUMN IF EXISTS compensation_payout_clear_id;

ALTER TABLE invoices
  DROP CONSTRAINT IF EXISTS invoices_kind_check;

ALTER TABLE invoices
  ADD CONSTRAINT invoices_kind_check
    CHECK (kind IN ('starting_balance', 'prepay_request'));
