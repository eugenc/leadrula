ALTER TABLE contracts RENAME COLUMN billing_period TO cap_period;

ALTER TABLE contracts RENAME CONSTRAINT contracts_billing_period_check TO contracts_cap_period_check;

ALTER TABLE contracts DROP CONSTRAINT contracts_cap_period_check;
ALTER TABLE contracts
  ADD CONSTRAINT contracts_cap_period_check
    CHECK (cap_period IN ('one_time', 'weekly', 'monthly'));

ALTER TABLE contract_compensations RENAME COLUMN billing_period TO cap_period;

ALTER TABLE contract_compensations RENAME CONSTRAINT contract_compensations_billing_period_check TO contract_compensations_cap_period_check;
