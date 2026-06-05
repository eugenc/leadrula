CREATE TABLE contract_required_fields (
    id               BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    contract_id      BIGINT NOT NULL REFERENCES contracts(id) ON DELETE CASCADE,
    field_type       TEXT NOT NULL,
    builtin_field    TEXT,
    custom_field_id  BIGINT REFERENCES custom_fields(id) ON DELETE CASCADE,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK (field_type IN ('builtin', 'custom')),
    CHECK (
      (field_type = 'builtin' AND builtin_field IS NOT NULL AND custom_field_id IS NULL)
      OR (field_type = 'custom' AND custom_field_id IS NOT NULL)
    )
);
CREATE INDEX idx_contract_required_fields_contract ON contract_required_fields(contract_id);

CREATE TABLE contract_field_map (
    id                  BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    contract_id         BIGINT NOT NULL REFERENCES contracts(id) ON DELETE CASCADE,
    src_type            TEXT NOT NULL,
    src_builtin         TEXT,
    src_custom_field_id BIGINT REFERENCES custom_fields(id) ON DELETE CASCADE,
    dst_type            TEXT NOT NULL,
    dst_builtin         TEXT,
    dst_custom_field_id BIGINT REFERENCES custom_fields(id) ON DELETE CASCADE,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK (src_type IN ('builtin', 'custom')),
    CHECK (dst_type IN ('builtin', 'custom'))
);
CREATE INDEX idx_contract_field_map_contract ON contract_field_map(contract_id);

CREATE TABLE contract_filter_rules (
    id           BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    contract_id  BIGINT NOT NULL REFERENCES contracts(id) ON DELETE CASCADE,
    field_type   TEXT NOT NULL,
    builtin_field TEXT,
    custom_field_id BIGINT REFERENCES custom_fields(id) ON DELETE CASCADE,
    operator     TEXT NOT NULL,
    value        TEXT NOT NULL,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK (field_type IN ('builtin', 'custom')),
    CHECK (operator IN ('eq', 'neq', 'contains', 'not_empty'))
);
CREATE INDEX idx_contract_filter_rules_contract ON contract_filter_rules(contract_id);

CREATE TABLE contract_quality_rules (
    id              BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    contract_id     BIGINT NOT NULL REFERENCES contracts(id) ON DELETE CASCADE,
    buyer_stage_id  BIGINT NOT NULL REFERENCES pipeline_stages(id) ON DELETE CASCADE,
    on_fail         TEXT NOT NULL DEFAULT 'return',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK (on_fail IN ('return', 'disqualify'))
);
CREATE INDEX idx_contract_quality_rules_contract ON contract_quality_rules(contract_id);
