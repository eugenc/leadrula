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
