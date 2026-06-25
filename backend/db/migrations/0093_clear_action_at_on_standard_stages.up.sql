UPDATE leads l
SET action_at = NULL
FROM pipeline_stages st
WHERE l.stage_id = st.id
  AND st.stage_type = 'standard'
  AND l.action_at IS NOT NULL;
