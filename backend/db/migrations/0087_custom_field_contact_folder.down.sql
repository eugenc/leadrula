DELETE FROM custom_field_folders WHERE system_key = 'contact';

ALTER TABLE custom_field_folders DROP COLUMN IF EXISTS system_key;
ALTER TABLE custom_field_folders DROP COLUMN IF EXISTS is_system;
