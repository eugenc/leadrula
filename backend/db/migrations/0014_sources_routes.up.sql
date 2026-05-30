CREATE TYPE route_origin AS ENUM ('source','pipeline');
CREATE TYPE route_destination AS ENUM ('publisher','buyer');
CREATE TYPE route_delivery AS ENUM ('leads','leads_pipeline');

CREATE TABLE routing_sources (
    id           BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    publisher_id BIGINT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    name         TEXT NOT NULL,
    slug         TEXT NOT NULL UNIQUE,
    is_active    BOOLEAN NOT NULL DEFAULT true,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_routing_sources_publisher ON routing_sources(publisher_id);

CREATE TABLE routing_source_field_map (
    id              BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    source_id       BIGINT NOT NULL REFERENCES routing_sources(id) ON DELETE CASCADE,
    source_key      TEXT NOT NULL,
    target_type     map_target NOT NULL,
    builtin_field   TEXT,
    custom_field_id BIGINT REFERENCES custom_fields(id),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK ( (target_type='builtin' AND builtin_field IS NOT NULL AND custom_field_id IS NULL)
         OR (target_type='custom'  AND custom_field_id IS NOT NULL AND builtin_field IS NULL) )
);
CREATE INDEX idx_source_fieldmap ON routing_source_field_map(source_id);

CREATE TABLE routes (
    id                 BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    publisher_id       BIGINT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    name               TEXT NOT NULL,
    origin             route_origin NOT NULL,
    source_id          BIGINT REFERENCES routing_sources(id) ON DELETE CASCADE,
    origin_pipeline_id BIGINT REFERENCES pipelines(id),
    origin_stage_id    BIGINT REFERENCES pipeline_stages(id),
    destination        route_destination NOT NULL,
    contract_id        BIGINT REFERENCES contracts(id),
    delivery           route_delivery NOT NULL DEFAULT 'leads_pipeline',
    target_pipeline_id BIGINT REFERENCES pipelines(id),
    target_stage_id    BIGINT REFERENCES pipeline_stages(id),
    is_active          BOOLEAN NOT NULL DEFAULT true,
    created_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK (
        (origin = 'source' AND source_id IS NOT NULL AND origin_pipeline_id IS NULL AND origin_stage_id IS NULL)
        OR (origin = 'pipeline' AND source_id IS NULL AND origin_pipeline_id IS NOT NULL AND origin_stage_id IS NOT NULL)
    ),
    CHECK (
        (destination = 'buyer' AND contract_id IS NOT NULL)
        OR (destination = 'publisher' AND contract_id IS NULL)
    ),
    CHECK (NOT (origin = 'pipeline' AND destination = 'publisher')),
    CHECK (
        (destination = 'publisher' AND delivery = 'leads')
        OR (destination = 'publisher' AND delivery = 'leads_pipeline' AND target_pipeline_id IS NOT NULL AND target_stage_id IS NOT NULL)
        OR (destination = 'buyer')
    )
);
CREATE INDEX idx_routes_publisher ON routes(publisher_id);
CREATE INDEX idx_routes_source ON routes(source_id) WHERE source_id IS NOT NULL;
CREATE INDEX idx_routes_stage ON routes(publisher_id, origin_stage_id) WHERE origin = 'pipeline';

CREATE TABLE route_field_map (
    id                  BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    route_id            BIGINT NOT NULL REFERENCES routes(id) ON DELETE CASCADE,
    src_type            map_target NOT NULL,
    src_builtin         TEXT,
    src_custom_field_id BIGINT REFERENCES custom_fields(id),
    dst_type            map_target NOT NULL,
    dst_builtin         TEXT,
    dst_custom_field_id BIGINT REFERENCES custom_fields(id),
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK ( (src_type='builtin' AND src_builtin IS NOT NULL AND src_custom_field_id IS NULL)
         OR (src_type='custom'  AND src_custom_field_id IS NOT NULL AND src_builtin IS NULL) ),
    CHECK ( (dst_type='builtin' AND dst_builtin IS NOT NULL AND dst_custom_field_id IS NULL)
         OR (dst_type='custom'  AND dst_custom_field_id IS NOT NULL AND dst_builtin IS NULL) )
);
CREATE INDEX idx_route_fieldmap ON route_field_map(route_id);

DROP TABLE routing_field_map;
DROP TABLE routing_campaigns;
