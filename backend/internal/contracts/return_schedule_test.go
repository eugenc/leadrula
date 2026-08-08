package contracts

import (
	"testing"
	"time"
)

func TestNextReturnExecuteAt_delay(t *testing.T) {
	secs := 3600
	now := time.Date(2026, 3, 10, 12, 0, 0, 0, time.UTC)
	got, err := NextReturnExecuteAt(now, "UTC", ReturnRule{
		ReturnScheduleMode: ReturnScheduleDelay,
		ReturnDelaySeconds: &secs,
	})
	if err != nil {
		t.Fatal(err)
	}
	want := now.Add(time.Hour)
	if !got.Equal(want) {
		t.Fatalf("got %v want %v", got, want)
	}
}

func TestNextReturnExecuteAt_dailySameDay(t *testing.T) {
	clock := "09:00"
	now := time.Date(2026, 3, 10, 8, 0, 0, 0, time.UTC)
	got, err := NextReturnExecuteAt(now, "UTC", ReturnRule{
		ReturnScheduleMode: ReturnScheduleDaily,
		ReturnTime:         &clock,
	})
	if err != nil {
		t.Fatal(err)
	}
	want := time.Date(2026, 3, 10, 9, 0, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Fatalf("got %v want %v", got, want)
	}
}

func TestNextReturnExecuteAt_dailyNextDay(t *testing.T) {
	clock := "09:00"
	now := time.Date(2026, 3, 10, 10, 0, 0, 0, time.UTC)
	got, err := NextReturnExecuteAt(now, "UTC", ReturnRule{
		ReturnScheduleMode: ReturnScheduleDaily,
		ReturnTime:         &clock,
	})
	if err != nil {
		t.Fatal(err)
	}
	want := time.Date(2026, 3, 11, 9, 0, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Fatalf("got %v want %v", got, want)
	}
}

func TestNextReturnExecuteAt_weekly(t *testing.T) {
	clock := "09:00"
	// Tuesday 2026-03-10, next Fri 9am New York
	now := time.Date(2026, 3, 10, 15, 0, 0, 0, time.UTC)
	got, err := NextReturnExecuteAt(now, "America/New_York", ReturnRule{
		ReturnScheduleMode: ReturnScheduleWeekly,
		ReturnTime:         &clock,
		ReturnWeekdays:     []int{5},
	})
	if err != nil {
		t.Fatal(err)
	}
	loc, _ := time.LoadLocation("America/New_York")
	want := time.Date(2026, 3, 13, 9, 0, 0, 0, loc).UTC()
	if !got.Equal(want) {
		t.Fatalf("got %v want %v", got, want)
	}
}

func TestValidateReturnSchedule_weeklyRequiresWeekday(t *testing.T) {
	clock := "09:00"
	_, err := ValidateReturnSchedule(ReturnScheduleInput{
		Mode:       ReturnScheduleWeekly,
		ReturnTime: &clock,
		Weekdays:   []int{},
	})
	if err == nil {
		t.Fatal("expected validation error")
	}
}

func TestParseReturnDelay(t *testing.T) {
	secs, err := ParseReturnDelay(2, "hours")
	if err != nil {
		t.Fatal(err)
	}
	if secs != 7200 {
		t.Fatalf("got %d", secs)
	}
}

func TestReturnSchedulePatchResolved_delay(t *testing.T) {
	val := 3
	unit := "days"
	patch := ReturnSchedulePatch{Mode: strPtr(ReturnScheduleDelay), DelayValue: &val, DelayUnit: &unit}
	got, err := patch.Resolved(ReturnRule{})
	if err != nil {
		t.Fatal(err)
	}
	if got.Mode != ReturnScheduleDelay || got.DelaySeconds == nil || *got.DelaySeconds != 259200 {
		t.Fatalf("unexpected %+v", got)
	}
}

func strPtr(s string) *string { return &s }
