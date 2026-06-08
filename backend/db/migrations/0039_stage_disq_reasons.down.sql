ALTER TABLE disqualification_reasons ADD COLUMN account_id BIGINT REFERENCES accounts(id) ON DELETE CASCADE;

UPDATE disqualification_reasons dr
SET account_id = p.account_id
FROM pipeline_stages ps
JOIN pipelines p ON p.id = ps.pipeline_id
WHERE ps.id = dr.stage_id;

-- Keep one reason per (account, label); delete duplicates.
DELETE FROM disqualification_reasons dr
USING disqualification_reasons dr2
JOIN pipeline_stages ps2 ON ps2.id = dr2.stage_id
JOIN pipelines p2 ON p2.id = ps2.pipeline_id
WHERE dr.account_id = p2.account_id
  AND dr.label = dr2.label
  AND dr.id > dr2.id;

ALTER TABLE disqualification_reasons ALTER COLUMN account_id SET NOT NULL;
ALTER TABLE disqualification_reasons DROP COLUMN stage_id;
DROP INDEX IF EXISTS idx_disq_stage;
CREATE INDEX idx_disq_account ON disqualification_reasons(account_id);
