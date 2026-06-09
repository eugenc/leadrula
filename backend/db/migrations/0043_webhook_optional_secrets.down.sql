ALTER TABLE webhooks
    DROP COLUMN IF EXISTS outbound_sign_enabled,
    DROP COLUMN IF EXISTS inbound_secret_required,
    ALTER COLUMN secret_hash SET NOT NULL,
    ALTER COLUMN secret_prefix SET NOT NULL;
