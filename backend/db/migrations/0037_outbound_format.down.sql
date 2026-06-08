ALTER TABLE webhooks
    DROP COLUMN IF EXISTS outbound_field_map,
    DROP COLUMN IF EXISTS outbound_method,
    DROP COLUMN IF EXISTS outbound_format;
