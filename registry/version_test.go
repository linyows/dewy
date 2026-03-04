package registry

import (
	"testing"
)

func TestComparePreRelease(t *testing.T) {
	tests := []struct {
		name     string
		a        string
		b        string
		expected int // negative if a < b, 0 if equal, positive if a > b
	}{
		// SemVer v2 spec item 11 ordering
		{"alpha < alpha.1", "alpha", "alpha.1", -1},
		{"alpha.1 < alpha.beta", "alpha.1", "alpha.beta", -1},
		{"alpha.beta < beta", "alpha.beta", "beta", -1},
		{"beta < beta.2", "beta", "beta.2", -1},
		{"beta.2 < beta.11", "beta.2", "beta.11", -1},
		{"beta.11 < rc.1", "beta.11", "rc.1", -1},

		// Numeric comparisons
		{"numeric 1 < 2", "1", "2", -1},
		{"numeric 2 < 11", "2", "11", -1},
		{"numeric equal", "5", "5", 0},

		// Numeric vs alphanumeric
		{"numeric < alphanumeric", "1", "alpha", -1},
		{"alphanumeric > numeric", "alpha", "1", 1},

		// Equal
		{"equal strings", "alpha", "alpha", 0},
		{"equal compound", "rc.1", "rc.1", 0},

		// Lexicographic for alphanumeric
		{"alpha < beta", "alpha", "beta", -1},
		{"beta < rc", "beta", "rc", -1},

		// Longer field count wins when prefix equal
		{"a.b < a.b.c", "a.b", "a.b.c", -1},
		{"a.b.c > a.b", "a.b.c", "a.b", 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := comparePreRelease(tt.a, tt.b)
			if (tt.expected < 0 && got >= 0) || (tt.expected > 0 && got <= 0) || (tt.expected == 0 && got != 0) {
				t.Errorf("comparePreRelease(%q, %q) = %d, want sign %d", tt.a, tt.b, got, tt.expected)
			}
		})
	}
}
