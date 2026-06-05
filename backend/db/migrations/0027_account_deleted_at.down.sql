DROP INDEX IF EXISTS idx_accounts_not_deleted;
ALTER TABLE accounts DROP COLUMN IF EXISTS deleted_at;
