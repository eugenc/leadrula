CREATE TABLE publisher_partnerships (
    id              BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    publisher_a_id  BIGINT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    publisher_b_id  BIGINT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    status          TEXT NOT NULL DEFAULT 'active',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT publisher_partnerships_status_check
      CHECK (status IN ('active', 'revoked')),
    CONSTRAINT publisher_partnerships_order_check
      CHECK (publisher_a_id < publisher_b_id),
    UNIQUE (publisher_a_id, publisher_b_id)
);

CREATE INDEX idx_publisher_partnerships_a ON publisher_partnerships(publisher_a_id);
CREATE INDEX idx_publisher_partnerships_b ON publisher_partnerships(publisher_b_id);
