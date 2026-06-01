ALTER TABLE pipeline_stages
    ADD COLUMN prompt_action_datetime BOOLEAN NOT NULL DEFAULT true,
    ADD COLUMN prompt_disqualification BOOLEAN NOT NULL DEFAULT false;

UPDATE pipeline_stages SET
    prompt_action_datetime = (stage_type = 'action'),
    prompt_disqualification = (stage_type = 'disqualification');

ALTER TABLE pipeline_stages DROP COLUMN stage_type;
