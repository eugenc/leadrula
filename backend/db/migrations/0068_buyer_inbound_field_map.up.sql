CREATE TABLE buyer_inbound_field_map (
    id              BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    buyer_id        BIGINT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    source_slug     TEXT NOT NULL DEFAULT '',
    source_key      TEXT NOT NULL,
    target_type     map_target NOT NULL,
    builtin_field   TEXT,
    custom_field_id BIGINT REFERENCES custom_fields(id),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (buyer_id, source_slug, source_key),
    CHECK (
        (target_type = 'builtin' AND builtin_field IS NOT NULL AND custom_field_id IS NULL)
        OR (target_type = 'custom' AND custom_field_id IS NOT NULL AND builtin_field IS NULL)
        OR (target_type = 'ignore' AND builtin_field IS NULL AND custom_field_id IS NULL)
    )
);
CREATE INDEX idx_buyer_inbound_field_map_buyer ON buyer_inbound_field_map(buyer_id);
