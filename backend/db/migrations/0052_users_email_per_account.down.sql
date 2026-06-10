DROP INDEX IF EXISTS users_email_account_id;

CREATE UNIQUE INDEX users_email_key ON users (email);
