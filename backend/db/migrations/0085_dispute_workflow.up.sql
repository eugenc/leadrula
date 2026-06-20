-- Negotiated dispute workflow: publisher return disputes, Accept/Reject, deadlines,
-- attachments, and a disputed lead status that freezes routing side-effects.

-- New lead status used while a lead is under active dispute (set at runtime).
ALTER TYPE lead_status ADD VALUE IF NOT EXISTS 'disputed';

-- Notification events for the dispute workflow (used at runtime).
ALTER TYPE notification_type ADD VALUE IF NOT EXISTS 'lead_disputed';
ALTER TYPE notification_type ADD VALUE IF NOT EXISTS 'dispute_message';
ALTER TYPE notification_type ADD VALUE IF NOT EXISTS 'dispute_deadline';

-- Workflow columns on disputes. status stays 'open' during negotiation and
-- becomes 'accepted' (or 'rejected' when a buyer withdraws) when closed; the
-- outcome column records how it closed.
ALTER TABLE disputes
  ADD COLUMN initiated_by           TEXT NOT NULL DEFAULT 'buyer',
  ADD COLUMN lead_id                BIGINT REFERENCES leads(id),
  ADD COLUMN contract_id            BIGINT REFERENCES contracts(id),
  ADD COLUMN amount                 NUMERIC(14,2) NOT NULL DEFAULT 0,
  ADD COLUMN deadline_days          INT NOT NULL DEFAULT 7,
  ADD COLUMN response_deadline_at   TIMESTAMPTZ,
  ADD COLUMN awaiting_party         TEXT,
  ADD COLUMN outcome                TEXT,
  ADD COLUMN winner_party           TEXT,
  ADD COLUMN placement_party        TEXT,
  ADD COLUMN placement_pipeline_id  BIGINT REFERENCES pipelines(id),
  ADD COLUMN placement_stage_id     BIGINT REFERENCES pipeline_stages(id),
  ADD COLUMN placement_completed_at TIMESTAMPTZ;

CREATE INDEX idx_disputes_deadline ON disputes(response_deadline_at) WHERE status = 'open';

CREATE TABLE dispute_messages (
    id           BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    dispute_id   BIGINT NOT NULL REFERENCES disputes(id) ON DELETE CASCADE,
    user_id      BIGINT REFERENCES users(id),
    account_id   BIGINT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    author_party TEXT NOT NULL,
    kind         TEXT NOT NULL DEFAULT 'message',
    body         TEXT NOT NULL DEFAULT '',
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_dispute_messages_dispute ON dispute_messages(dispute_id, created_at);

CREATE TABLE dispute_message_attachments (
    id           BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    message_id   BIGINT NOT NULL REFERENCES dispute_messages(id) ON DELETE CASCADE,
    storage_key  TEXT NOT NULL,
    filename     TEXT NOT NULL,
    content_type TEXT NOT NULL DEFAULT '',
    byte_size    BIGINT NOT NULL DEFAULT 0,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_dispute_attachments_message ON dispute_message_attachments(message_id);
