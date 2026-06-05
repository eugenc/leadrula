ALTER TABLE contracts
  ADD COLUMN contract_type TEXT NOT NULL DEFAULT 'sell',
  ADD COLUMN mirror_contract_id BIGINT REFERENCES contracts(id) ON DELETE SET NULL;

ALTER TABLE contracts
  ADD CONSTRAINT contracts_contract_type_check
    CHECK (contract_type IN ('buy', 'sell'));

ALTER TABLE contracts DROP CONSTRAINT IF EXISTS contracts_publisher_id_buyer_id_key;
ALTER TABLE contracts
  ADD CONSTRAINT contracts_publisher_buyer_type_key
    UNIQUE (publisher_id, buyer_id, contract_type);

CREATE TABLE contract_compensations (
    id                         BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    contract_id                BIGINT NOT NULL REFERENCES contracts(id) ON DELETE CASCADE,
    kind                       TEXT NOT NULL,
    flat_amount                NUMERIC(14,2),
    bid_min                    NUMERIC(14,2),
    bid_max                    NUMERIC(14,2),
    rev_percent                NUMERIC(5,2),
    profit_percent             NUMERIC(5,2),
    billing_period             TEXT NOT NULL DEFAULT 'one_time',
    cap_total                  INT,
    cap_max_daily              INT,
    trigger                    TEXT NOT NULL DEFAULT 'per_lead',
    trigger_stage_id           BIGINT REFERENCES pipeline_stages(id),
    source_pipeline_id         BIGINT REFERENCES pipelines(id),
    source_stage_id            BIGINT REFERENCES pipeline_stages(id),
    counterparty_pipeline_id   BIGINT REFERENCES pipelines(id),
    counterparty_stage_id      BIGINT REFERENCES pipeline_stages(id),
    return_stage_id            BIGINT REFERENCES pipeline_stages(id),
    delivery                   TEXT NOT NULL DEFAULT 'leads_pipeline',
    position                   INT NOT NULL DEFAULT 0,
    created_at                 TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT contract_compensations_kind_check
      CHECK (kind IN ('flat_rate', 'bid', 'rev_share', 'profit_share')),
    CONSTRAINT contract_compensations_billing_period_check
      CHECK (billing_period IN ('one_time', 'weekly', 'monthly')),
    CONSTRAINT contract_compensations_trigger_check
      CHECK (trigger IN ('per_lead', 'buyer_stage', 'manual')),
    CONSTRAINT contract_compensations_delivery_check
      CHECK (delivery IN ('leads', 'leads_pipeline')),
    CONSTRAINT contract_compensations_cap_nonneg_check
      CHECK (cap_total IS NULL OR cap_total > 0),
    CONSTRAINT contract_compensations_cap_daily_nonneg_check
      CHECK (cap_max_daily IS NULL OR cap_max_daily > 0)
);

CREATE INDEX idx_contract_compensations_contract ON contract_compensations(contract_id);

CREATE TABLE contract_compensation_accruals (
    id               BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    compensation_id  BIGINT NOT NULL REFERENCES contract_compensations(id) ON DELETE CASCADE,
    lead_id          BIGINT NOT NULL REFERENCES leads(id) ON DELETE CASCADE,
    amount           NUMERIC(14,2) NOT NULL,
    trigger_source   TEXT NOT NULL,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT contract_comp_accruals_trigger_check
      CHECK (trigger_source IN ('stage', 'manual')),
    UNIQUE (compensation_id, lead_id, trigger_source)
);

CREATE INDEX idx_contract_comp_accruals_comp ON contract_compensation_accruals(compensation_id);

ALTER TABLE routes
  ADD COLUMN compensation_id BIGINT REFERENCES contract_compensations(id) ON DELETE SET NULL;

-- Backfill contract_type and default compensation from legacy columns
UPDATE contracts SET contract_type = 'sell' WHERE contract_type IS NULL;

INSERT INTO contract_compensations (
    contract_id, kind, flat_amount, billing_period, cap_total, cap_max_daily,
    trigger, source_pipeline_id, source_stage_id,
    counterparty_pipeline_id, counterparty_stage_id, return_stage_id, delivery, position
)
SELECT
    id, 'flat_rate', rate_per_lead, billing_period, cap_total, cap_max_daily,
    'per_lead', source_pipeline_id, source_stage_id,
    buyer_pipeline_id, NULL, return_stage_id, 'leads_pipeline', 0
FROM contracts
WHERE deleted_at IS NULL;
