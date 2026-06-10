DROP TABLE IF EXISTS buyer_publisher_stripe;

ALTER TABLE accounts
  DROP COLUMN IF EXISTS stripe_connect_type,
  DROP COLUMN IF EXISTS stripe_secret_key_encrypted,
  DROP COLUMN IF EXISTS stripe_publishable_key,
  DROP COLUMN IF EXISTS stripe_keys_status;
