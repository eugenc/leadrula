package contracts

import (
	"context"
	"errors"
	"log"
	"math"
	"time"

	"github.com/echayko/leadrula/backend/internal/database"
	"github.com/echayko/leadrula/backend/pkg/httpx"
	"github.com/jackc/pgx/v5"
)

// RecordEarningDistribute credits publisher hold when a lead is distributed.
func RecordEarningDistribute(ctx context.Context, q database.Querier, compensationID, leadID int64, amount float64, costBasis *float64) error {
	if compensationID == 0 || amount <= 0 {
		return nil
	}
	_, err := q.Exec(ctx,
		`INSERT INTO compensation_earnings(compensation_id, lead_id, amount, kind, cost_basis)
		 VALUES ($1,$2,$3,'distribute',$4)
		 ON CONFLICT DO NOTHING`,
		compensationID, leadID, amount, costBasis)
	return err
}

// RecordEarningStage credits publisher hold when rev/profit share triggers.
func RecordEarningStage(ctx context.Context, q database.Querier, compensationID, leadID int64, amount float64) error {
	if compensationID == 0 || amount <= 0 {
		return nil
	}
	_, err := q.Exec(ctx,
		`INSERT INTO compensation_earnings(compensation_id, lead_id, amount, kind)
		 VALUES ($1,$2,$3,'stage')
		 ON CONFLICT DO NOTHING`,
		compensationID, leadID, amount)
	return err
}

// RecordEarningReturn reverses prior earnings when a lead is returned to the publisher.
func RecordEarningReturn(ctx context.Context, q database.Querier, leadID int64, contractID *int64) error {
	if contractID == nil || *contractID == 0 {
		return nil
	}
	rows, err := q.Query(ctx,
		`SELECT ce.compensation_id, ce.amount
		 FROM compensation_earnings ce
		 JOIN contract_compensations cc ON cc.id = ce.compensation_id
		 WHERE ce.lead_id = $1 AND cc.contract_id = $2
		   AND ce.kind IN ('distribute', 'stage') AND ce.amount > 0`,
		leadID, *contractID)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var compID int64
		var amount float64
		if err := rows.Scan(&compID, &amount); err != nil {
			return err
		}
		if _, err := q.Exec(ctx,
			`INSERT INTO compensation_earnings(compensation_id, lead_id, amount, kind)
			 VALUES ($1,$2,$3,'return')`,
			compID, leadID, -amount); err != nil {
			return err
		}
	}
	return rows.Err()
}

// RecordEarningDispute reverses distribute earnings when a buyer dispute is accepted.
func RecordEarningDispute(ctx context.Context, q database.Querier, txnID int64, leadID *int64, contractID *int64, amount float64) error {
	if leadID == nil || contractID == nil || *leadID == 0 || *contractID == 0 || amount <= 0 {
		return nil
	}
	rows, err := q.Query(ctx,
		`SELECT ce.compensation_id, ce.amount
		 FROM compensation_earnings ce
		 JOIN contract_compensations cc ON cc.id = ce.compensation_id
		 WHERE ce.lead_id = $1 AND cc.contract_id = $2
		   AND ce.kind = 'distribute' AND ce.amount > 0`,
		*leadID, *contractID)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var compID int64
		var earned float64
		if err := rows.Scan(&compID, &earned); err != nil {
			return err
		}
		reversal := math.Min(earned, amount)
		if _, err := q.Exec(ctx,
			`INSERT INTO compensation_earnings(compensation_id, lead_id, amount, kind, source_txn_id)
			 VALUES ($1,$2,$3,'dispute',$4)
			 ON CONFLICT DO NOTHING`,
			compID, *leadID, -reversal, txnID); err != nil {
			return err
		}
	}
	return rows.Err()
}

// PayoutTransferExecutor sends cleared marketplace earnings via Stripe Connect.
type PayoutTransferExecutor interface {
	ExecuteMarketplacePayoutTransfers(ctx context.Context, publisherID int64) error
}

type PayoutSummary struct {
	Hold              float64 `json:"hold"`
	Cleared           float64 `json:"cleared"`
	PrepayBalance     float64 `json:"prepay_balance"`
	DistributedValue  float64 `json:"distributed_value"`
	ReturnedValue     float64 `json:"returned_value"`
	ClearedFromPrepay float64 `json:"cleared_from_prepay"`
}

type CompensationPayoutRow struct {
	CompensationID        int64   `json:"compensation_id"`
	ContractID            int64   `json:"contract_id"`
	ContractName          string  `json:"contract_name"`
	Kind                  string  `json:"kind"`
	BuyerKind             string  `json:"buyer_kind"`
	PayoutFrequency       *string `json:"payout_frequency,omitempty"`
	PayoutWeekday         *int    `json:"payout_weekday,omitempty"`
	PayoutMonthDay        *int    `json:"payout_month_day,omitempty"`
	Hold                  float64 `json:"hold"`
	Cleared               float64 `json:"cleared"`
	NextPeriodEnd         *string `json:"next_period_end,omitempty"`
	LatestTransferStatus  *string `json:"latest_transfer_status,omitempty"`
}

func (s *Service) runPayoutTransfers(ctx context.Context, publisherID int64) {
	if s.payoutTransfers == nil {
		return
	}
	if err := s.payoutTransfers.ExecuteMarketplacePayoutTransfers(ctx, publisherID); err != nil {
		log.Printf("contracts: marketplace payout transfers: %v", err)
	}
}

func (s *Service) PayoutSummary(ctx context.Context, publisherID int64) (*PayoutSummary, error) {
	if err := s.EnsurePublisherPayoutClears(ctx, publisherID); err != nil {
		return nil, err
	}
	s.runPayoutTransfers(ctx, publisherID)
	sum := &PayoutSummary{}
	if err := s.pool.QueryRow(ctx,
		`SELECT COALESCE(SUM(GREATEST(earned - cleared, 0)), 0),
		        COALESCE(SUM(cleared), 0)
		 FROM (
		   SELECT cc.id,
		     COALESCE((SELECT SUM(amount::float8) FROM compensation_earnings ce WHERE ce.compensation_id = cc.id), 0) AS earned,
		     COALESCE((SELECT SUM(amount::float8) FROM compensation_payout_clears pc WHERE pc.compensation_id = cc.id), 0) AS cleared
		   FROM contract_compensations cc
		   JOIN contracts c ON c.id = cc.contract_id
		   WHERE c.publisher_id = $1 AND c.deleted_at IS NULL
		 ) x`,
		publisherID).Scan(&sum.Hold, &sum.Cleared); err != nil {
		return nil, err
	}
	if err := s.pool.QueryRow(ctx,
		`SELECT COALESCE(SUM(bb.balance::float8), 0)
		 FROM buyer_balances bb
		 JOIN contracts c ON c.buyer_id = bb.buyer_id
		 WHERE c.publisher_id = $1 AND c.deleted_at IS NULL`,
		publisherID).Scan(&sum.PrepayBalance); err != nil {
		return nil, err
	}
	if err := s.pool.QueryRow(ctx,
		`SELECT COALESCE(SUM(ce.amount::float8), 0)
		 FROM compensation_earnings ce
		 JOIN contract_compensations cc ON cc.id = ce.compensation_id
		 JOIN contracts c ON c.id = cc.contract_id
		 WHERE c.publisher_id = $1 AND ce.kind = 'distribute' AND ce.amount > 0`,
		publisherID).Scan(&sum.DistributedValue); err != nil {
		return nil, err
	}
	if err := s.pool.QueryRow(ctx,
		`SELECT COALESCE(SUM(ABS(ce.amount::float8)), 0)
		 FROM compensation_earnings ce
		 JOIN contract_compensations cc ON cc.id = ce.compensation_id
		 JOIN contracts c ON c.id = cc.contract_id
		 WHERE c.publisher_id = $1 AND ce.kind IN ('return', 'dispute')`,
		publisherID).Scan(&sum.ReturnedValue); err != nil {
		return nil, err
	}
	sum.ClearedFromPrepay = sum.Cleared
	return sum, nil
}

func (s *Service) PayoutByCompensation(ctx context.Context, publisherID int64) ([]CompensationPayoutRow, error) {
	if err := s.EnsurePublisherPayoutClears(ctx, publisherID); err != nil {
		return nil, err
	}
	s.runPayoutTransfers(ctx, publisherID)
	rows, err := s.pool.Query(ctx,
		`SELECT cc.id, cc.contract_id, c.name, cc.kind, buyer.buyer_kind::text,
		        cc.payout_frequency, cc.payout_weekday, cc.payout_month_day,
		        COALESCE((SELECT SUM(amount::float8) FROM compensation_earnings ce WHERE ce.compensation_id = cc.id), 0),
		        COALESCE((SELECT SUM(amount::float8) FROM compensation_payout_clears pc WHERE pc.compensation_id = cc.id), 0),
		        a.timezone,
		        (SELECT pc.stripe_transfer_status
		         FROM compensation_payout_clears pc
		         WHERE pc.compensation_id = cc.id
		         ORDER BY pc.period_end DESC
		         LIMIT 1)
		 FROM contract_compensations cc
		 JOIN contracts c ON c.id = cc.contract_id
		 JOIN accounts a ON a.id = c.publisher_id
		 JOIN accounts buyer ON buyer.id = c.buyer_id
		 WHERE c.publisher_id = $1 AND c.deleted_at IS NULL
		 ORDER BY c.name, cc.position, cc.id`, publisherID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []CompensationPayoutRow
	now := time.Now()
	for rows.Next() {
		var row CompensationPayoutRow
		var earned, cleared float64
		var tz string
		var transferStatus *string
		if err := rows.Scan(
			&row.CompensationID, &row.ContractID, &row.ContractName, &row.Kind, &row.BuyerKind,
			&row.PayoutFrequency, &row.PayoutWeekday, &row.PayoutMonthDay,
			&earned, &cleared, &tz, &transferStatus,
		); err != nil {
			return nil, err
		}
		row.Hold = math.Max(0, earned-cleared)
		row.Cleared = cleared
		row.LatestTransferStatus = transferStatus
		if row.PayoutFrequency != nil {
			loc := loadLocation(tz)
			_, end := currentPayoutPeriod(now, *row.PayoutFrequency, row.PayoutWeekday, row.PayoutMonthDay, loc)
			endStr := end.UTC().Format(time.RFC3339)
			row.NextPeriodEnd = &endStr
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

func (s *Service) EnsurePublisherPayoutClears(ctx context.Context, publisherID int64) error {
	rows, err := s.pool.Query(ctx,
		`SELECT cc.id, cc.payout_frequency, cc.payout_weekday, cc.payout_month_day, a.timezone
		 FROM contract_compensations cc
		 JOIN contracts c ON c.id = cc.contract_id
		 JOIN accounts a ON a.id = c.publisher_id
		 WHERE c.publisher_id = $1 AND c.deleted_at IS NULL AND cc.payout_frequency IS NOT NULL`,
		publisherID)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var compID int64
		var freq string
		var weekday, monthDay *int
		var tz string
		if err := rows.Scan(&compID, &freq, &weekday, &monthDay, &tz); err != nil {
			return err
		}
		if err := ensurePayoutClears(ctx, s.pool, compID, freq, weekday, monthDay, loadLocation(tz)); err != nil {
			return err
		}
	}
	return rows.Err()
}

func ensurePayoutClears(ctx context.Context, q database.Querier, compID int64, freq string, weekday, monthDay *int, loc *time.Location) error {
	now := time.Now()
	var lastEnd *time.Time
	err := q.QueryRow(ctx,
		`SELECT period_end FROM compensation_payout_clears
		 WHERE compensation_id = $1 ORDER BY period_end DESC LIMIT 1`, compID).Scan(&lastEnd)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return err
	}
	var cursor time.Time
	if lastEnd != nil {
		cursor = lastEnd.In(loc)
	} else {
		var createdAt time.Time
		if err := q.QueryRow(ctx,
			`SELECT created_at FROM contract_compensations WHERE id = $1`, compID).Scan(&createdAt); err != nil {
			return err
		}
		start, _ := currentPayoutPeriod(createdAt, freq, weekday, monthDay, loc)
		cursor = start
	}
	wd := 0
	if weekday != nil {
		wd = *weekday
	}
	md := 1
	if monthDay != nil {
		md = *monthDay
	}
	for {
		start, end := payoutPeriodAt(cursor, freq, wd, md, loc)
		if !end.After(start) {
			break
		}
		if end.After(now) {
			break
		}
		var net float64
		if err := q.QueryRow(ctx,
			`SELECT COALESCE(SUM(amount::float8), 0)
			 FROM compensation_earnings
			 WHERE compensation_id = $1 AND created_at >= $2 AND created_at < $3`,
			compID, start.UTC(), end.UTC()).Scan(&net); err != nil {
			return err
		}
		if net > 0 {
			if _, err := q.Exec(ctx,
				`INSERT INTO compensation_payout_clears(compensation_id, period_start, period_end, amount)
				 VALUES ($1,$2,$3,$4)
				 ON CONFLICT (compensation_id, period_start, period_end) DO NOTHING`,
				compID, start.UTC(), end.UTC(), net); err != nil {
				return err
			}
		}
		cursor = end
	}
	return nil
}

func loadLocation(tz string) *time.Location {
	if tz == "" {
		tz = "UTC"
	}
	loc, err := time.LoadLocation(tz)
	if err != nil {
		return time.UTC
	}
	return loc
}

func currentPayoutPeriod(now time.Time, freq string, weekday, monthDay *int, loc *time.Location) (start, end time.Time) {
	wd := 0
	if weekday != nil {
		wd = *weekday
	}
	md := 1
	if monthDay != nil {
		md = *monthDay
	}
	return payoutPeriodContaining(now.In(loc), freq, wd, md, loc)
}

func payoutPeriodContaining(t time.Time, freq string, weekday, monthDay int, loc *time.Location) (start, end time.Time) {
	switch freq {
	case "daily":
		start = startOfDay(t, loc)
		end = start.Add(24 * time.Hour)
	case "weekly":
		start = startOfWeekOn(t, weekday, loc)
		end = start.AddDate(0, 0, 7)
	case "monthly":
		start = startOfMonthOn(t, monthDay, loc)
		end = nextMonthPeriodStart(start, monthDay, loc)
	default:
		start = startOfDay(t, loc)
		end = start.Add(24 * time.Hour)
	}
	return start, end
}

func payoutPeriodAt(cursor time.Time, freq string, weekday, monthDay int, loc *time.Location) (start, end time.Time) {
	t := cursor.In(loc)
	switch freq {
	case "daily":
		start = startOfDay(t, loc)
		end = start.Add(24 * time.Hour)
	case "weekly":
		start = startOfWeekOn(t, weekday, loc)
		end = start.AddDate(0, 0, 7)
	case "monthly":
		start = startOfMonthOn(t, monthDay, loc)
		end = nextMonthPeriodStart(start, monthDay, loc)
	default:
		start = startOfDay(t, loc)
		end = start.Add(24 * time.Hour)
	}
	return start, end
}

func startOfDay(t time.Time, loc *time.Location) time.Time {
	y, m, d := t.In(loc).Date()
	return time.Date(y, m, d, 0, 0, 0, 0, loc)
}

func startOfWeekOn(t time.Time, weekday int, loc *time.Location) time.Time {
	d := startOfDay(t, loc)
	for int(d.Weekday()) != weekday {
		d = d.AddDate(0, 0, -1)
	}
	return d
}

func startOfMonthOn(t time.Time, day int, loc *time.Location) time.Time {
	y, m, _ := t.In(loc).Date()
	start := time.Date(y, m, day, 0, 0, 0, 0, loc)
	if t.Before(start) {
		if m == time.January {
			start = time.Date(y-1, time.December, day, 0, 0, 0, 0, loc)
		} else {
			start = time.Date(y, m-1, day, 0, 0, 0, 0, loc)
		}
	}
	return start
}

func nextMonthPeriodStart(start time.Time, day int, loc *time.Location) time.Time {
	y, m, _ := start.Date()
	if m == time.December {
		return time.Date(y+1, time.January, day, 0, 0, 0, 0, loc)
	}
	return time.Date(y, m+1, day, 0, 0, 0, 0, loc)
}

var allowedPayoutFrequencies = map[string]bool{
	"daily": true, "weekly": true, "monthly": true,
}

func validatePayoutParams(freq *string, weekday, monthDay *int) error {
	if freq == nil || *freq == "" {
		return nil
	}
	f := *freq
	if !allowedPayoutFrequencies[f] {
		return httpx.Validation("payout_frequency must be daily, weekly, or monthly")
	}
	switch f {
	case "weekly":
		if weekday == nil || *weekday < 0 || *weekday > 6 {
			return httpx.Validation("payout_weekday is required for weekly payout (0=Sunday … 6=Saturday)")
		}
	case "monthly":
		if monthDay == nil || *monthDay < 1 || *monthDay > 28 {
			return httpx.Validation("payout_month_day is required for monthly payout (1–28)")
		}
	}
	return nil
}
