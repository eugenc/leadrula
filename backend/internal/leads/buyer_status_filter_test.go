package leads

import (
	"strings"
	"testing"
)

func TestBuyerStatusFilterSQL_containsStageMoveCount(t *testing.T) {
	for _, sql := range []string{buyerStatusNewSQL, buyerStatusActiveSQL} {
		if !strings.Contains(sql, stageMoveCountSQL) {
			t.Fatalf("expected stage move subquery in %q", sql)
		}
	}
	if !strings.Contains(buyerStatusNewSQL, "review") || !strings.Contains(buyerStatusNewSQL, "distributed") {
		t.Fatalf("buyerStatusNewSQL: %q", buyerStatusNewSQL)
	}
}
