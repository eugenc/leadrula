DROP TABLE IF EXISTS scheduled_lead_returns;

ALTER TABLE contract_return_rules
    DROP CONSTRAINT IF EXISTS contract_return_rules_schedule_mode_check;

ALTER TABLE contract_return_rules
    DROP COLUMN IF EXISTS return_weekdays,
    DROP COLUMN IF EXISTS return_time,
    DROP COLUMN IF EXISTS return_delay_seconds,
    DROP COLUMN IF EXISTS return_schedule_mode;

ALTER TABLE contracts DROP COLUMN IF EXISTS schedule_timezone;
