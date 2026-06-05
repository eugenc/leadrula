package contracts

import "testing"

func TestValidateLeadType(t *testing.T) {
	for _, tc := range []struct {
		value    string
		required bool
		ok       bool
	}{
		{"Data", true, true},
		{"Appointment", true, true},
		{"Call", true, true},
		{"", true, false},
		{"", false, true},
		{"Solar", true, false},
		{" data ", true, false},
	} {
		err := validateLeadType(tc.value, tc.required)
		if tc.ok && err != nil {
			t.Fatalf("%q required=%v: want ok, got %v", tc.value, tc.required, err)
		}
		if !tc.ok && err == nil {
			t.Fatalf("%q required=%v: want error", tc.value, tc.required)
		}
	}
}
