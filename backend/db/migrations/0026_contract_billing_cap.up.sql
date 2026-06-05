ALTER TABLE contracts
  ADD COLUMN billing_period TEXT NOT NULL DEFAULT 'one_time',
  ADD COLUMN cap_total INT,
  ADD COLUMN cap_max_daily INT;

ALTER TABLE contracts
  ADD CONSTRAINT contracts_billing_period_check
    CHECK (billing_period IN ('one_time', 'weekly'));

ALTER TABLE contracts
  ADD CONSTRAINT contracts_cap_nonneg_check
    CHECK (cap_total IS NULL OR cap_total > 0),
  ADD CONSTRAINT contracts_cap_daily_nonneg_check
    CHECK (cap_max_daily IS NULL OR cap_max_daily > 0);
