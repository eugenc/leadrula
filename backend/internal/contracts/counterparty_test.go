package contracts

import "testing"

func TestValidateCounterpartyAccountType(t *testing.T) {
	for _, tc := range []struct {
		contractType string
		accountType  string
		ok           bool
	}{
		{"buy", "publisher", true},
		{"buy", "buyer", false},
		{"sell", "buyer", true},
		{"sell", "publisher", true},
		{"sell", "platform", false},
		{"", "publisher", false},
	} {
		err := ValidateCounterpartyAccountType(tc.contractType, tc.accountType)
		if tc.ok && err != nil {
			t.Fatalf("%s+%s: want ok, got %v", tc.contractType, tc.accountType, err)
		}
		if !tc.ok && err == nil {
			t.Fatalf("%s+%s: want error", tc.contractType, tc.accountType)
		}
	}
}
