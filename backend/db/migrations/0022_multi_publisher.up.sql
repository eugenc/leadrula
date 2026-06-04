DROP INDEX IF EXISTS one_publisher;

ALTER TYPE account_type ADD VALUE IF NOT EXISTS 'platform';

CREATE TABLE account_switch_log (
    id              BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    actor_user_id   BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    from_account_id BIGINT NOT NULL REFERENCES accounts(id),
    to_account_id   BIGINT NOT NULL REFERENCES accounts(id),
    switched_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_switch_log_actor ON account_switch_log(actor_user_id, switched_at DESC);
