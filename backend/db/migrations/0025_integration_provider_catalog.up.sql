INSERT INTO integration_providers (slug, name, description, auth_type, direction, config_schema, is_active) VALUES
    ('pipedrive', 'Pipedrive', 'Push routed leads as persons and optional deals in Pipedrive.', 'oauth2', 'outbound', '{}', true),
    ('ghl', 'GoHighLevel', 'Send leads to GHL contacts and optional pipeline stages.', 'api_key', 'outbound', '{"location_id": {"type": "string", "label": "Location ID", "required": true}}', true),
    ('hubspot', 'HubSpot', 'Create HubSpot contacts from distributed leads.', 'oauth2', 'outbound', '{}', true),
    ('salesforce', 'Salesforce', 'Create Salesforce leads on your connected org.', 'oauth2', 'outbound', '{"instance_url": {"type": "string", "label": "Instance URL", "required": false}}', true)
ON CONFLICT (slug) DO UPDATE SET
    name = EXCLUDED.name,
    description = EXCLUDED.description,
    auth_type = EXCLUDED.auth_type,
    direction = EXCLUDED.direction,
    config_schema = EXCLUDED.config_schema,
    is_active = EXCLUDED.is_active;
