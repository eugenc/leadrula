ALTER TABLE contract_quality_rules DROP COLUMN IF EXISTS participation_id;
ALTER TABLE contract_filter_rules DROP COLUMN IF EXISTS participation_id;
ALTER TABLE contract_field_map DROP COLUMN IF EXISTS participation_id;
ALTER TABLE contract_required_fields DROP COLUMN IF EXISTS participation_id;

ALTER TABLE contract_compensations DROP COLUMN IF EXISTS participation_id;
ALTER TABLE contract_compensations DROP CONSTRAINT IF EXISTS contract_compensations_delivery_check;
ALTER TABLE contract_compensations
  ADD CONSTRAINT contract_compensations_delivery_check
    CHECK (delivery IN ('leads', 'leads_pipeline'));

DROP TABLE IF EXISTS contract_participations;
DROP TYPE IF EXISTS participation_status;

ALTER TABLE contracts
  DROP CONSTRAINT IF EXISTS contracts_distribution_strategy_check,
  DROP COLUMN IF EXISTS allowed_delivery_modes,
  DROP COLUMN IF EXISTS distribution_strategy,
  DROP COLUMN IF EXISTS parent_contract_id,
  DROP COLUMN IF EXISTS distribution_cursor,
  DROP COLUMN IF EXISTS invite_token;

CREATE UNIQUE INDEX IF NOT EXISTS contracts_publisher_buyer_type_key
  ON contracts (publisher_id, buyer_id, contract_type)
  WHERE buyer_id IS NOT NULL AND deleted_at IS NULL;
