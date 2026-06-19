package customfields

import "testing"

func TestFormatForSunbaseExport(t *testing.T) {
	tests := []struct {
		ftype string
		raw   string
		want  string
	}{
		{"datetime", "2026-06-08T14:30:00-04:00", "2026-06-08T14:30"},
		{"datetime", "2026-06-08T14:30", "2026-06-08T14:30"},
		{"datetime", "2026-06-08T14:30:00Z", "2026-06-08T14:30"},
		{"date", "2026-06-08", "2026-06-08"},
		{"date", "2026-06-08T14:30:00-04:00", "2026-06-08"},
		{"text", "2026-06-08T14:30:00-04:00", "2026-06-08T14:30:00-04:00"},
		{"datetime", "not-a-date", "not-a-date"},
		{"datetime", "", ""},
	}
	for _, tc := range tests {
		got := FormatForSunbaseExport(tc.ftype, tc.raw)
		if got != tc.want {
			t.Errorf("FormatForSunbaseExport(%q, %q) = %q, want %q", tc.ftype, tc.raw, got, tc.want)
		}
	}
}

func TestFormatForSunbaseExportInTimezone(t *testing.T) {
	got := FormatForSunbaseExportInTimezone("datetime", "2026-06-19T23:00:00Z", "America/New_York")
	want := "2026-06-19T19:00"
	if got != want {
		t.Fatalf("FormatForSunbaseExportInTimezone() = %q, want %q", got, want)
	}
}
