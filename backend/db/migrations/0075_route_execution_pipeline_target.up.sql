ALTER TABLE route_executions
  ADD COLUMN target_pipeline_id BIGINT,
  ADD COLUMN target_stage_id BIGINT,
  ADD COLUMN target_pipeline_name TEXT,
  ADD COLUMN target_stage_name TEXT,
  ADD COLUMN delivery TEXT;

WITH resolved AS (
  SELECT
    e.id,
    COALESCE(
      CASE WHEN COALESCE(e.branch_position, 0) > 0 THEN
        (SELECT (b.elem->>'target_pipeline_id')::bigint
         FROM routes r2
         CROSS JOIN LATERAL jsonb_array_elements(r2.branches) AS b(elem)
         WHERE r2.id = e.route_id
           AND (b.elem->>'position')::int = e.branch_position
           AND COALESCE(b.elem->>'destination', '') = 'pipeline'
         LIMIT 1)
      END,
      r.target_pipeline_id
    ) AS pipeline_id,
    COALESCE(
      CASE WHEN COALESCE(e.branch_position, 0) > 0 THEN
        (SELECT (b.elem->>'target_stage_id')::bigint
         FROM routes r2
         CROSS JOIN LATERAL jsonb_array_elements(r2.branches) AS b(elem)
         WHERE r2.id = e.route_id
           AND (b.elem->>'position')::int = e.branch_position
           AND COALESCE(b.elem->>'destination', '') = 'pipeline'
         LIMIT 1)
      END,
      r.target_stage_id
    ) AS stage_id,
    COALESCE(
      CASE WHEN COALESCE(e.branch_position, 0) > 0 THEN
        (SELECT b.elem->>'delivery'
         FROM routes r2
         CROSS JOIN LATERAL jsonb_array_elements(r2.branches) AS b(elem)
         WHERE r2.id = e.route_id
           AND (b.elem->>'position')::int = e.branch_position
           AND COALESCE(b.elem->>'destination', '') = 'pipeline'
         LIMIT 1)
      END,
      r.delivery::text
    ) AS delivery
  FROM route_executions e
  JOIN routes r ON r.id = e.route_id
  WHERE e.destination = 'pipeline'
    AND e.route_id IS NOT NULL
)
UPDATE route_executions e
SET
  target_pipeline_id = resolved.pipeline_id,
  target_stage_id = resolved.stage_id,
  delivery = resolved.delivery,
  target_pipeline_name = p.name,
  target_stage_name = ps.name
FROM resolved
LEFT JOIN pipelines p ON p.id = resolved.pipeline_id
LEFT JOIN pipeline_stages ps ON ps.id = resolved.stage_id
WHERE e.id = resolved.id;
