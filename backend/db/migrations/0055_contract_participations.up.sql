CREATE TYPE participation_status AS ENUM (
  'pending', 'active', 'declined', 'counter_pending', 'superseded'
);

ALTER TABLE contracts
  ADD COLUMN IF NOT EXISTS allowed_delivery_modes TEXT[] NOT NULL DEFAULT '{leads,leads_pipeline}',
  ADD COLUMN IF NOT EXISTS distribution_strategy TEXT NOT NULL DEFAULT 'round_robin',
  ADD COLUMN IF NOT EXISTS parent_contract_id BIGINT REFERENCES contracts(id) ON DELETE SET NULL,
  ADD COLUMN IF NOT EXISTS distribution_cursor INT NOT NULL DEFAULT 0,
  ADD COLUMN IF NOT EXISTS invite_token TEXT UNIQUE;

ALTER TABLE contracts
  ADD CONSTRAINT contracts_distribution_strategy_check
    CHECK (distribution_strategy IN ('round_robin', 'highest_price', 'largest_spread'));

DROP INDEX IF EXISTS contracts_publisher_buyer_type_key;

CREATE TABLE contract_participations (
    id                         BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    contract_id                BIGINT NOT NULL REFERENCES contracts(id) ON DELETE CASCADE,
    buyer_id                   BIGINT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    status                     participation_status NOT NULL DEFAULT 'pending',
    delivery                   TEXT,
    buyer_pipeline_id          BIGINT REFERENCES pipelines(id),
    buyer_target_stage_id      BIGINT REFERENCES pipeline_stages(id),
    source_pipeline_id         BIGINT REFERENCES pipelines(id),
    source_stage_id            BIGINT REFERENCES pipeline_stages(id),
    return_stage_id            BIGINT REFERENCES pipeline_stages(id),
    integration_connection_id  BIGINT REFERENCES integration_connections(id) ON DELETE SET NULL,
    outbound_webhook_id        BIGINT REFERENCES webhooks(id) ON DELETE SET NULL,
    counter_proposal           JSONB,
    superseded_by_contract_id  BIGINT REFERENCES contracts(id) ON DELETE SET NULL,
    created_at                 TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at                 TIMESTAMPTZ NOT NULL DEFAULT now(),
    buyer_responded_at         TIMESTAMPTZ,
    publisher_responded_at     TIMESTAMPTZ,
    CHECK (delivery IS NULL OR delivery IN ('leads', 'leads_pipeline', 'webhook'))
);

CREATE UNIQUE INDEX contract_participations_active_buyer
  ON contract_participations (contract_id, buyer_id)
  WHERE status <> 'superseded';

CREATE INDEX idx_contract_participations_contract ON contract_participations(contract_id);
CREATE INDEX idx_contract_participations_buyer ON contract_participations(buyer_id);

ALTER TABLE contract_compensations
  ADD COLUMN IF NOT EXISTS participation_id BIGINT REFERENCES contract_participations(id) ON DELETE CASCADE;

ALTER TABLE contract_compensations
  DROP CONSTRAINT IF EXISTS contract_compensations_delivery_check;

ALTER TABLE contract_compensations
  ADD CONSTRAINT contract_compensations_delivery_check
    CHECK (delivery IN ('leads', 'leads_pipeline', 'webhook'));

ALTER TABLE contract_required_fields ADD COLUMN IF NOT EXISTS participation_id BIGINT REFERENCES contract_participations(id) ON DELETE CASCADE;
ALTER TABLE contract_field_map ADD COLUMN IF NOT EXISTS participation_id BIGINT REFERENCES contract_participations(id) ON DELETE CASCADE;
ALTER TABLE contract_filter_rules ADD COLUMN IF NOT EXISTS participation_id BIGINT REFERENCES contract_participations(id) ON DELETE CASCADE;
ALTER TABLE contract_quality_rules ADD COLUMN IF NOT EXISTS participation_id BIGINT REFERENCES contract_participations(id) ON DELETE CASCADE;

-- Backfill participations for existing sell contracts with a buyer
INSERT INTO contract_participations (
    contract_id, buyer_id, status, delivery,
    buyer_pipeline_id, buyer_target_stage_id,
    source_pipeline_id, source_stage_id, return_stage_id,
    created_at, updated_at, buyer_responded_at
)
SELECT
    c.id, c.buyer_id, 'active',
    COALESCE(cc.delivery, 'leads_pipeline'),
    c.buyer_pipeline_id,
    (SELECT ps.id FROM pipeline_stages ps
     WHERE ps.pipeline_id = c.buyer_pipeline_id
     ORDER BY ps.position, ps.id LIMIT 1),
    c.source_pipeline_id, c.source_stage_id, c.return_stage_id,
    c.created_at, now(), c.created_at
FROM contracts c
LEFT JOIN LATERAL (
    SELECT delivery FROM contract_compensations
    WHERE contract_id = c.id ORDER BY position, id LIMIT 1
) cc ON true
WHERE c.deleted_at IS NULL
  AND c.buyer_id IS NOT NULL
  AND c.contract_type = 'sell'
  AND NOT EXISTS (
    SELECT 1 FROM contract_participations p WHERE p.contract_id = c.id AND p.buyer_id = c.buyer_id
  );

UPDATE contract_compensations cc
SET participation_id = p.id
FROM contract_participations p
WHERE p.contract_id = cc.contract_id
  AND p.buyer_id = (SELECT buyer_id FROM contracts WHERE id = cc.contract_id)
  AND cc.participation_id IS NULL;

UPDATE contract_required_fields crf
SET participation_id = p.id
FROM contract_participations p
WHERE p.contract_id = crf.contract_id AND crf.participation_id IS NULL;

UPDATE contract_field_map cfm
SET participation_id = p.id
FROM contract_participations p
WHERE p.contract_id = cfm.contract_id AND cfm.participation_id IS NULL;

UPDATE contract_filter_rules cfr
SET participation_id = p.id
FROM contract_participations p
WHERE p.contract_id = cfr.contract_id AND cfr.participation_id IS NULL;

UPDATE contract_quality_rules cqr
SET participation_id = p.id
FROM contract_participations p
WHERE p.contract_id = cqr.contract_id AND cqr.participation_id IS NULL;

UPDATE contracts
SET allowed_delivery_modes = ARRAY['leads', 'leads_pipeline']
WHERE allowed_delivery_modes IS NULL OR cardinality(allowed_delivery_modes) = 0;
