ALTER TABLE leads ADD COLUMN tags TEXT[] NOT NULL DEFAULT '{}';
CREATE INDEX idx_leads_tags ON leads USING gin (tags);
