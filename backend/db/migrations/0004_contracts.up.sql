CREATE TYPE contract_status AS ENUM ('active','paused','terminated');

CREATE TABLE contracts (
    id                 BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    public_id          UUID NOT NULL DEFAULT gen_random_uuid() UNIQUE,
    publisher_id       BIGINT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    buyer_id           BIGINT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    name               TEXT NOT NULL,
    source_pipeline_id BIGINT NOT NULL REFERENCES pipelines(id),
    source_stage_id    BIGINT NOT NULL REFERENCES pipeline_stages(id),
    buyer_pipeline_id  BIGINT NOT NULL REFERENCES pipelines(id),
    return_stage_id    BIGINT NOT NULL REFERENCES pipeline_stages(id),
    rate_per_lead      NUMERIC(14,2) NOT NULL DEFAULT 0,
    status             contract_status NOT NULL DEFAULT 'active',
    created_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at         TIMESTAMPTZ,
    UNIQUE (publisher_id, buyer_id)
);
CREATE INDEX idx_contracts_buyer ON contracts(buyer_id);
CREATE INDEX idx_contracts_target_pipeline ON contracts(buyer_pipeline_id);

CREATE TABLE contract_return_rules (
    id             BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    contract_id    BIGINT NOT NULL REFERENCES contracts(id) ON DELETE CASCADE,
    buyer_stage_id BIGINT NOT NULL REFERENCES pipeline_stages(id),
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (contract_id, buyer_stage_id)
);
CREATE INDEX idx_return_rules_contract ON contract_return_rules(contract_id);
