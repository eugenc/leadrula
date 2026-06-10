UPDATE transactions t
SET type = 'manual_invoice'
FROM invoices i
WHERE i.credit_txn_id = t.id
  AND t.type = 'credit';
