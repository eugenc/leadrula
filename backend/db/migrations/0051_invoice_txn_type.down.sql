UPDATE transactions t
SET type = 'credit'
FROM invoices i
WHERE i.credit_txn_id = t.id
  AND t.type = 'manual_invoice';
