ALTER TABLE accounts ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMPTZ;

CREATE INDEX IF NOT EXISTS idx_accounts_not_deleted ON accounts (type) WHERE deleted_at IS NULL;
