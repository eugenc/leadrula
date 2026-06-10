DROP TABLE IF EXISTS compensation_payout_clears;
DROP TABLE IF EXISTS compensation_earnings;

ALTER TABLE contract_compensations
  DROP CONSTRAINT IF EXISTS contract_compensations_payout_month_day_check,
  DROP CONSTRAINT IF EXISTS contract_compensations_payout_weekday_check,
  DROP CONSTRAINT IF EXISTS contract_compensations_payout_frequency_check,
  DROP COLUMN IF EXISTS payout_month_day,
  DROP COLUMN IF EXISTS payout_weekday,
  DROP COLUMN IF EXISTS payout_frequency;
