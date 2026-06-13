ALTER TABLE contract_return_rules
  ADD COLUMN IF NOT EXISTS participation_id BIGINT REFERENCES contract_participations(id) ON DELETE CASCADE;

ALTER TABLE contract_return_rules DROP CONSTRAINT IF EXISTS contract_return_rules_contract_id_buyer_stage_id_key;

CREATE UNIQUE INDEX IF NOT EXISTS idx_return_rules_contract_stage
  ON contract_return_rules (contract_id, buyer_stage_id)
  WHERE participation_id IS NULL;

CREATE UNIQUE INDEX IF NOT EXISTS idx_return_rules_participation_stage
  ON contract_return_rules (participation_id, buyer_stage_id)
  WHERE participation_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_return_rules_participation ON contract_return_rules (participation_id);
