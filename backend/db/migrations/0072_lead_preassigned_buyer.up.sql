ALTER TABLE leads ADD COLUMN preassigned_buyer_id BIGINT REFERENCES accounts(id);
CREATE INDEX idx_leads_preassigned_buyer ON leads(preassigned_buyer_id);
