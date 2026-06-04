ALTER TABLE accounts
  DROP COLUMN IF EXISTS stripe_account_id,
  DROP COLUMN IF EXISTS stripe_account_status,
  DROP COLUMN IF EXISTS stripe_customer_id;

DROP INDEX IF EXISTS idx_txn_stripe_pi;

ALTER TABLE transactions
  DROP COLUMN IF EXISTS stripe_payment_intent_id,
  DROP COLUMN IF EXISTS stripe_charge_id;
