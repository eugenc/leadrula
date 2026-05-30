CREATE TABLE routing_campaigns (
    id                 BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    publisher_id       BIGINT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    campaign_name      TEXT NOT NULL,
    target_pipeline_id BIGINT NOT NULL REFERENCES pipelines(id),
    target_stage_id    BIGINT NOT NULL REFERENCES pipeline_stages(id),
    is_active          BOOLEAN NOT NULL DEFAULT true,
    created_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (publisher_id, campaign_name)
);

CREATE TABLE routing_field_map (
    id              BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    campaign_id     BIGINT NOT NULL REFERENCES routing_campaigns(id) ON DELETE CASCADE,
    source_key      TEXT NOT NULL,
    target_type     map_target NOT NULL,
    builtin_field   TEXT,
    custom_field_id BIGINT REFERENCES custom_fields(id),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK ( (target_type='builtin' AND builtin_field IS NOT NULL AND custom_field_id IS NULL)
         OR (target_type='custom'  AND custom_field_id IS NOT NULL AND builtin_field IS NULL) )
);
CREATE INDEX idx_fieldmap_campaign ON routing_field_map(campaign_id);

DROP TABLE route_field_map;
DROP TABLE routes;
DROP TABLE routing_source_field_map;
DROP TABLE routing_sources;
DROP TYPE route_delivery;
DROP TYPE route_destination;
DROP TYPE route_origin;
