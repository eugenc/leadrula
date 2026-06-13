ALTER TABLE webhooks
  ADD COLUMN integration_connection_id BIGINT
  REFERENCES integration_connections(id) ON DELETE SET NULL;

CREATE INDEX idx_webhooks_integration_connection
  ON webhooks(integration_connection_id)
  WHERE integration_connection_id IS NOT NULL;

UPDATE webhooks w SET integration_connection_id = ic.id
FROM integration_connections ic
JOIN integration_providers ip ON ip.id = ic.provider_id
WHERE ip.slug = 'sunbase'
  AND w.id IN (
    NULLIF(ic.config->'webhook_ids'->>'inbound', '')::bigint,
    NULLIF(ic.config->'webhook_ids'->>'outbound_post', '')::bigint,
    NULLIF(ic.config->'webhook_ids'->>'outbound_get', '')::bigint
  );
