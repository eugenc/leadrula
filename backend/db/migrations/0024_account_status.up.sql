CREATE TYPE account_operational_status AS ENUM ('active', 'suspended');

ALTER TABLE accounts
  ADD COLUMN operational_status account_operational_status NOT NULL DEFAULT 'active';
