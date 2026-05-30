ALTER TABLE pipeline_stages
    ADD COLUMN color TEXT NOT NULL DEFAULT 'gray';

CREATE TABLE stage_rules (
    id               BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    stage_id         BIGINT NOT NULL REFERENCES pipeline_stages(id) ON DELETE CASCADE,
    position         INT NOT NULL DEFAULT 0,
    condition_logic  TEXT NOT NULL DEFAULT 'and' CHECK (condition_logic IN ('and', 'or')),
    conditions       JSONB NOT NULL DEFAULT '[]',
    actions          JSONB NOT NULL DEFAULT '[]',
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_stage_rules_stage ON stage_rules(stage_id, position);
