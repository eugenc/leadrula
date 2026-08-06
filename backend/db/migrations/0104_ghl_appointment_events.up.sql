CREATE TABLE lead_integration_appointment_events (
    lead_id         BIGINT NOT NULL REFERENCES leads(id) ON DELETE CASCADE,
    connection_id   BIGINT NOT NULL REFERENCES integration_connections(id) ON DELETE CASCADE,
    event_id        TEXT NOT NULL,
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (lead_id, connection_id)
);

CREATE INDEX lead_integration_appointment_events_connection_idx
    ON lead_integration_appointment_events(connection_id);
