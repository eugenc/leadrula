ALTER TABLE routing_sources
    ADD COLUMN api_key_required BOOLEAN NOT NULL DEFAULT true;
