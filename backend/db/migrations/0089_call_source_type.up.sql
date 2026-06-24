-- Call leads: add the 'call' source type. Isolated from table changes so the
-- new enum value is committed before any later migration uses it as a literal.
ALTER TYPE source_type ADD VALUE IF NOT EXISTS 'call';
