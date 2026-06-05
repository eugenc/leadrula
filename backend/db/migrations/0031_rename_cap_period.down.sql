ALTER TABLE contract_compensations RENAME CONSTRAINT contract_compensations_cap_period_check TO contract_compensations_billing_period_check;

ALTER TABLE contract_compensations RENAME COLUMN cap_period TO billing_period;

ALTER TABLE contracts DROP CONSTRAINT contracts_cap_period_check;
ALTER TABLE contracts
  ADD CONSTRAINT contracts_billing_period_check
    CHECK (cap_period IN ('one_time', 'weekly'));

ALTER TABLE contracts RENAME COLUMN cap_period TO billing_period;
