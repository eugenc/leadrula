CREATE TYPE lead_status AS ENUM
    ('review','distributed','returned','closed');

CREATE TABLE leads (
    id               BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    public_id        UUID NOT NULL DEFAULT gen_random_uuid() UNIQUE,
    owner_account_id BIGINT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    publisher_id     BIGINT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    contract_id      BIGINT REFERENCES contracts(id),
    first_name       TEXT NOT NULL DEFAULT '',
    last_name        TEXT NOT NULL DEFAULT '',
    phone            TEXT,
    email            CITEXT,
    address          TEXT,
    city             TEXT,
    state            TEXT,
    zip              TEXT,
    campaign_name    TEXT,
    raw_payload      JSONB NOT NULL DEFAULT '{}',
    pipeline_id      BIGINT REFERENCES pipelines(id),
    stage_id         BIGINT REFERENCES pipeline_stages(id),
    position         INT NOT NULL DEFAULT 0,
    assigned_user_id BIGINT REFERENCES users(id),
    action_at        TIMESTAMPTZ,
    status           lead_status NOT NULL DEFAULT 'review',
    disqualification_reason_id BIGINT REFERENCES disqualification_reasons(id),
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_leads_owner    ON leads(owner_account_id);
CREATE INDEX idx_leads_status   ON leads(status);
CREATE INDEX idx_leads_campaign ON leads(campaign_name);
CREATE INDEX idx_leads_stage    ON leads(stage_id);
CREATE INDEX idx_leads_assigned ON leads(assigned_user_id);
CREATE INDEX idx_leads_action   ON leads(action_at);
CREATE INDEX idx_leads_payload  ON leads USING gin(raw_payload);

CREATE TABLE lead_notes (
    id         BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    lead_id    BIGINT NOT NULL REFERENCES leads(id) ON DELETE CASCADE,
    user_id    BIGINT REFERENCES users(id),
    body       TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_lead_notes_lead ON lead_notes(lead_id);

CREATE TABLE lead_followers (
    lead_id BIGINT NOT NULL REFERENCES leads(id) ON DELETE CASCADE,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (lead_id, user_id)
);

CREATE TABLE lead_custom_values (
    lead_id         BIGINT NOT NULL REFERENCES leads(id) ON DELETE CASCADE,
    custom_field_id BIGINT NOT NULL REFERENCES custom_fields(id) ON DELETE CASCADE,
    value           JSONB NOT NULL,
    PRIMARY KEY (lead_id, custom_field_id)
);
CREATE INDEX idx_lcv_field ON lead_custom_values(custom_field_id);
