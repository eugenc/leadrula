CREATE TYPE buyer_kind AS ENUM ('direct', 'marketplace');

ALTER TABLE accounts
  ADD COLUMN buyer_kind buyer_kind NOT NULL DEFAULT 'direct';
