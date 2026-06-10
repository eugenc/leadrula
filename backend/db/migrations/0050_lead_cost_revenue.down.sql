ALTER TABLE compensation_earnings DROP COLUMN IF EXISTS cost_basis;

ALTER TABLE leads
  DROP COLUMN IF EXISTS revenue,
  DROP COLUMN IF EXISTS cost;
