package appointments

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

const (
	minDurationMin = 15
	maxDurationMin = 240
	minCapacity    = 1
	maxCapacity    = 20
	maxBufferMin   = 60
	maxAdvanceDays = 90
)

// DaySchedule is one day's working window.
type DaySchedule struct {
	Start string `json:"start"`
	End   string `json:"end"`
}

// WeeklySchedule is working hours per weekday key (mon..sun) plus optional tz in JSON.
type WeeklySchedule map[string]json.RawMessage

var weekdayKeys = []string{"sun", "mon", "tue", "wed", "thu", "fri", "sat"}

func weekdayKey(w time.Weekday) string {
	return weekdayKeys[int(w)]
}

func parseDayWindow(raw json.RawMessage) (DaySchedule, bool) {
	var w DaySchedule
	if json.Unmarshal(raw, &w) != nil || w.Start == "" || w.End == "" {
		return DaySchedule{}, false
	}
	return w, true
}

func (s WeeklySchedule) dayWindow(weekday time.Weekday) (DaySchedule, bool) {
	raw, ok := s[weekdayKey(weekday)]
	if !ok {
		return DaySchedule{}, false
	}
	return parseDayWindow(raw)
}

func timeInWindow(t time.Time, start, end string) bool {
	cur := t.Format("15:04")
	return cur >= start && cur <= end
}

func slotInsideWorkingHours(schedule WeeklySchedule, weekday time.Weekday, start time.Time, durationMin int) bool {
	w, ok := schedule.dayWindow(weekday)
	if !ok {
		return false
	}
	end := start.Add(time.Duration(durationMin) * time.Minute)
	return timeInWindow(start, w.Start, w.End) && timeInWindow(end, w.Start, w.End)
}

// interval overlaps [start, start+duration) with buffer after end.
type timeInterval struct {
	start    time.Time // wall clock on same day
	duration int
	buffer   int
}

func (a timeInterval) endWithBuffer() time.Time {
	return a.start.Add(time.Duration(a.duration+a.buffer) * time.Minute)
}

func intervalsOverlap(a, b timeInterval) bool {
	aEnd := a.endWithBuffer()
	bEnd := b.endWithBuffer()
	return a.start.Before(bEnd) && b.start.Before(aEnd)
}

func validateNoOverlap(existing []timeInterval, candidate timeInterval) error {
	for _, e := range existing {
		if intervalsOverlap(e, candidate) {
			return fmt.Errorf("slot overlaps another slot on this day")
		}
	}
	return nil
}

func combineDateAndTime(date time.Time, startTime string, loc *time.Location) (time.Time, error) {
	startTime = strings.TrimSpace(startTime)
	if len(startTime) < 4 {
		return time.Time{}, fmt.Errorf("invalid start time")
	}
	parts := strings.Split(startTime, ":")
	if len(parts) < 2 {
		return time.Time{}, fmt.Errorf("invalid start time")
	}
	var h, m int
	if _, err := fmt.Sscanf(parts[0], "%d", &h); err != nil {
		return time.Time{}, fmt.Errorf("invalid start time")
	}
	if _, err := fmt.Sscanf(parts[1], "%d", &m); err != nil {
		return time.Time{}, fmt.Errorf("invalid start time")
	}
	y, mo, d := date.In(loc).Date()
	return time.Date(y, mo, d, h, m, 0, 0, loc), nil
}

func loadLocation(tz string) *time.Location {
	tz = strings.TrimSpace(tz)
	if tz == "" {
		return time.UTC
	}
	loc, err := time.LoadLocation(tz)
	if err != nil {
		return time.UTC
	}
	return loc
}

func formatStartTime(t time.Time) string {
	return t.Format("15:04:05")
}

func parseDateParam(v string, loc *time.Location) (time.Time, error) {
	v = strings.TrimSpace(v)
	if v == "" {
		return time.Time{}, fmt.Errorf("date is required")
	}
	if t, err := time.ParseInLocation("2006-01-02", v, loc); err == nil {
		return t, nil
	}
	return time.Time{}, fmt.Errorf("invalid date")
}

func bookingWindowOK(slotStart time.Time, now time.Time) bool {
	if slotStart.Before(now) {
		return false
	}
	max := now.AddDate(0, 0, maxAdvanceDays)
	return !slotStart.After(max)
}

func timeNowWeekdayRef(weekday int, loc *time.Location) time.Time {
	now := time.Now().In(loc)
	diff := int(time.Weekday(weekday)) - int(now.Weekday())
	return now.AddDate(0, 0, diff)
}

func roundTo15Min(t time.Time) time.Time {
	m := t.Minute()
	rem := m % 15
	if rem != 0 {
		t = t.Add(time.Duration(15-rem) * time.Minute)
	}
	return time.Date(t.Year(), t.Month(), t.Day(), t.Hour(), t.Minute(), 0, 0, t.Location())
}
