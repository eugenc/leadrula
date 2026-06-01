DROP TABLE IF EXISTS partnerships;
DROP TYPE IF EXISTS partnership_status;
DROP INDEX IF EXISTS idx_contracts_handler_id;
ALTER TABLE contracts DROP COLUMN IF EXISTS handler_id;
DROP INDEX IF EXISTS idx_accounts_handler_id;
ALTER TABLE accounts DROP COLUMN IF EXISTS handler_id;
