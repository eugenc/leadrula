CREATE TABLE dashboard_views (
    id              BIGSERIAL PRIMARY KEY,
    public_id       UUID NOT NULL DEFAULT gen_random_uuid() UNIQUE,
    account_id      BIGINT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    owner_user_id   BIGINT REFERENCES users(id) ON DELETE CASCADE,
    name            TEXT NOT NULL,
    widgets         JSONB NOT NULL DEFAULT '[]',
    period          TEXT NOT NULL DEFAULT 'all' CHECK (period IN ('today', 'week', 'month', 'all')),
    goals           JSONB NOT NULL DEFAULT '{}',
    position        INT NOT NULL DEFAULT 0,
    created_by      BIGINT NOT NULL REFERENCES users(id),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_dashboard_views_account ON dashboard_views(account_id);
