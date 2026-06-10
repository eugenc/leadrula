ALTER TYPE contract_status ADD VALUE IF NOT EXISTS 'draft';

ALTER TABLE contracts ALTER COLUMN buyer_id DROP NOT NULL;
ALTER TABLE contracts ALTER COLUMN source_pipeline_id DROP NOT NULL;
ALTER TABLE contracts ALTER COLUMN source_stage_id DROP NOT NULL;
ALTER TABLE contracts ALTER COLUMN buyer_pipeline_id DROP NOT NULL;
ALTER TABLE contracts ALTER COLUMN return_stage_id DROP NOT NULL;

ALTER TABLE contracts DROP CONSTRAINT IF EXISTS contracts_publisher_buyer_type_key;
CREATE UNIQUE INDEX contracts_publisher_buyer_type_key
  ON contracts (publisher_id, buyer_id, contract_type)
  WHERE buyer_id IS NOT NULL AND deleted_at IS NULL;
