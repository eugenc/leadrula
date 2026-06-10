ALTER TABLE accounts
  ADD COLUMN stripe_connect_type TEXT,
  ADD COLUMN stripe_secret_key_encrypted BYTEA,
  ADD COLUMN stripe_publishable_key TEXT,
  ADD COLUMN stripe_keys_status TEXT NOT NULL DEFAULT 'none';

CREATE TABLE buyer_publisher_stripe (
  buyer_id BIGINT NOT NULL REFERENCES accounts(id),
  publisher_id BIGINT NOT NULL REFERENCES accounts(id),
  stripe_customer_id TEXT NOT NULL,
  PRIMARY KEY (buyer_id, publisher_id)
);
