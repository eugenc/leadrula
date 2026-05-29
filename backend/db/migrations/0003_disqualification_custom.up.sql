CREATE TABLE disqualification_reasons (
    id         BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    account_id BIGINT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    label      TEXT NOT NULL,
    position   INT NOT NULL DEFAULT 0,
    is_active  BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_disq_account ON disqualification_reasons(account_id);

CREATE TYPE custom_field_type AS ENUM
    ('text','number','date','datetime','dropdown','checkbox');

CREATE TABLE custom_fields (
    id          BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    public_id   UUID NOT NULL DEFAULT gen_random_uuid() UNIQUE,
    account_id  BIGINT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    name        TEXT NOT NULL,
    field_key   TEXT NOT NULL,
    type        custom_field_type NOT NULL,
    options     JSONB NOT NULL DEFAULT '[]',
    position    INT NOT NULL DEFAULT 0,
    is_active   BOOLEAN NOT NULL DEFAULT true,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (account_id, field_key)
);
CREATE INDEX idx_custom_fields_account ON custom_fields(account_id);
