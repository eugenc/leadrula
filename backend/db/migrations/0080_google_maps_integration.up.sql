INSERT INTO integration_providers (slug, name, description, auth_type, direction, config_schema)
VALUES (
  'google_maps',
  'Google Maps',
  'Validate lead addresses and show property locations on a map.',
  'api_key',
  'both',
  '{"type":"object","properties":{}}'
);
