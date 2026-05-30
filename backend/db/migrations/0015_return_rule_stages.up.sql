ALTER TABLE contract_return_rules ADD COLUMN return_stage_id BIGINT REFERENCES pipeline_stages(id);

UPDATE contract_return_rules rr
SET return_stage_id = c.return_stage_id
FROM contracts c
WHERE c.id = rr.contract_id;

ALTER TABLE contract_return_rules ALTER COLUMN return_stage_id SET NOT NULL;
