DROP INDEX IF EXISTS contracts_publisher_buyer_type_key;
ALTER TABLE contracts
  ADD CONSTRAINT contracts_publisher_buyer_type_key
    UNIQUE (publisher_id, buyer_id, contract_type);

ALTER TABLE contracts ALTER COLUMN return_stage_id SET NOT NULL;
ALTER TABLE contracts ALTER COLUMN buyer_pipeline_id SET NOT NULL;
ALTER TABLE contracts ALTER COLUMN source_stage_id SET NOT NULL;
ALTER TABLE contracts ALTER COLUMN source_pipeline_id SET NOT NULL;
ALTER TABLE contracts ALTER COLUMN buyer_id SET NOT NULL;

-- Postgres cannot remove enum values; draft rows would need migration before rollback.
