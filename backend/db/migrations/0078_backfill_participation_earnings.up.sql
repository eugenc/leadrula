-- Backfill compensation_earnings for open-offer / participation routes.
-- Migration 0077 only matched contract_id; participation comps use participation_id.
-- Safe to re-run: guarded by NOT EXISTS and partial unique indexes.

INSERT INTO compensation_earnings (compensation_id, lead_id, amount, kind)
SELECT cc.id, t.lead_id, ABS(t.amount), 'distribute'
FROM transactions t
JOIN contracts c ON c.id = t.contract_id AND c.deleted_at IS NULL
JOIN LATERAL (
  SELECT cc2.id
  FROM contract_compensations cc2
  WHERE cc2.trigger = 'per_lead'
    AND (
      cc2.contract_id = c.id
      OR cc2.participation_id IN (
        SELECT p.id FROM contract_participations p
        WHERE p.contract_id = c.id AND p.buyer_id = t.buyer_id AND p.status = 'active'
      )
    )
  ORDER BY CASE WHEN cc2.kind = 'flat_rate' THEN 0 ELSE 1 END, cc2.position, cc2.id
  LIMIT 1
) cc ON true
WHERE t.type = 'debit'
  AND t.amount < 0
  AND t.lead_id IS NOT NULL
  AND NOT EXISTS (
    SELECT 1 FROM compensation_earnings ce
    WHERE ce.compensation_id = cc.id
      AND ce.lead_id = t.lead_id
      AND ce.kind = 'distribute'
  );

INSERT INTO compensation_earnings (compensation_id, lead_id, amount, kind)
SELECT cc.id, t.lead_id, -ABS(t.amount), 'return'
FROM transactions t
JOIN contracts c ON c.id = t.contract_id AND c.deleted_at IS NULL
JOIN LATERAL (
  SELECT cc2.id
  FROM contract_compensations cc2
  WHERE cc2.trigger = 'per_lead'
    AND (
      cc2.contract_id = c.id
      OR cc2.participation_id IN (
        SELECT p.id FROM contract_participations p
        WHERE p.contract_id = c.id AND p.buyer_id = t.buyer_id AND p.status = 'active'
      )
    )
  ORDER BY CASE WHEN cc2.kind = 'flat_rate' THEN 0 ELSE 1 END, cc2.position, cc2.id
  LIMIT 1
) cc ON true
WHERE t.type = 'credit'
  AND t.description = 'lead returned'
  AND t.lead_id IS NOT NULL
  AND NOT EXISTS (
    SELECT 1 FROM compensation_earnings ce
    WHERE ce.compensation_id = cc.id
      AND ce.lead_id = t.lead_id
      AND ce.kind = 'return'
  );

INSERT INTO compensation_earnings (compensation_id, lead_id, amount, kind, source_txn_id)
SELECT cc.id, orig.lead_id, -LEAST(ABS(orig.amount), ABS(t.amount)), 'dispute', orig.id
FROM transactions t
JOIN disputes d ON d.buyer_id = t.buyer_id AND d.status = 'accepted'
JOIN transactions orig ON orig.id = d.transaction_id AND orig.type = 'debit'
JOIN contracts c ON c.id = orig.contract_id AND c.deleted_at IS NULL
JOIN LATERAL (
  SELECT cc2.id
  FROM contract_compensations cc2
  WHERE cc2.trigger = 'per_lead'
    AND (
      cc2.contract_id = c.id
      OR cc2.participation_id IN (
        SELECT p.id FROM contract_participations p
        WHERE p.contract_id = c.id AND p.buyer_id = t.buyer_id AND p.status = 'active'
      )
    )
  ORDER BY CASE WHEN cc2.kind = 'flat_rate' THEN 0 ELSE 1 END, cc2.position, cc2.id
  LIMIT 1
) cc ON true
WHERE t.type = 'dispute_credit'
  AND orig.lead_id IS NOT NULL
  AND NOT EXISTS (
    SELECT 1 FROM compensation_earnings ce
    WHERE ce.source_txn_id = orig.id AND ce.kind = 'dispute'
  );
