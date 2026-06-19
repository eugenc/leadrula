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
	tests := []struct {
		ftype    string
		raw      string
		timezone string
		want     string
	}{
		{"datetime", "2026-06-19T23:00:00Z", "America/New_York", "2026-06-19T19:00"},
		{"datetime", "2026-06-19T23:00:00Z", "", "2026-06-19T19:00"},
		// Unknown zone falls back to defaultSunbaseTimezone when tzdata is available.
		{"datetime", "2026-06-08T14:30:00Z", "Invalid/Zone", "2026-06-08T10:30"},
		{"datetime", "2026-06-08T14:30:00Z", "America/Chicago", "2026-06-08T09:30"},
		{"datetime", "not-a-date", "America/New_York", "not-a-date"},
		{"datetime", "", "America/New_York", ""},
	}
	for _, tc := range tests {
		got := FormatForSunbaseExportInTimezone(tc.ftype, tc.raw, tc.timezone)
		if got != tc.want {
			t.Errorf("FormatForSunbaseExportInTimezone(%q, %q, %q) = %q, want %q",
				tc.ftype, tc.raw, tc.timezone, got, tc.want)
		}
	}
}
