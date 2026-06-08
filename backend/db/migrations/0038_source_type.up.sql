CREATE TYPE source_type AS ENUM ('webhook');

ALTER TABLE routing_sources
  ADD COLUMN type source_type NOT NULL DEFAULT 'webhook';
