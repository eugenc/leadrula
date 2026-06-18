-- Re-backfill any stage history rows that still lack owner_account_id.
UPDATE lead_stage_history h
SET owner_account_id = COALESCE(
  (
    SELECT CASE
      WHEN re.trigger_type = 'return' THEN re.target_account_id
      ELSE re.target_account_id
    END
    FROM route_executions re
    WHERE re.lead_id = h.lead_id
      AND re.created_at <= h.created_at
      AND re.target_account_id IS NOT NULL
    ORDER BY re.created_at DESC, re.id DESC
    LIMIT 1
  ),
  (SELECT l.publisher_id FROM leads l WHERE l.id = h.lead_id)
)
WHERE h.owner_account_id IS NULL;
