CREATE TYPE invoice_status AS ENUM ('open', 'paid', 'void');
CREATE TYPE invoice_payment_method AS ENUM (
    'stripe', 'bank_transfer', 'check', 'cash', 'other_digital', 'other'
);

CREATE TABLE invoices (
    id                      BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    public_id               UUID NOT NULL DEFAULT gen_random_uuid() UNIQUE,
    publisher_id            BIGINT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    buyer_id                BIGINT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    amount                  NUMERIC(14,2) NOT NULL CHECK (amount > 0),
    description             TEXT NOT NULL DEFAULT '',
    kind                    TEXT NOT NULL CHECK (kind IN ('starting_balance', 'prepay_request')),
    status                  invoice_status NOT NULL DEFAULT 'open',
    payment_method          invoice_payment_method,
    payment_note            TEXT,
    stripe_payment_intent_id TEXT,
    credit_txn_id           BIGINT REFERENCES transactions(id),
    paid_at                 TIMESTAMPTZ,
    paid_by                 BIGINT REFERENCES users(id),
    created_at              TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_invoices_publisher_status ON invoices(publisher_id, status, created_at DESC);
CREATE INDEX idx_invoices_buyer_status ON invoices(buyer_id, status, created_at DESC);
CREATE UNIQUE INDEX idx_invoices_stripe_pi ON invoices(stripe_payment_intent_id)
    WHERE stripe_payment_intent_id IS NOT NULL;
