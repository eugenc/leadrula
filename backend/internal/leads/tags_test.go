package leads

import "testing"

func TestParseTagsFromValue(t *testing.T) {
	tests := []struct {
		name string
		in   any
		want []string
	}{
		{"single string", "Broward DM 100k 2025", []string{"Broward DM 100k 2025"}},
		{"comma separated", "a, b ,c", []string{"a", "b", "c"}},
		{"string slice", []string{"a", "b"}, []string{"a", "b"}},
		{"any slice", []any{"x", "y"}, []string{"x", "y"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := ParseTagsFromValue(tc.in)
			if len(got) != len(tc.want) {
				t.Fatalf("got %v want %v", got, tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Fatalf("got %v want %v", got, tc.want)
				}
			}
		})
	}
}
