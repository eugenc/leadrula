CREATE TABLE lead_stage_history (
    id                         BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    lead_id                    BIGINT NOT NULL REFERENCES leads(id) ON DELETE CASCADE,
    from_stage_id              BIGINT REFERENCES pipeline_stages(id),
    to_stage_id                BIGINT NOT NULL REFERENCES pipeline_stages(id),
    moved_by_user_id           BIGINT REFERENCES users(id),
    action_at_captured         TIMESTAMPTZ,
    disqualification_reason_id BIGINT REFERENCES disqualification_reasons(id),
    created_at                 TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_stage_history_lead ON lead_stage_history(lead_id, created_at DESC);
