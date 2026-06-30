-- Return routes can be pending until the publisher maps a destination stage.
ALTER TABLE contract_return_rules ALTER COLUMN return_stage_id DROP NOT NULL;
