DROP INDEX IF EXISTS idx_pipelines_collab_publisher;
ALTER TABLE pipelines DROP COLUMN IF EXISTS collaboration_publisher_id;
