ALTER TABLE pipelines
  ADD COLUMN collaboration_publisher_id BIGINT REFERENCES accounts(id) ON DELETE SET NULL;

CREATE INDEX idx_pipelines_collab_publisher
  ON pipelines(account_id, collaboration_publisher_id)
  WHERE collaboration_publisher_id IS NOT NULL;
