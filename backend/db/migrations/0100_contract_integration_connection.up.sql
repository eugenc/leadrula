ALTER TABLE contracts
  ADD COLUMN integration_connection_id BIGINT
  REFERENCES integration_connections(id) ON DELETE SET NULL;
