package contracts

import "testing"

func TestValidateCapLimits(t *testing.T) {
	total := 10
	daily := 5
	for _, tc := range []struct {
		period string
		total  *int
		daily  *int
		ok     bool
	}{
		{"one_time", &total, nil, true},
		{"weekly", &total, &daily, true},
		{"weekly", &total, nil, true},
		{"one_time", nil, nil, true},
		{"one_time", &total, &daily, false},
		{"monthly", &total, &daily, true},
		{"monthly", &total, nil, true},
		{"one_time", ptrInt(0), nil, false},
	} {
		err := validateCapLimits(tc.period, tc.total, tc.daily)
		if tc.ok && err != nil {
			t.Fatalf("%+v: want ok, got %v", tc, err)
		}
		if !tc.ok && err == nil {
			t.Fatalf("%+v: want error", tc)
		}
	}
}

func ptrInt(n int) *int { return &n }
