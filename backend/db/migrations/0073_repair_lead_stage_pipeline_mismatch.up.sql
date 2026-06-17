-- Repair buyer-owned leads whose stage_id points at a different pipeline than pipeline_id.
-- Caused by contract distribution falling back to publisher route stage IDs.

WITH mismatched AS (
    SELECT l.id AS lead_id,
           l.pipeline_id,
           l.stage_id AS old_stage_id,
           l.contract_id,
           l.owner_account_id,
           ps.name AS old_stage_name
    FROM leads l
    JOIN pipeline_stages ps ON ps.id = l.stage_id
    JOIN accounts oa ON oa.id = l.owner_account_id
    WHERE l.pipeline_id IS NOT NULL
      AND l.stage_id IS NOT NULL
      AND l.pipeline_id <> ps.pipeline_id
      AND l.deleted_at IS NULL
      AND oa.type = 'buyer'
),
repaired AS (
    SELECT m.lead_id,
           COALESCE(
               (SELECT p.buyer_target_stage_id
                FROM contract_participations p
                JOIN pipeline_stages bps ON bps.id = p.buyer_target_stage_id
                WHERE p.contract_id = m.contract_id
                  AND p.buyer_id = m.owner_account_id
                  AND p.status = 'active'
                  AND p.buyer_target_stage_id IS NOT NULL
                  AND bps.pipeline_id = m.pipeline_id
                ORDER BY p.id
                LIMIT 1),
               (SELECT ps2.id
                FROM pipeline_stages ps2
                WHERE ps2.pipeline_id = m.pipeline_id
                  AND ps2.name = m.old_stage_name
                ORDER BY ps2.position, ps2.id
                LIMIT 1),
               (SELECT ps3.id
                FROM pipeline_stages ps3
                WHERE ps3.pipeline_id = m.pipeline_id
                ORDER BY ps3.position, ps3.id
                LIMIT 1)
           ) AS new_stage_id
    FROM mismatched m
)
UPDATE leads l
SET stage_id = r.new_stage_id
FROM repaired r
WHERE l.id = r.lead_id
  AND r.new_stage_id IS NOT NULL;

-- Re-sync publisher mirror for contracted leads with publisher tracking.
UPDATE leads l
SET publisher_stage_id = COALESCE(
        (SELECT csm.publisher_stage_id
         FROM contract_stage_maps csm
         WHERE csm.contract_id = l.contract_id
           AND csm.buyer_stage_id = l.stage_id
         ORDER BY CASE WHEN csm.participation_id IS NOT NULL THEN 0 ELSE 1 END,
                  csm.id
         LIMIT 1),
        (SELECT c.source_stage_id FROM contracts c WHERE c.id = l.contract_id),
        l.publisher_stage_id
    ),
    publisher_pipeline_id = COALESCE(
        l.publisher_pipeline_id,
        (SELECT c.source_pipeline_id FROM contracts c WHERE c.id = l.contract_id)
    )
WHERE l.contract_id IS NOT NULL
  AND l.deleted_at IS NULL
  AND l.publisher_pipeline_id IS NOT NULL
  AND l.stage_id IS NOT NULL
  AND EXISTS (
      SELECT 1 FROM pipeline_stages ps
      WHERE ps.id = l.stage_id AND ps.pipeline_id = l.pipeline_id
  );
