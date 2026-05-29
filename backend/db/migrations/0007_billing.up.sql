CREATE TYPE txn_type AS ENUM ('debit','credit','dispute_credit','manual_invoice');
CREATE TYPE dispute_status AS ENUM ('open','accepted','rejected');

CREATE TABLE buyer_balances (
    buyer_id   BIGINT PRIMARY KEY REFERENCES accounts(id) ON DELETE CASCADE,
    balance    NUMERIC(14,2) NOT NULL DEFAULT 0,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- deal_id removed: there is no deals table in v2 (the lead is the card)
CREATE TABLE transactions (
    id            BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    public_id     UUID NOT NULL DEFAULT gen_random_uuid() UNIQUE,
    buyer_id      BIGINT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    lead_id       BIGINT REFERENCES leads(id),
    contract_id   BIGINT REFERENCES contracts(id),
    type          txn_type NOT NULL,
    amount        NUMERIC(14,2) NOT NULL,
    balance_after NUMERIC(14,2) NOT NULL,
    description   TEXT NOT NULL DEFAULT '',
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_txn_buyer ON transactions(buyer_id, created_at DESC);

CREATE TABLE disputes (
    id             BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    transaction_id BIGINT NOT NULL REFERENCES transactions(id),
    buyer_id       BIGINT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    reason         TEXT NOT NULL,
    status         dispute_status NOT NULL DEFAULT 'open',
    resolved_by    BIGINT REFERENCES users(id),
    resolved_at    TIMESTAMPTZ,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_disputes_status ON disputes(status);
