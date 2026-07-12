-- In-app messaging: threads, messages, connect gate, broadcasts, audit mode,
-- internal team chat, and 2-year retention. Phase 2 external channels are
-- stubbed (tables only, no handlers).

-- ── enums ────────────────────────────────────────────────────
CREATE TYPE thread_type AS ENUM (
    'direct',       -- 1-on-1 between two external accounts
    'group',        -- multiple members (external accounts OR internal users)
    'internal',     -- 1-on-1 between two users on the same account
    'broadcast'     -- reserved
);

CREATE TYPE thread_context AS ENUM (
    'general',      -- no specific context
    'lead',         -- thread is about a specific lead
    'contract',     -- thread is about a specific contract
    'connect'       -- marketplace connect request thread
);

CREATE TYPE thread_status AS ENUM (
    'active',       -- normal messaging
    'pending',      -- connect request not yet accepted
    'archived',     -- archived by creator
    'blocked'       -- read-only; a member blocked the thread
);

-- Phase 2 external channel delivery status (reserved).
CREATE TYPE ext_delivery_status AS ENUM (
    'pending', 'sent', 'delivered', 'read', 'failed'
);

-- ── threads ──────────────────────────────────────────────────
CREATE TABLE threads (
    id                BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    public_id         UUID NOT NULL DEFAULT gen_random_uuid() UNIQUE,
    type              thread_type NOT NULL DEFAULT 'direct',
    context           thread_context NOT NULL DEFAULT 'general',
    status            thread_status NOT NULL DEFAULT 'active',
    title             TEXT,
    -- Internal threads are scoped to one account.
    account_id        BIGINT REFERENCES accounts(id) ON DELETE CASCADE,
    -- Optional context references.
    lead_id           BIGINT REFERENCES leads(id) ON DELETE SET NULL,
    contract_id       BIGINT REFERENCES contracts(id) ON DELETE SET NULL,
    -- Connect request tracking.
    initiator_id      BIGINT REFERENCES accounts(id),
    connect_requested_at TIMESTAMPTZ,
    connect_accepted_at  TIMESTAMPTZ,
    connect_accepted_by  BIGINT REFERENCES accounts(id),
    -- Block tracking (who put the thread read-only).
    blocked_by        BIGINT REFERENCES accounts(id),
    -- Audit.
    audit_mode        BOOLEAN NOT NULL DEFAULT false,
    audit_enabled_by  BIGINT REFERENCES users(id),
    audit_enabled_at  TIMESTAMPTZ,
    -- Retention.
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_message_at   TIMESTAMPTZ,
    purge_after       TIMESTAMPTZ
);
CREATE INDEX idx_threads_status ON threads(status);
CREATE INDEX idx_threads_last_message ON threads(last_message_at DESC NULLS LAST);
CREATE INDEX idx_threads_lead ON threads(lead_id) WHERE lead_id IS NOT NULL;
CREATE INDEX idx_threads_contract ON threads(contract_id) WHERE contract_id IS NOT NULL;
CREATE INDEX idx_threads_purge ON threads(purge_after) WHERE purge_after IS NOT NULL;

-- ── messages ─────────────────────────────────────────────────
CREATE TABLE messages (
    id              BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    public_id       UUID NOT NULL DEFAULT gen_random_uuid() UNIQUE,
    thread_id       BIGINT NOT NULL REFERENCES threads(id) ON DELETE CASCADE,
    sender_id       BIGINT NOT NULL REFERENCES accounts(id),
    sender_user_id  BIGINT REFERENCES users(id),
    body            TEXT,
    body_edited     TEXT,
    edited_at       TIMESTAMPTZ,
    deleted_at      TIMESTAMPTZ,
    type            TEXT NOT NULL DEFAULT 'text',
    -- 'text' | 'attachment' | 'lead_card' | 'system' | 'connect_request'
    lead_id         BIGINT REFERENCES leads(id) ON DELETE SET NULL,
    reply_to_id     BIGINT REFERENCES messages(id) ON DELETE SET NULL,
    -- Phase 2 external channel origin (reserved).
    external_channel     TEXT,
    external_message_id  TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    purge_after     TIMESTAMPTZ NOT NULL DEFAULT (now() + interval '2 years')
);
CREATE INDEX idx_messages_thread ON messages(thread_id, created_at DESC);
CREATE INDEX idx_messages_sender ON messages(sender_id);
CREATE INDEX idx_messages_purge ON messages(purge_after) WHERE deleted_at IS NULL;

-- ── thread members ───────────────────────────────────────────
CREATE TABLE thread_members (
    id              BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    thread_id       BIGINT NOT NULL REFERENCES threads(id) ON DELETE CASCADE,
    account_id      BIGINT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    -- Set for internal (user-scoped) membership; NULL for external account membership.
    user_id         BIGINT REFERENCES users(id) ON DELETE CASCADE,
    role            TEXT NOT NULL DEFAULT 'member', -- 'owner' | 'member'
    muted           BOOLEAN NOT NULL DEFAULT false,
    -- Group invite state: 'active' | 'pending' | 'declined'.
    invite_status   TEXT NOT NULL DEFAULT 'active',
    last_read_message_id BIGINT REFERENCES messages(id) ON DELETE SET NULL,
    last_read_at    TIMESTAMPTZ,
    archived_at     TIMESTAMPTZ,
    joined_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    left_at         TIMESTAMPTZ
);
-- External membership: unique per (thread, account).
CREATE UNIQUE INDEX uniq_thread_members_account
    ON thread_members(thread_id, account_id) WHERE user_id IS NULL;
-- Internal membership: unique per (thread, user).
CREATE UNIQUE INDEX uniq_thread_members_user
    ON thread_members(thread_id, user_id) WHERE user_id IS NOT NULL;
CREATE INDEX idx_thread_members_account ON thread_members(account_id);
CREATE INDEX idx_thread_members_user ON thread_members(user_id) WHERE user_id IS NOT NULL;
CREATE INDEX idx_thread_members_thread ON thread_members(thread_id);

-- ── attachments ──────────────────────────────────────────────
CREATE TABLE message_attachments (
    id              BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    public_id       UUID NOT NULL DEFAULT gen_random_uuid() UNIQUE,
    message_id      BIGINT NOT NULL REFERENCES messages(id) ON DELETE CASCADE,
    storage_key     TEXT NOT NULL,
    filename        TEXT NOT NULL,
    content_type    TEXT NOT NULL DEFAULT '',
    byte_size       BIGINT NOT NULL DEFAULT 0,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_message_attachments_message ON message_attachments(message_id);

-- ── connect requests (pair-level marketplace gate) ───────────
CREATE TABLE connect_requests (
    id              BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    public_id       UUID NOT NULL DEFAULT gen_random_uuid() UNIQUE,
    initiator_id    BIGINT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    recipient_id    BIGINT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    thread_id       BIGINT NOT NULL REFERENCES threads(id) ON DELETE CASCADE,
    status          TEXT NOT NULL DEFAULT 'pending',
    -- 'pending' | 'accepted' | 'declined' | 'blocked'
    message_preview TEXT,
    responded_at    TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (initiator_id, recipient_id)
);
CREATE INDEX idx_connect_requests_recipient ON connect_requests(recipient_id, status);
CREATE INDEX idx_connect_requests_initiator ON connect_requests(initiator_id, status);

-- ── broadcast jobs ───────────────────────────────────────────
CREATE TABLE broadcast_jobs (
    id              BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    public_id       UUID NOT NULL DEFAULT gen_random_uuid() UNIQUE,
    sender_id       BIGINT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    sender_user_id  BIGINT REFERENCES users(id) ON DELETE SET NULL,
    target_type     TEXT NOT NULL DEFAULT 'all_connections',
    body            TEXT NOT NULL,
    status          TEXT NOT NULL DEFAULT 'pending',
    total_count     INT NOT NULL DEFAULT 0,
    sent_count      INT NOT NULL DEFAULT 0,
    failed_count    INT NOT NULL DEFAULT 0,
    started_at      TIMESTAMPTZ,
    completed_at    TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_broadcast_jobs_sender ON broadcast_jobs(sender_id, created_at DESC);

-- ── phase 2 stubs (no handlers yet) ──────────────────────────
CREATE TABLE channel_connections (
    id              BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    user_id         BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    account_id      BIGINT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    channel         TEXT NOT NULL,
    external_id     TEXT,
    verified        BOOLEAN NOT NULL DEFAULT false,
    active          BOOLEAN NOT NULL DEFAULT true,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (user_id, channel)
);

CREATE TABLE thread_channel_routing (
    id              BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    thread_id       BIGINT NOT NULL REFERENCES threads(id) ON DELETE CASCADE,
    member_id       BIGINT NOT NULL REFERENCES thread_members(id) ON DELETE CASCADE,
    connection_id   BIGINT NOT NULL REFERENCES channel_connections(id) ON DELETE CASCADE,
    channel         TEXT NOT NULL,
    last_delivered_at TIMESTAMPTZ,
    UNIQUE (thread_id, member_id)
);

-- ── triggers ─────────────────────────────────────────────────
CREATE OR REPLACE FUNCTION set_thread_purge() RETURNS TRIGGER AS $$
BEGIN
    NEW.purge_after := NEW.created_at + interval '2 years';
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_thread_purge
    BEFORE INSERT ON threads
    FOR EACH ROW EXECUTE FUNCTION set_thread_purge();

CREATE OR REPLACE FUNCTION update_thread_last_message() RETURNS TRIGGER AS $$
BEGIN
    UPDATE threads SET last_message_at = NEW.created_at, updated_at = now()
    WHERE id = NEW.thread_id;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_last_message
    AFTER INSERT ON messages
    FOR EACH ROW EXECUTE FUNCTION update_thread_last_message();

-- ── notification event ───────────────────────────────────────
ALTER TYPE notification_type ADD VALUE IF NOT EXISTS 'message_received';
