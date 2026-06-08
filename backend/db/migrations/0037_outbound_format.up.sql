ALTER TABLE webhooks
    ADD COLUMN outbound_format    TEXT NOT NULL DEFAULT 'json'
        CHECK (outbound_format IN ('json', 'url')),
    ADD COLUMN outbound_method    TEXT NOT NULL DEFAULT 'POST'
        CHECK (outbound_method IN ('GET', 'POST')),
    ADD COLUMN outbound_field_map JSONB NOT NULL DEFAULT '[]';
