ALTER TYPE source_type ADD VALUE IF NOT EXISTS 'appointment';

ALTER TABLE routing_sources
  ADD COLUMN contract_id              BIGINT REFERENCES contracts(id) ON DELETE SET NULL,
  ADD COLUMN delivery_mode            TEXT NOT NULL DEFAULT 'contract'
    CHECK (delivery_mode IN ('contract', 'publisher_pipeline')),
  ADD COLUMN publisher_pipeline_id    BIGINT REFERENCES pipelines(id) ON DELETE SET NULL,
  ADD COLUMN publisher_stage_id       BIGINT REFERENCES pipeline_stages(id) ON DELETE SET NULL,
  ADD COLUMN phone_match_mode         TEXT NOT NULL DEFAULT 'update_and_book'
    CHECK (phone_match_mode IN ('update_and_book', 'book_existing', 'reject_duplicate'));
