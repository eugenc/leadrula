ALTER TABLE route_executions DROP CONSTRAINT route_executions_lead_id_fkey;
ALTER TABLE route_executions
  ADD CONSTRAINT route_executions_lead_id_fkey
  FOREIGN KEY (lead_id) REFERENCES leads(id) ON DELETE CASCADE;
