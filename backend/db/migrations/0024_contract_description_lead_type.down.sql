ALTER TABLE contracts
  DROP COLUMN IF EXISTS description,
  DROP COLUMN IF EXISTS lead_type;
