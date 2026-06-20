ALTER TABLE custom_field_folders
  ADD COLUMN IF NOT EXISTS contact_builtin_order TEXT[];
