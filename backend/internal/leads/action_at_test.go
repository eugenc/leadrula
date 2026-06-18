package leads

import (
	"testing"
	"time"
)

func TestParseActionAt(t *testing.T) {
	rfc := "2026-06-15T14:00:00Z"
	dateOnly := "2026-06-15"
	fallback := "06/15/2026 2:00 PM"

	tests := []struct {
		name    string
		in      any
		wantErr bool
		check   func(t *testing.T, got *time.Time)
	}{
		{
			name: "empty",
			in:   "",
			check: func(t *testing.T, got *time.Time) {
				if got != nil {
					t.Fatalf("got %v want nil", got)
				}
			},
		},
		{
			name: "rfc3339",
			in:   rfc,
			check: func(t *testing.T, got *time.Time) {
				if got == nil {
					t.Fatal("expected time")
				}
				if got.UTC().Format(time.RFC3339) != rfc {
					t.Fatalf("got %v want %v", got.UTC().Format(time.RFC3339), rfc)
				}
			},
		},
		{
			name: "date only",
			in:   dateOnly,
			check: func(t *testing.T, got *time.Time) {
				if got == nil {
					t.Fatal("expected time")
				}
				if got.Format("2006-01-02") != dateOnly {
					t.Fatalf("got %v want date %v", got.Format("2006-01-02"), dateOnly)
				}
			},
		},
		{
			name: "fallback datetime",
			in:   fallback,
			check: func(t *testing.T, got *time.Time) {
				if got == nil {
					t.Fatal("expected time")
				}
				if got.Format("01/02/2006 3:04 PM") != fallback {
					t.Fatalf("got %v want %v", got.Format("01/02/2006 3:04 PM"), fallback)
				}
			},
		},
		{
			name:    "invalid",
			in:      "not-a-date",
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ParseActionAt(tc.in)
			if tc.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tc.check != nil {
				tc.check(t, got)
			}
		})
	}
}
