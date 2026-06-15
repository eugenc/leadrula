ALTER TABLE routes
  ADD COLUMN condition_logic TEXT NOT NULL DEFAULT 'and',
  ADD COLUMN conditions JSONB NOT NULL DEFAULT '[]',
  ADD COLUMN position INT NOT NULL DEFAULT 0;

CREATE INDEX idx_routes_origin_priority ON routes(origin, position);
