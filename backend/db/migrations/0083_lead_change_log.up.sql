CREATE TABLE lead_change_log (
  id               BIGSERIAL PRIMARY KEY,
  lead_id          BIGINT NOT NULL REFERENCES leads(id) ON DELETE CASCADE,
  owner_account_id BIGINT NOT NULL REFERENCES accounts(id),
  actor_type       TEXT NOT NULL,
  actor_user_id    BIGINT REFERENCES users(id),
  actor_label      TEXT,
  change_kind      TEXT NOT NULL,
  field_name       TEXT,
  from_value       TEXT,
  to_value         TEXT,
  created_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX lead_change_log_lead_id_created_at_idx ON lead_change_log (lead_id, created_at DESC);
