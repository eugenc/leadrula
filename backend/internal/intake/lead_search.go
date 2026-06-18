package intake

import (
	"fmt"
	"strings"
)

// logLeadFilterClause returns a WHERE suffix and args for filtering logs by lead.
// startArg is the first $N placeholder index. leadIDCol is the qualified lead id column (e.g. "d.lead_id").
func logLeadFilterClause(startArg int, leadID int64, search string, leadIDCol string) (string, []any) {
	if leadID > 0 {
		return fmt.Sprintf(" AND %s = $%d", leadIDCol, startArg), []any{leadID}
	}
	search = strings.TrimSpace(search)
	if search == "" {
		return "", nil
	}
	like := "%" + search + "%"
	n := startArg
	clause := fmt.Sprintf(` AND (
		l.first_name ILIKE $%d OR
		l.last_name ILIKE $%d OR
		TRIM(CONCAT(l.first_name, ' ', l.last_name)) ILIKE $%d OR
		l.email ILIKE $%d OR
		l.phone ILIKE $%d OR
		l.public_id::text ILIKE $%d
	)`, n, n, n, n, n, n)
	return clause, []any{like}
}

func appendLogLeadFilter(where string, args []any, leadID int64, search string, leadIDCol string) (string, []any) {
	startArg := len(args) + 1
	clause, extra := logLeadFilterClause(startArg, leadID, search, leadIDCol)
	if clause == "" {
		return where, args
	}
	return where + clause, append(args, extra...)
}
