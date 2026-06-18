CREATE TABLE route_executions (
  id                BIGSERIAL PRIMARY KEY,
  route_id          BIGINT REFERENCES routes(id) ON DELETE SET NULL,
  route_name        TEXT NOT NULL,
  lead_id           BIGINT NOT NULL REFERENCES leads(id),
  owner_account_id  BIGINT NOT NULL REFERENCES accounts(id),
  target_account_id BIGINT REFERENCES accounts(id),
  destination       TEXT NOT NULL,
  trigger_type      TEXT NOT NULL,
  trigger_label     TEXT,
  branch_position   INT,
  status            TEXT NOT NULL DEFAULT 'success',
  error_message     TEXT,
  reviewed_by       BIGINT REFERENCES users(id),
  created_at        TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_route_exec_owner ON route_executions(owner_account_id, created_at DESC);
CREATE INDEX idx_route_exec_target ON route_executions(target_account_id, created_at DESC)
  WHERE target_account_id IS NOT NULL;
