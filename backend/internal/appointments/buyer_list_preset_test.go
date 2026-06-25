package appointments

import (
	"testing"
	"time"
)

func TestAppointmentPresetBounds_all(t *testing.T) {
	from, to, exclude := appointmentPresetBounds("all", "America/New_York", time.Now())
	if from != nil || to != nil || exclude {
		t.Fatalf("all preset should not filter")
	}
}

func TestAppointmentPresetBounds_today(t *testing.T) {
	now := time.Date(2026, 6, 25, 15, 30, 0, 0, time.UTC)
	from, to, exclude := appointmentPresetBounds("today", "UTC", now)
	if !exclude || from == nil || to == nil {
		t.Fatal("today should exclude nulls and set bounds")
	}
	if !from.Equal(time.Date(2026, 6, 25, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("from=%v", from)
	}
	if !to.Equal(time.Date(2026, 6, 26, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("to=%v", to)
	}
}

func TestAppointmentPresetBounds_thisWeekMonday(t *testing.T) {
	// 2026-06-25 is Thursday
	now := time.Date(2026, 6, 25, 12, 0, 0, 0, time.UTC)
	from, to, exclude := appointmentPresetBounds("this_week", "UTC", now)
	if !exclude || from == nil || to == nil {
		t.Fatal("this_week should set bounds")
	}
	wantStart := time.Date(2026, 6, 22, 0, 0, 0, 0, time.UTC) // Monday
	if !from.Equal(wantStart) {
		t.Fatalf("from=%v want %v", from, wantStart)
	}
	if !to.Equal(wantStart.AddDate(0, 0, 7)) {
		t.Fatalf("to=%v", to)
	}
}

func TestAppointmentPresetBounds_thisMonth(t *testing.T) {
	now := time.Date(2026, 6, 15, 12, 0, 0, 0, time.UTC)
	from, to, exclude := appointmentPresetBounds("this_month", "UTC", now)
	if !exclude || from == nil || to == nil {
		t.Fatal("this_month should set bounds")
	}
	if !from.Equal(time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("from=%v", from)
	}
	if !to.Equal(time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("to=%v", to)
	}
}
