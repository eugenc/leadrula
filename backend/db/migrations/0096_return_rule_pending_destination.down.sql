-- Pending routes must be mapped before reverting.
UPDATE contract_return_rules rr
SET return_stage_id = c.return_stage_id
FROM contracts c
WHERE rr.contract_id = c.id
  AND rr.return_stage_id IS NULL
  AND c.return_stage_id IS NOT NULL;

DELETE FROM contract_return_rules WHERE return_stage_id IS NULL;

ALTER TABLE contract_return_rules ALTER COLUMN return_stage_id SET NOT NULL;
