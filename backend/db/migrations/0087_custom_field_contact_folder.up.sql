ALTER TABLE custom_field_folders
  ADD COLUMN IF NOT EXISTS is_system BOOLEAN NOT NULL DEFAULT false,
  ADD COLUMN IF NOT EXISTS system_key TEXT;

CREATE UNIQUE INDEX IF NOT EXISTS idx_cff_account_system_key
  ON custom_field_folders(account_id, system_key)
  WHERE system_key IS NOT NULL;

UPDATE custom_field_folders SET position = position + 1
WHERE is_system = false
  AND account_id IN (
    SELECT a.id FROM accounts a
    WHERE NOT EXISTS (
      SELECT 1 FROM custom_field_folders f
      WHERE f.account_id = a.id AND f.system_key = 'contact'
    )
  );

INSERT INTO custom_field_folders (account_id, name, position, is_system, system_key)
SELECT a.id, 'Contact', 0, true, 'contact'
FROM accounts a
WHERE NOT EXISTS (
  SELECT 1 FROM custom_field_folders f
  WHERE f.account_id = a.id AND f.system_key = 'contact'
);
