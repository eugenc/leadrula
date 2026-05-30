CREATE TYPE collaboration_status AS ENUM (
    'active',
    'revoked',
    'pending_buyer',
    'pending_publisher'
);

CREATE TYPE collaboration_event_type AS ENUM (
    'granted',
    'revoked',
    'request_sent',
    'request_accepted',
    'request_rejected',
    'impersonation_start',
    'impersonation_end',
    'impersonation_action'
);

CREATE TABLE buyer_collaborations (
    id                         BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    publisher_id               BIGINT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    buyer_id                   BIGINT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    status                     collaboration_status NOT NULL,
    version                    BIGINT NOT NULL DEFAULT 1,
    auto_granted               BOOLEAN NOT NULL DEFAULT false,
    target_publisher_user_id   BIGINT REFERENCES users(id) ON DELETE SET NULL,
    requested_by_user_id       BIGINT REFERENCES users(id) ON DELETE SET NULL,
    revoked_at                 TIMESTAMPTZ,
    revoked_by_user_id         BIGINT REFERENCES users(id) ON DELETE SET NULL,
    created_at                 TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at                 TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (publisher_id, buyer_id)
);
CREATE INDEX idx_collab_buyer ON buyer_collaborations(buyer_id);
CREATE INDEX idx_collab_publisher ON buyer_collaborations(publisher_id);

CREATE TABLE collaboration_audit_log (
    id            BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    event_type    collaboration_event_type NOT NULL,
    publisher_id  BIGINT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    buyer_id      BIGINT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    actor_user_id BIGINT REFERENCES users(id) ON DELETE SET NULL,
    metadata      JSONB NOT NULL DEFAULT '{}',
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_collab_audit_buyer ON collaboration_audit_log(buyer_id, created_at DESC);

ALTER TYPE notification_type ADD VALUE IF NOT EXISTS 'collaboration_request';
