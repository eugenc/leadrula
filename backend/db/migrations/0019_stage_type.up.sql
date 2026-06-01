ALTER TABLE pipeline_stages
    ADD COLUMN stage_type TEXT NOT NULL DEFAULT 'action'
        CHECK (stage_type IN ('standard', 'action', 'disqualification', 'won'));

UPDATE pipeline_stages SET stage_type = CASE
    WHEN prompt_disqualification THEN 'disqualification'
    WHEN prompt_action_datetime THEN 'action'
    ELSE 'standard'
END;

ALTER TABLE pipeline_stages
    DROP COLUMN prompt_action_datetime,
    DROP COLUMN prompt_disqualification;
