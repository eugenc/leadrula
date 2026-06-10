ALTER TABLE contract_filter_rules DROP CONSTRAINT IF EXISTS contract_filter_rules_operator_check;
ALTER TABLE contract_filter_rules ADD CONSTRAINT contract_filter_rules_operator_check
  CHECK (operator IN ('eq', 'neq', 'contains', 'not_empty', 'gt', 'lt'));
