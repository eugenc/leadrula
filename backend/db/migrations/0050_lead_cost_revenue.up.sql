ALTER TABLE leads
  ADD COLUMN cost NUMERIC(12,2),
  ADD COLUMN revenue NUMERIC(12,2);

ALTER TABLE compensation_earnings
  ADD COLUMN cost_basis NUMERIC(12,2);
