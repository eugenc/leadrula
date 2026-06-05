ALTER TABLE contracts DROP CONSTRAINT IF EXISTS contracts_cap_daily_nonneg_check;
ALTER TABLE contracts DROP CONSTRAINT IF EXISTS contracts_cap_nonneg_check;
ALTER TABLE contracts DROP CONSTRAINT IF EXISTS contracts_billing_period_check;
ALTER TABLE contracts
  DROP COLUMN IF EXISTS cap_max_daily,
  DROP COLUMN IF EXISTS cap_total,
  DROP COLUMN IF EXISTS billing_period;
