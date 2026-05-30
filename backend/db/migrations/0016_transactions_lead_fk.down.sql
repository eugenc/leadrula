ALTER TABLE transactions DROP CONSTRAINT transactions_lead_id_fkey;
ALTER TABLE transactions
  ADD CONSTRAINT transactions_lead_id_fkey
  FOREIGN KEY (lead_id) REFERENCES leads(id);
