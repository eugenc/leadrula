CREATE INDEX idx_txn_lead_debit ON transactions(lead_id, created_at DESC) WHERE type = 'debit' AND lead_id IS NOT NULL;
CREATE INDEX idx_comp_earnings_lead ON compensation_earnings(lead_id, created_at DESC) WHERE kind = 'distribute';
