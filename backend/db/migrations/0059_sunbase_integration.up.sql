INSERT INTO integration_providers (slug, name, description, auth_type, direction, config_schema, is_active)
VALUES (
    'sunbase',
    'SunBase',
    'Push and receive leads via SunBase lead_post API.',
    'api_key',
    'both',
    '{"endpoint_url":{"type":"string","label":"Endpoint URL","required":true},"schema_name":{"type":"string","label":"Schema Name","required":true}}',
    true
)
ON CONFLICT (slug) DO UPDATE SET
    name = EXCLUDED.name,
    description = EXCLUDED.description,
    auth_type = EXCLUDED.auth_type,
    direction = EXCLUDED.direction,
    config_schema = EXCLUDED.config_schema,
    is_active = EXCLUDED.is_active;
