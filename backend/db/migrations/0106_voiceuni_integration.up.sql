INSERT INTO integration_providers (slug, name, description, auth_type, direction, config_schema, is_active)
VALUES (
    'voiceuni',
    'VoiceUni',
    'Receive leads from VoiceUni dialer via public API ingest.',
    'none',
    'inbound',
    '{"source_slug":{"type":"string","label":"Lead source slug"},"call_source_slug":{"type":"string","label":"Call preload source slug"}}',
    true
)
ON CONFLICT (slug) DO UPDATE SET
    name = EXCLUDED.name,
    description = EXCLUDED.description,
    auth_type = EXCLUDED.auth_type,
    direction = EXCLUDED.direction,
    config_schema = EXCLUDED.config_schema,
    is_active = EXCLUDED.is_active;
