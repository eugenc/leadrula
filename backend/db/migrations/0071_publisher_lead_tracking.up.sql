ALTER TABLE leads
  ADD COLUMN publisher_pipeline_id BIGINT REFERENCES pipelines(id) ON DELETE SET NULL,
  ADD COLUMN publisher_stage_id BIGINT REFERENCES pipeline_stages(id) ON DELETE SET NULL;

CREATE TABLE contract_stage_maps (
  id                 BIGSERIAL PRIMARY KEY,
  contract_id        BIGINT NOT NULL REFERENCES contracts(id) ON DELETE CASCADE,
  participation_id   BIGINT REFERENCES contract_participations(id) ON DELETE CASCADE,
  buyer_stage_id     BIGINT NOT NULL REFERENCES pipeline_stages(id) ON DELETE CASCADE,
  publisher_stage_id BIGINT NOT NULL REFERENCES pipeline_stages(id) ON DELETE CASCADE,
  UNIQUE (contract_id, participation_id, buyer_stage_id)
);

CREATE INDEX idx_contract_stage_maps_contract ON contract_stage_maps(contract_id);
CREATE INDEX idx_leads_publisher_tracking ON leads(publisher_id, publisher_pipeline_id)
  WHERE publisher_pipeline_id IS NOT NULL;
