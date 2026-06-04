DROP TABLE IF EXISTS account_switch_log;

CREATE UNIQUE INDEX IF NOT EXISTS one_publisher ON accounts ((type)) WHERE type = 'publisher';
