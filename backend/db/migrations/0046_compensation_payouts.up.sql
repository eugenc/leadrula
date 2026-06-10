ALTER TABLE contract_compensations
  ADD COLUMN payout_frequency TEXT,
  ADD COLUMN payout_weekday INT,
  ADD COLUMN payout_month_day INT,
  ADD CONSTRAINT contract_compensations_payout_frequency_check
    CHECK (payout_frequency IS NULL OR payout_frequency IN ('daily', 'weekly', 'monthly')),
  ADD CONSTRAINT contract_compensations_payout_weekday_check
    CHECK (payout_weekday IS NULL OR (payout_weekday >= 0 AND payout_weekday <= 6)),
  ADD CONSTRAINT contract_compensations_payout_month_day_check
    CHECK (payout_month_day IS NULL OR (payout_month_day >= 1 AND payout_month_day <= 28));

CREATE TABLE compensation_earnings (
    id               BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    compensation_id  BIGINT NOT NULL REFERENCES contract_compensations(id) ON DELETE CASCADE,
    lead_id          BIGINT NOT NULL REFERENCES leads(id) ON DELETE CASCADE,
    amount           NUMERIC(14,2) NOT NULL,
    kind             TEXT NOT NULL,
    source_txn_id    BIGINT REFERENCES transactions(id),
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT compensation_earnings_kind_check
      CHECK (kind IN ('distribute', 'stage', 'return', 'dispute'))
);

CREATE UNIQUE INDEX idx_comp_earnings_distribute
  ON compensation_earnings(compensation_id, lead_id, kind)
  WHERE kind = 'distribute';

CREATE UNIQUE INDEX idx_comp_earnings_stage
  ON compensation_earnings(compensation_id, lead_id, kind)
  WHERE kind = 'stage';

CREATE UNIQUE INDEX idx_comp_earnings_dispute
  ON compensation_earnings(source_txn_id)
  WHERE kind = 'dispute';

CREATE INDEX idx_comp_earnings_comp ON compensation_earnings(compensation_id, created_at);

CREATE TABLE compensation_payout_clears (
    id               BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    compensation_id  BIGINT NOT NULL REFERENCES contract_compensations(id) ON DELETE CASCADE,
    period_start     TIMESTAMPTZ NOT NULL,
    period_end       TIMESTAMPTZ NOT NULL,
    amount           NUMERIC(14,2) NOT NULL,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (compensation_id, period_start, period_end),
    CONSTRAINT compensation_payout_clears_amount_nonneg CHECK (amount >= 0)
);

CREATE INDEX idx_comp_payout_clears_comp ON compensation_payout_clears(compensation_id);
