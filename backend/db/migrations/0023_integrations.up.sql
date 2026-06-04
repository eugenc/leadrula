CREATE TABLE integration_providers (
    id            BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    slug          TEXT NOT NULL UNIQUE,
    name          TEXT NOT NULL,
    description   TEXT NOT NULL DEFAULT '',
    auth_type     TEXT NOT NULL,
    direction     TEXT NOT NULL,
    config_schema JSONB NOT NULL DEFAULT '{}',
    is_active     BOOLEAN NOT NULL DEFAULT true,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

INSERT INTO integration_providers (slug, name, auth_type, direction, config_schema) VALUES
    ('webhook', 'Webhook', 'none', 'outbound', '{}'),
    ('ghl', 'GoHighLevel', 'api_key', 'outbound', '{"location_id": {"type": "string", "label": "Location ID", "required": true}}'),
    ('pipedrive', 'Pipedrive', 'oauth2', 'outbound', '{}'),
    ('hubspot', 'HubSpot', 'oauth2', 'outbound', '{}'),
    ('zoho_crm', 'Zoho CRM', 'oauth2', 'outbound', '{"api_domain": {"type": "string", "label": "Zoho data center", "required": true, "enum": ["com","eu","in","com.au","jp"]}}'),
    ('salesforce', 'Salesforce', 'oauth2', 'outbound', '{"instance_url": {"type": "string", "label": "Instance URL", "required": false}}'),
    ('google_calendar', 'Google Calendar', 'oauth2', 'both', '{}'),
    ('calendly', 'Calendly', 'oauth2', 'inbound', '{}');

CREATE TABLE integration_connections (
    id          BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    public_id   UUID NOT NULL DEFAULT gen_random_uuid() UNIQUE,
    account_id  BIGINT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    provider_id BIGINT NOT NULL REFERENCES integration_providers(id),
    name        TEXT NOT NULL,
    credentials BYTEA,
    oauth_state TEXT,
    config      JSONB NOT NULL DEFAULT '{}',
    status      TEXT NOT NULL DEFAULT 'active',
    last_error  TEXT,
    last_used_at TIMESTAMPTZ,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (account_id, provider_id, name)
);
CREATE INDEX idx_integrations_account ON integration_connections(account_id);
CREATE INDEX idx_integrations_provider ON integration_connections(provider_id);

CREATE TABLE route_integrations (
    id              BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    route_id        BIGINT NOT NULL REFERENCES routes(id) ON DELETE CASCADE,
    connection_id   BIGINT NOT NULL REFERENCES integration_connections(id) ON DELETE CASCADE,
    delivery_config JSONB NOT NULL DEFAULT '{}',
    is_active       BOOLEAN NOT NULL DEFAULT true,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (route_id, connection_id)
);
CREATE INDEX idx_route_integrations_route ON route_integrations(route_id);

CREATE TYPE delivery_status AS ENUM ('pending', 'processing', 'success', 'failed', 'dead');

CREATE TABLE integration_delivery_queue (
    id              BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    lead_id         BIGINT NOT NULL REFERENCES leads(id) ON DELETE CASCADE,
    connection_id   BIGINT NOT NULL REFERENCES integration_connections(id) ON DELETE CASCADE,
    route_id        BIGINT REFERENCES routes(id) ON DELETE SET NULL,
    payload         JSONB NOT NULL DEFAULT '{}',
    status          delivery_status NOT NULL DEFAULT 'pending',
    attempts        INT NOT NULL DEFAULT 0,
    max_attempts    INT NOT NULL DEFAULT 5,
    next_attempt_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_error      TEXT,
    external_id     TEXT,
    delivered_at    TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_delivery_queue_status ON integration_delivery_queue(status, next_attempt_at)
    WHERE status IN ('pending', 'processing');
CREATE INDEX idx_delivery_queue_lead ON integration_delivery_queue(lead_id);

CREATE TABLE integration_delivery_logs (
    id              BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    queue_item_id   BIGINT NOT NULL REFERENCES integration_delivery_queue(id) ON DELETE CASCADE,
    attempt_number  INT NOT NULL,
    status          TEXT NOT NULL,
    http_status     INT,
    request_body    JSONB,
    response_body   TEXT,
    duration_ms     INT,
    error           TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_delivery_logs_queue ON integration_delivery_logs(queue_item_id);

CREATE TABLE integration_oauth_tokens (
    id            BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    connection_id BIGINT NOT NULL REFERENCES integration_connections(id) ON DELETE CASCADE UNIQUE,
    access_token  BYTEA NOT NULL,
    refresh_token BYTEA,
    token_type    TEXT NOT NULL DEFAULT 'Bearer',
    expires_at    TIMESTAMPTZ,
    scope         TEXT,
    raw           BYTEA,
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);
