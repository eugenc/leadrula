package leads

// Buyer-facing status filters (presentation layer). Canonical l.status values unchanged.

const buyerStatusNewSQL = `(
	(l.status = 'review' AND l.stage_id IS NULL)
	OR (l.status = 'distributed' AND ` + stageMoveCountSQL + ` = 0)
	OR (l.status = 'review' AND l.stage_id IS NOT NULL AND ` + stageMoveCountSQL + ` <= 1)
)`

const buyerStatusActiveSQL = `(
	(l.status = 'distributed' AND ` + stageMoveCountSQL + ` >= 1)
	OR (l.status = 'review' AND l.stage_id IS NOT NULL AND ` + stageMoveCountSQL + ` >= 2)
)`
