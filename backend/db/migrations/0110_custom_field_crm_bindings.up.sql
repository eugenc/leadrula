CREATE TABLE custom_field_crm_bindings (
    id                 BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    account_id         BIGINT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    custom_field_id    BIGINT NOT NULL REFERENCES custom_fields(id) ON DELETE CASCADE,
    connection_id      BIGINT NOT NULL REFERENCES integration_connections(id) ON DELETE CASCADE,
    crm_field_id       TEXT NOT NULL,
    crm_field_key      TEXT NOT NULL,
    crm_object         TEXT NOT NULL DEFAULT 'contact',
    inbound_source_key TEXT NOT NULL,
    created_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (connection_id, crm_field_id)
);
CREATE INDEX idx_cf_crm_bindings_account ON custom_field_crm_bindings(account_id);
CREATE INDEX idx_cf_crm_bindings_connection ON custom_field_crm_bindings(connection_id);
CREATE INDEX idx_cf_crm_bindings_field ON custom_field_crm_bindings(custom_field_id);
