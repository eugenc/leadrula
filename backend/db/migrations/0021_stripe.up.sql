-- Publisher: Stripe Connect Express account
ALTER TABLE accounts
  ADD COLUMN stripe_account_id     TEXT,
  ADD COLUMN stripe_account_status TEXT NOT NULL DEFAULT 'none';

-- Buyer: Stripe customer (saved payment methods)
ALTER TABLE accounts
  ADD COLUMN stripe_customer_id TEXT;

-- Track Stripe IDs on transactions
ALTER TABLE transactions
  ADD COLUMN stripe_payment_intent_id TEXT,
  ADD COLUMN stripe_charge_id         TEXT;

CREATE UNIQUE INDEX idx_txn_stripe_pi ON transactions(stripe_payment_intent_id)
  WHERE stripe_payment_intent_id IS NOT NULL;

ALTER TYPE txn_type ADD VALUE IF NOT EXISTS 'topup';
