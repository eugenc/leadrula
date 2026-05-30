CREATE TABLE lead_saved_views (
    id              BIGSERIAL PRIMARY KEY,
    public_id       UUID NOT NULL DEFAULT gen_random_uuid() UNIQUE,
    account_id      BIGINT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    owner_user_id   BIGINT REFERENCES users(id) ON DELETE CASCADE,
    name            TEXT NOT NULL,
    placement       TEXT NOT NULL CHECK (placement IN ('list', 'board', 'both')),
    filters         JSONB NOT NULL DEFAULT '[]',
    columns         JSONB,
    sort            TEXT,
    sort_dir        TEXT CHECK (sort_dir IN ('asc', 'desc')),
    is_builtin      BOOLEAN NOT NULL DEFAULT false,
    builtin_key     TEXT,
    position        INT NOT NULL DEFAULT 0,
    created_by      BIGINT NOT NULL REFERENCES users(id),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_lead_saved_views_account ON lead_saved_views(account_id);
CREATE INDEX idx_lead_saved_views_owner ON lead_saved_views(owner_user_id);
