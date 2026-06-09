-- Enum value must be committed before it can appear in CHECK constraints (separate migration).
ALTER TYPE map_target ADD VALUE IF NOT EXISTS 'ignore';
