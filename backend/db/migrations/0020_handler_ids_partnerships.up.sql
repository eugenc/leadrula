CREATE OR REPLACE FUNCTION gen_handler_id(prefix text) RETURNS text AS $$
DECLARE
    chars text := '23456789ABCDEFGHJKMNPQRSTUVWXYZ';
    result text;
    i int;
    idx int;
BEGIN
    result := prefix || '-';
    FOR i IN 1..5 LOOP
        idx := floor(random() * length(chars) + 1)::int;
        result := result || substr(chars, idx, 1);
    END LOOP;
    RETURN result;
END;
$$ LANGUAGE plpgsql;

CREATE OR REPLACE FUNCTION assign_unique_handler_id(prefix text, table_name text, id_col text, row_id bigint) RETURNS void AS $$
DECLARE
    new_id text;
    attempts int := 0;
BEGIN
    LOOP
        new_id := gen_handler_id(prefix);
        BEGIN
            EXECUTE format('UPDATE %I SET handler_id = $1 WHERE %I = $2', table_name, id_col)
                USING new_id, row_id;
            RETURN;
        EXCEPTION WHEN unique_violation THEN
            attempts := attempts + 1;
            IF attempts > 20 THEN
                RAISE EXCEPTION 'failed to assign handler_id for %.%', table_name, row_id;
            END IF;
        END;
    END LOOP;
END;
$$ LANGUAGE plpgsql;

ALTER TABLE accounts ADD COLUMN handler_id TEXT;
ALTER TABLE contracts ADD COLUMN handler_id TEXT;

DO $$
DECLARE
    r record;
    pfx text;
BEGIN
    FOR r IN SELECT id, type FROM accounts LOOP
        IF r.type = 'publisher' THEN
            pfx := 'P';
        ELSE
            pfx := 'B';
        END IF;
        PERFORM assign_unique_handler_id(pfx, 'accounts', 'id', r.id);
    END LOOP;

    FOR r IN SELECT id FROM contracts LOOP
        PERFORM assign_unique_handler_id('C', 'contracts', 'id', r.id);
    END LOOP;
END $$;

ALTER TABLE accounts ALTER COLUMN handler_id SET NOT NULL;
ALTER TABLE contracts ALTER COLUMN handler_id SET NOT NULL;
CREATE UNIQUE INDEX idx_accounts_handler_id ON accounts(handler_id);
CREATE UNIQUE INDEX idx_contracts_handler_id ON contracts(handler_id);

CREATE TYPE partnership_status AS ENUM (
    'active',
    'pending_buyer',
    'pending_publisher',
    'rejected',
    'revoked'
);

CREATE TABLE partnerships (
    id                     BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    publisher_id           BIGINT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    buyer_id               BIGINT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    status                 partnership_status NOT NULL,
    requested_by           account_type NOT NULL,
    requested_by_user_id   BIGINT REFERENCES users(id) ON DELETE SET NULL,
    created_at             TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at             TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (publisher_id, buyer_id)
);
CREATE INDEX idx_partnerships_publisher ON partnerships(publisher_id);
CREATE INDEX idx_partnerships_buyer ON partnerships(buyer_id);

-- Active partnerships for existing contracts
INSERT INTO partnerships (publisher_id, buyer_id, status, requested_by)
SELECT publisher_id, buyer_id, 'active', 'publisher'
FROM contracts
WHERE deleted_at IS NULL
ON CONFLICT (publisher_id, buyer_id) DO NOTHING;

-- Active partnerships for buyers without contracts (keeps publisher buyer list working)
INSERT INTO partnerships (publisher_id, buyer_id, status, requested_by)
SELECT pub.id, a.id, 'active', 'publisher'
FROM accounts pub
CROSS JOIN accounts a
WHERE pub.type = 'publisher'
  AND a.type = 'buyer'
  AND NOT EXISTS (
      SELECT 1 FROM partnerships p
      WHERE p.publisher_id = pub.id AND p.buyer_id = a.id
  )
ON CONFLICT (publisher_id, buyer_id) DO NOTHING;

ALTER TYPE notification_type ADD VALUE IF NOT EXISTS 'partnership_request';
ALTER TYPE notification_type ADD VALUE IF NOT EXISTS 'partnership_accepted';

DROP FUNCTION IF EXISTS assign_unique_handler_id(text, text, text, bigint);
DROP FUNCTION IF EXISTS gen_handler_id(text);
