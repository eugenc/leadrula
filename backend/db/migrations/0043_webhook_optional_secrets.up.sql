ALTER TABLE webhooks
    ALTER COLUMN secret_hash DROP NOT NULL,
    ALTER COLUMN secret_prefix DROP NOT NULL,
    ADD COLUMN inbound_secret_required BOOLEAN NOT NULL DEFAULT true,
    ADD COLUMN outbound_sign_enabled BOOLEAN NOT NULL DEFAULT true;
