DROP INDEX IF EXISTS idx_return_rules_participation;
DROP INDEX IF EXISTS idx_return_rules_participation_stage;
DROP INDEX IF EXISTS idx_return_rules_contract_stage;

ALTER TABLE contract_return_rules DROP COLUMN IF EXISTS participation_id;

ALTER TABLE contract_return_rules
  ADD CONSTRAINT contract_return_rules_contract_id_buyer_stage_id_key UNIQUE (contract_id, buyer_stage_id);
