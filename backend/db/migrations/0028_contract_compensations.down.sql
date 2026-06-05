ALTER TABLE routes DROP COLUMN IF EXISTS compensation_id;
DROP TABLE IF EXISTS contract_compensation_accruals;
DROP TABLE IF EXISTS contract_compensations;

ALTER TABLE contracts DROP CONSTRAINT IF EXISTS contracts_publisher_buyer_type_key;
ALTER TABLE contracts
  ADD CONSTRAINT contracts_publisher_id_buyer_id_key UNIQUE (publisher_id, buyer_id);

ALTER TABLE contracts
  DROP CONSTRAINT IF EXISTS contracts_contract_type_check,
  DROP COLUMN IF EXISTS mirror_contract_id,
  DROP COLUMN IF EXISTS contract_type;
