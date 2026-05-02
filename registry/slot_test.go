package registry

import "testing"

func TestSlotMatcherMatches(t *testing.T) {
	tests := []struct {
		expected string
		actual   string
		want     bool
	}{
		{"", "", true},
		{"", "blue", true},
		{"blue", "blue", true},
		{"blue", "green", false},
		{"blue", "", false},
	}
	for _, tt := range tests {
		m := SlotMatcher{Expected: tt.expected}
		if got := m.Matches(tt.actual); got != tt.want {
			t.Errorf("SlotMatcher{%q}.Matches(%q) = %v, want %v", tt.expected, tt.actual, got, tt.want)
		}
	}
}
