package contracts

import (
	"fmt"
	"strings"
	"time"

	"github.com/echayko/leadrula/backend/pkg/httpx"
)

const (
	ReturnScheduleImmediate = "immediate"
	ReturnScheduleDelay     = "delay"
	ReturnScheduleDaily     = "daily"
	ReturnScheduleWeekly    = "weekly"

	maxReturnDelaySeconds = 30 * 24 * 60 * 60
)

// ReturnScheduleInput is the API-facing schedule configuration for a return route.
type ReturnScheduleInput struct {
	Mode         string
	DelaySeconds *int
	ReturnTime   *string
	Weekdays     []int
}

// ReturnSchedulePatch holds optional schedule fields from PATCH/POST bodies.
type ReturnSchedulePatch struct {
	Mode              *string
	DelayValue        *int
	DelayUnit         *string
	DelaySeconds      *int
	ReturnTime        *string
	ReturnWeekdays    []int
	ReturnWeekdaysSet bool
}

func ParseReturnDelay(value int, unit string) (int, error) {
	if value <= 0 {
		return 0, httpx.Validation("delay value must be positive")
	}
	switch strings.TrimSpace(strings.ToLower(unit)) {
	case "minutes", "minute", "min":
		return value * 60, nil
	case "hours", "hour", "hr":
		return value * 3600, nil
	case "days", "day":
		return value * 86400, nil
	default:
		return 0, httpx.Validation("delay unit must be minutes, hours, or days")
	}
}

func (p ReturnSchedulePatch) Resolved(existing ReturnRule) (ReturnScheduleInput, error) {
	mode := existing.ReturnScheduleMode
	if mode == "" {
		mode = ReturnScheduleImmediate
	}
	if p.Mode != nil {
		mode = strings.TrimSpace(*p.Mode)
	}

	var delaySeconds *int
	var returnTime *string
	var weekdays []int

	switch mode {
	case ReturnScheduleImmediate:
	case ReturnScheduleDelay:
		if p.DelaySeconds != nil {
			delaySeconds = p.DelaySeconds
		} else if p.DelayValue != nil && p.DelayUnit != nil {
			secs, err := ParseReturnDelay(*p.DelayValue, *p.DelayUnit)
			if err != nil {
				return ReturnScheduleInput{}, err
			}
			delaySeconds = &secs
		} else if existing.ReturnDelaySeconds != nil {
			delaySeconds = existing.ReturnDelaySeconds
		} else {
			return ReturnScheduleInput{}, httpx.Validation("return_delay_value and return_delay_unit are required for delay schedule")
		}
	case ReturnScheduleDaily:
		if p.ReturnTime != nil {
			returnTime = p.ReturnTime
		} else if existing.ReturnTime != nil {
			returnTime = existing.ReturnTime
		} else {
			return ReturnScheduleInput{}, httpx.Validation("return_time is required for daily schedule")
		}
	case ReturnScheduleWeekly:
		if p.ReturnTime != nil {
			returnTime = p.ReturnTime
		} else if existing.ReturnTime != nil {
			returnTime = existing.ReturnTime
		} else {
			return ReturnScheduleInput{}, httpx.Validation("return_time is required for weekly schedule")
		}
		if p.ReturnWeekdaysSet {
			weekdays = append([]int(nil), p.ReturnWeekdays...)
		} else if len(existing.ReturnWeekdays) > 0 {
			weekdays = append([]int(nil), existing.ReturnWeekdays...)
		} else {
			return ReturnScheduleInput{}, httpx.Validation("return_weekdays is required for weekly schedule")
		}
	default:
		return ReturnScheduleInput{}, httpx.Validation("return_schedule_mode must be immediate, delay, daily, or weekly")
	}

	return ValidateReturnSchedule(ReturnScheduleInput{
		Mode:         mode,
		DelaySeconds: delaySeconds,
		ReturnTime:   returnTime,
		Weekdays:     weekdays,
	})
}

func ValidateReturnSchedule(in ReturnScheduleInput) (ReturnScheduleInput, error) {
	mode := strings.TrimSpace(in.Mode)
	if mode == "" {
		mode = ReturnScheduleImmediate
	}
	switch mode {
	case ReturnScheduleImmediate:
		return ReturnScheduleInput{Mode: mode}, nil
	case ReturnScheduleDelay:
		if in.DelaySeconds == nil || *in.DelaySeconds <= 0 {
			return ReturnScheduleInput{}, httpx.Validation("return_delay_seconds must be positive for delay schedule")
		}
		if *in.DelaySeconds > maxReturnDelaySeconds {
			return ReturnScheduleInput{}, httpx.Validation("return delay cannot exceed 30 days")
		}
		secs := *in.DelaySeconds
		return ReturnScheduleInput{Mode: mode, DelaySeconds: &secs}, nil
	case ReturnScheduleDaily:
		t, err := parseReturnClockTime(in.ReturnTime)
		if err != nil {
			return ReturnScheduleInput{}, err
		}
		return ReturnScheduleInput{Mode: mode, ReturnTime: &t}, nil
	case ReturnScheduleWeekly:
		t, err := parseReturnClockTime(in.ReturnTime)
		if err != nil {
			return ReturnScheduleInput{}, err
		}
		if len(in.Weekdays) == 0 {
			return ReturnScheduleInput{}, httpx.Validation("return_weekdays must include at least one day for weekly schedule")
		}
		seen := make(map[int]bool, len(in.Weekdays))
		outDays := make([]int, 0, len(in.Weekdays))
		for _, d := range in.Weekdays {
			if d < 0 || d > 6 {
				return ReturnScheduleInput{}, httpx.Validation("return_weekdays must use 0=Sunday through 6=Saturday")
			}
			if seen[d] {
				continue
			}
			seen[d] = true
			outDays = append(outDays, d)
		}
		return ReturnScheduleInput{Mode: mode, ReturnTime: &t, Weekdays: outDays}, nil
	default:
		return ReturnScheduleInput{}, httpx.Validation("return_schedule_mode must be immediate, delay, daily, or weekly")
	}
}

func parseReturnClockTime(raw *string) (string, error) {
	if raw == nil || strings.TrimSpace(*raw) == "" {
		return "", httpx.Validation("return_time is required")
	}
	s := strings.TrimSpace(*raw)
	if len(s) == 5 {
		s += ":00"
	}
	parsed, err := time.Parse("15:04:05", s)
	if err != nil {
		return "", httpx.Validation("return_time must be HH:MM")
	}
	return parsed.Format("15:04:05"), nil
}

func formatReturnTimeForJSON(raw *string) *string {
	if raw == nil {
		return nil
	}
	s := strings.TrimSpace(*raw)
	if s == "" {
		return nil
	}
	if len(s) >= 5 {
		out := s[:5]
		return &out
	}
	return &s
}

func NextReturnExecuteAt(now time.Time, tzName string, rule ReturnRule) (time.Time, error) {
	mode := rule.ReturnScheduleMode
	if mode == "" {
		mode = ReturnScheduleImmediate
	}
	switch mode {
	case ReturnScheduleImmediate:
		return now, nil
	case ReturnScheduleDelay:
		if rule.ReturnDelaySeconds == nil || *rule.ReturnDelaySeconds <= 0 {
			return time.Time{}, httpx.BusinessRule("return delay is misconfigured")
		}
		return now.Add(time.Duration(*rule.ReturnDelaySeconds) * time.Second), nil
	case ReturnScheduleDaily, ReturnScheduleWeekly:
		loc, err := time.LoadLocation(normalizeTimezone(tzName))
		if err != nil {
			return time.Time{}, fmt.Errorf("load contract timezone: %w", err)
		}
		clock, err := parseReturnClockTime(rule.ReturnTime)
		if err != nil {
			return time.Time{}, err
		}
		h, m, s := clockParts(clock)
		localNow := now.In(loc)
		if mode == ReturnScheduleDaily {
			candidate := time.Date(localNow.Year(), localNow.Month(), localNow.Day(), h, m, s, 0, loc)
			if !candidate.After(localNow) {
				candidate = candidate.Add(24 * time.Hour)
			}
			return candidate.UTC(), nil
		}
		allowed := weekdaySet(rule.ReturnWeekdays)
		for i := 0; i < 8; i++ {
			day := localNow.AddDate(0, 0, i)
			if !allowed[int(day.Weekday())] {
				continue
			}
			candidate := time.Date(day.Year(), day.Month(), day.Day(), h, m, s, 0, loc)
			if candidate.After(localNow) {
				return candidate.UTC(), nil
			}
		}
		return time.Time{}, httpx.BusinessRule("could not compute next weekly return time")
	default:
		return time.Time{}, httpx.BusinessRule("return schedule is misconfigured")
	}
}

func normalizeTimezone(tz string) string {
	tz = strings.TrimSpace(tz)
	if tz == "" {
		return "America/New_York"
	}
	return tz
}

// NormalizeTimezoneForDisplay is used when formatting scheduled return labels.
func NormalizeTimezoneForDisplay(tz string) string {
	return normalizeTimezone(tz)
}

func clockParts(clock string) (hour, min, sec int) {
	t, err := time.Parse("15:04:05", clock)
	if err != nil {
		return 0, 0, 0
	}
	return t.Hour(), t.Minute(), t.Second()
}

func weekdaySet(days []int) map[int]bool {
	out := make(map[int]bool, len(days))
	for _, d := range days {
		out[d] = true
	}
	return out
}

func scheduleDelayValueUnit(seconds *int) (*int, *string) {
	if seconds == nil || *seconds <= 0 {
		return nil, nil
	}
	secs := *seconds
	if secs%86400 == 0 {
		v := secs / 86400
		u := "days"
		return &v, &u
	}
	if secs%3600 == 0 {
		v := secs / 3600
		u := "hours"
		return &v, &u
	}
	v := secs / 60
	if v <= 0 {
		v = 1
	}
	u := "minutes"
	return &v, &u
}

func enrichReturnRuleSchedule(rr *ReturnRule) {
	if rr == nil {
		return
	}
	if rr.ReturnScheduleMode == "" {
		rr.ReturnScheduleMode = ReturnScheduleImmediate
	}
	rr.ReturnTime = formatReturnTimeForJSON(rr.ReturnTime)
	rr.ReturnDelayValue, rr.ReturnDelayUnit = scheduleDelayValueUnit(rr.ReturnDelaySeconds)
}
