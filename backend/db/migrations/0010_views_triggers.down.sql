DROP TRIGGER IF EXISTS trg_balances_updated  ON buyer_balances;
DROP TRIGGER IF EXISTS trg_leads_updated     ON leads;
DROP TRIGGER IF EXISTS trg_contracts_updated ON contracts;
DROP TRIGGER IF EXISTS trg_pipelines_updated ON pipelines;
DROP TRIGGER IF EXISTS trg_users_updated     ON users;
DROP TRIGGER IF EXISTS trg_accounts_updated  ON accounts;
DROP FUNCTION IF EXISTS set_updated_at();
DROP VIEW IF EXISTS v_calendar;
