package contracts

import (
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/echayko/leadrula/backend/pkg/httpx"
)

const returnRuleBaseCols = `id, contract_id, participation_id, buyer_stage_id, return_stage_id, created_at, label`
const returnRuleScheduleCols = `return_schedule_mode, return_delay_seconds, return_time, return_weekdays`
const returnRuleAllCols = returnRuleBaseCols + `, ` + returnRuleScheduleCols

const maxReturnRuleLabelLen = 200

// returnRuleLabelArg normalizes an optional label patch. nil means omit; non-nil updates (empty → NULL).
func returnRuleLabelArg(label *string) (*string, bool, error) {
	if label == nil {
		return nil, false, nil
	}
	trimmed := strings.TrimSpace(*label)
	if trimmed == "" {
		return nil, true, nil
	}
	if len(trimmed) > maxReturnRuleLabelLen {
		return nil, false, httpx.Validation("label must be 200 characters or fewer")
	}
	return &trimmed, true, nil
}

func weekdaysToDB(days []int) []int16 {
	if len(days) == 0 {
		return nil
	}
	out := make([]int16, len(days))
	for i, d := range days {
		out[i] = int16(d)
	}
	return out
}

func weekdaysFromDB(days []int16) []int {
	if len(days) == 0 {
		return nil
	}
	out := make([]int, len(days))
	for i, d := range days {
		out[i] = int(d)
	}
	return out
}

func scanReturnRule(row pgx.Row) (*ReturnRule, error) {
	rr := &ReturnRule{}
	var clock *string
	var weekdays []int16
	var label *string
	err := row.Scan(
		&rr.ID, &rr.ContractID, &rr.ParticipationID, &rr.BuyerStageID, &rr.ReturnStageID, &rr.CreatedAt, &label,
		&rr.ReturnScheduleMode, &rr.ReturnDelaySeconds, &clock, &weekdays,
	)
	if err != nil {
		return nil, err
	}
	rr.Label = derefString(label)
	rr.ReturnTime = clock
	rr.ReturnWeekdays = weekdaysFromDB(weekdays)
	enrichReturnRuleSchedule(rr)
	return rr, nil
}

func derefString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func scanReturnRuleBasic(row pgx.Row) (*ReturnRule, error) {
	rr := &ReturnRule{}
	err := row.Scan(
		&rr.ID, &rr.ContractID, &rr.ParticipationID, &rr.BuyerStageID, &rr.ReturnStageID, &rr.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	rr.ReturnScheduleMode = ReturnScheduleImmediate
	enrichReturnRuleSchedule(rr)
	return rr, nil
}

func scanReturnRulesWithSchedule(rows pgx.Rows) ([]ReturnRule, error) {
	var out []ReturnRule
	for rows.Next() {
		var rr ReturnRule
		var clock *string
		var weekdays []int16
		var label *string
		if err := rows.Scan(
			&rr.ID, &rr.ContractID, &rr.ParticipationID, &rr.BuyerStageID, &rr.ReturnStageID, &rr.CreatedAt, &label,
			&rr.ReturnScheduleMode, &rr.ReturnDelaySeconds, &clock, &weekdays,
		); err != nil {
			return nil, err
		}
		rr.Label = derefString(label)
		rr.ReturnTime = clock
		rr.ReturnWeekdays = weekdaysFromDB(weekdays)
		enrichReturnRuleSchedule(&rr)
		out = append(out, rr)
	}
	return out, rows.Err()
}

func scanEnrichedReturnRulesWithSchedule(rows pgx.Rows) ([]ReturnRule, error) {
	var out []ReturnRule
	for rows.Next() {
		var rr ReturnRule
		var clock *string
		var weekdays []int16
		var label *string
		if err := rows.Scan(
			&rr.ID, &rr.ContractID, &rr.ParticipationID, &rr.BuyerStageID, &rr.ReturnStageID, &rr.CreatedAt, &label,
			&rr.BuyerStageName, &rr.Stale,
			&rr.ReturnScheduleMode, &rr.ReturnDelaySeconds, &clock, &weekdays,
		); err != nil {
			return nil, err
		}
		rr.Label = derefString(label)
		rr.ReturnTime = clock
		rr.ReturnWeekdays = weekdaysFromDB(weekdays)
		enrichReturnRuleSchedule(&rr)
		out = append(out, rr)
	}
	return out, rows.Err()
}

func scheduleSQLValues(schedule ReturnScheduleInput) (mode string, delay *int, clock *string, weekdays []int16) {
	mode = schedule.Mode
	if mode == ReturnScheduleImmediate {
		return mode, nil, nil, nil
	}
	if schedule.Mode == ReturnScheduleDelay {
		return mode, schedule.DelaySeconds, nil, nil
	}
	return mode, nil, schedule.ReturnTime, weekdaysToDB(schedule.Weekdays)
}

func resolveNewReturnSchedule(schedule ReturnSchedulePatch, hasSchedule bool) (ReturnScheduleInput, error) {
	if !hasSchedule {
		return ReturnScheduleInput{Mode: ReturnScheduleImmediate}, nil
	}
	return schedule.Resolved(ReturnRule{ReturnScheduleMode: ReturnScheduleImmediate})
}
