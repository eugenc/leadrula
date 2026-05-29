CREATE VIEW v_calendar AS
SELECT l.id, l.owner_account_id AS account_id, l.assigned_user_id AS user_id,
       l.pipeline_id, l.stage_id,
       (l.first_name || ' ' || l.last_name) AS title,
       l.action_at
FROM leads l
WHERE l.action_at IS NOT NULL;

CREATE OR REPLACE FUNCTION set_updated_at() RETURNS trigger AS $$
BEGIN NEW.updated_at = now(); RETURN NEW; END; $$ LANGUAGE plpgsql;

CREATE TRIGGER trg_accounts_updated  BEFORE UPDATE ON accounts
  FOR EACH ROW EXECUTE FUNCTION set_updated_at();
CREATE TRIGGER trg_users_updated     BEFORE UPDATE ON users
  FOR EACH ROW EXECUTE FUNCTION set_updated_at();
CREATE TRIGGER trg_pipelines_updated BEFORE UPDATE ON pipelines
  FOR EACH ROW EXECUTE FUNCTION set_updated_at();
CREATE TRIGGER trg_contracts_updated BEFORE UPDATE ON contracts
  FOR EACH ROW EXECUTE FUNCTION set_updated_at();
CREATE TRIGGER trg_leads_updated     BEFORE UPDATE ON leads
  FOR EACH ROW EXECUTE FUNCTION set_updated_at();
CREATE TRIGGER trg_balances_updated  BEFORE UPDATE ON buyer_balances
  FOR EACH ROW EXECUTE FUNCTION set_updated_at();
