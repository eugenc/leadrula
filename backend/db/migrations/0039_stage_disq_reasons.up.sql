ALTER TABLE disqualification_reasons ADD COLUMN stage_id BIGINT REFERENCES pipeline_stages(id) ON DELETE CASCADE;

-- Copy each account-scoped reason to every disqualification stage in that account.
DO $$
DECLARE
  r RECORD;
  s RECORD;
  assigned BOOLEAN;
BEGIN
  FOR r IN SELECT id, account_id, label, position, is_active, created_at
           FROM disqualification_reasons ORDER BY id
  LOOP
    assigned := FALSE;
    FOR s IN
      SELECT ps.id
      FROM pipeline_stages ps
      JOIN pipelines p ON p.id = ps.pipeline_id
      WHERE p.account_id = r.account_id AND ps.stage_type = 'disqualification'
      ORDER BY p.position, ps.position, ps.id
    LOOP
      IF NOT assigned THEN
        UPDATE disqualification_reasons SET stage_id = s.id WHERE id = r.id;
        assigned := TRUE;
      ELSE
        INSERT INTO disqualification_reasons (stage_id, label, position, is_active, created_at)
        VALUES (s.id, r.label, r.position, r.is_active, r.created_at);
      END IF;
    END LOOP;
    IF NOT assigned THEN
      UPDATE leads SET disqualification_reason_id = NULL WHERE disqualification_reason_id = r.id;
      UPDATE lead_stage_history SET disqualification_reason_id = NULL WHERE disqualification_reason_id = r.id;
      DELETE FROM disqualification_reasons WHERE id = r.id;
    END IF;
  END LOOP;
END $$;

-- Remap lead FKs to the reason row for the lead's current stage (matching label).
UPDATE leads l
SET disqualification_reason_id = dr_new.id
FROM disqualification_reasons dr_old,
     disqualification_reasons dr_new
WHERE l.disqualification_reason_id = dr_old.id
  AND l.stage_id IS NOT NULL
  AND dr_new.label = dr_old.label
  AND dr_new.stage_id = l.stage_id
  AND dr_old.id <> dr_new.id;

-- Remap stage history FKs to the reason row for the history to_stage.
UPDATE lead_stage_history h
SET disqualification_reason_id = dr_new.id
FROM disqualification_reasons dr_old,
     disqualification_reasons dr_new
WHERE h.disqualification_reason_id = dr_old.id
  AND dr_new.label = dr_old.label
  AND dr_new.stage_id = h.to_stage_id
  AND dr_old.id <> dr_new.id;

ALTER TABLE disqualification_reasons ALTER COLUMN stage_id SET NOT NULL;
ALTER TABLE disqualification_reasons DROP COLUMN account_id;
DROP INDEX IF EXISTS idx_disq_account;
CREATE INDEX idx_disq_stage ON disqualification_reasons(stage_id);
