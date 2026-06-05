package contracts

import (
	"context"
	"time"

	"github.com/echayko/leadrula/backend/internal/database"
	"github.com/echayko/leadrula/backend/pkg/httpx"
)

const distributedLeadSQL = `t.contract_id = $1 AND t.type = 'debit' AND t.lead_id IS NOT NULL
  AND (t.description LIKE 'lead routed:%'
       OR t.description = 'lead routed from intake queue'
       OR t.description = 'lead re-distributed')`

// CheckCap rejects routing when contract or compensation volume caps are exceeded.
func CheckCap(ctx context.Context, q database.Querier, contractID int64, compensationID int64) error {
	compID := compensationID
	if compID == 0 {
		t, err := loadPerLeadTarget(ctx, q, contractID)
		if err != nil {
			return err
		}
		compID = t.CompensationID
	}
	if compID == 0 {
		return checkLegacyContractCap(ctx, q, contractID)
	}

	var capPeriod, tz string
	var capTotal, capMaxDaily *int
	err := q.QueryRow(ctx,
		`SELECT cc.cap_period, cc.cap_total, cc.cap_max_daily, COALESCE(NULLIF(a.timezone, ''), 'UTC')
		 FROM contract_compensations cc
		 JOIN contracts c ON c.id = cc.contract_id
		 JOIN accounts a ON a.id = c.publisher_id
		 WHERE cc.id = $1 AND c.id = $2 AND c.deleted_at IS NULL`,
		compID, contractID).Scan(&capPeriod, &capTotal, &capMaxDaily, &tz)
	if err != nil {
		return checkLegacyContractCap(ctx, q, contractID)
	}
	if capTotal == nil && capMaxDaily == nil {
		return nil
	}

	loc := time.UTC
	if l, err := time.LoadLocation(tz); err == nil {
		loc = l
	}
	now := time.Now().In(loc)

	if capTotal != nil {
		var count int
		qry := `SELECT COUNT(DISTINCT t.lead_id) FROM transactions t WHERE ` + distributedLeadSQL
		args := []any{contractID}
		if since := capPeriodStart(capPeriod, now); !since.IsZero() {
			qry += ` AND t.created_at >= $2`
			args = append(args, since)
		}
		if err := q.QueryRow(ctx, qry, args...).Scan(&count); err != nil {
			return err
		}
		if count >= *capTotal {
			return httpx.BusinessRule("contract cap reached")
		}
	}

	if capPeriodAllowsDailyCap(capPeriod) && capMaxDaily != nil {
		todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc)
		var daily int
		if err := q.QueryRow(ctx,
			`SELECT COUNT(DISTINCT t.lead_id) FROM transactions t
			 WHERE `+distributedLeadSQL+` AND t.created_at >= $2`,
			contractID, todayStart).Scan(&daily); err != nil {
			return err
		}
		if daily >= *capMaxDaily {
			return httpx.BusinessRule("contract daily cap reached")
		}
	}
	return nil
}

func checkLegacyContractCap(ctx context.Context, q database.Querier, contractID int64) error {
	var capPeriod, tz string
	var capTotal, capMaxDaily *int
	err := q.QueryRow(ctx,
		`SELECT c.cap_period, c.cap_total, c.cap_max_daily, COALESCE(NULLIF(a.timezone, ''), 'UTC')
		 FROM contracts c JOIN accounts a ON a.id = c.publisher_id
		 WHERE c.id = $1 AND c.deleted_at IS NULL`,
		contractID).Scan(&capPeriod, &capTotal, &capMaxDaily, &tz)
	if err != nil {
		return err
	}
	if capTotal == nil && capMaxDaily == nil {
		return nil
	}

	loc := time.UTC
	if l, err := time.LoadLocation(tz); err == nil {
		loc = l
	}
	now := time.Now().In(loc)

	if capTotal != nil {
		var count int
		qry := `SELECT COUNT(DISTINCT t.lead_id) FROM transactions t WHERE ` + distributedLeadSQL
		args := []any{contractID}
		if since := capPeriodStart(capPeriod, now); !since.IsZero() {
			qry += ` AND t.created_at >= $2`
			args = append(args, since)
		}
		if err := q.QueryRow(ctx, qry, args...).Scan(&count); err != nil {
			return err
		}
		if count >= *capTotal {
			return httpx.BusinessRule("contract cap reached")
		}
	}

	if capPeriodAllowsDailyCap(capPeriod) && capMaxDaily != nil {
		todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc)
		var daily int
		if err := q.QueryRow(ctx,
			`SELECT COUNT(DISTINCT t.lead_id) FROM transactions t
			 WHERE `+distributedLeadSQL+` AND t.created_at >= $2`,
			contractID, todayStart).Scan(&daily); err != nil {
			return err
		}
		if daily >= *capMaxDaily {
			return httpx.BusinessRule("contract daily cap reached")
		}
	}
	return nil
}

func capPeriodAllowsDailyCap(period string) bool {
	return period == "weekly" || period == "monthly"
}

func capPeriodStart(period string, now time.Time) time.Time {
	switch period {
	case "weekly":
		return isoWeekStart(now)
	case "monthly":
		return time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	default:
		return time.Time{}
	}
}

func isoWeekStart(t time.Time) time.Time {
	weekday := int(t.Weekday())
	if weekday == 0 {
		weekday = 7
	}
	d := t.AddDate(0, 0, -(weekday - 1))
	return time.Date(d.Year(), d.Month(), d.Day(), 0, 0, 0, 0, t.Location())
}
