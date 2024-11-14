package registry

import (
	"testing"
)

func TestSemVerRegexWithoutPreRelease(t *testing.T) {
	tests := []struct {
		ver      string
		expected bool
	}{
		{"1.2.3", true},
		{"v1.2.3", true},
		{"v1.2.300", true},
		{"v1.2.3-rc", false},
		{"v1.2.3-beta.1", false},
		{"v8", false},
		{"12.3", false},
		{"abcdefg-10", false},
	}

	for _, tt := range tests {
		t.Run(tt.ver, func(t *testing.T) {
			got := SemVerRegexWithoutPreRelease.MatchString(tt.ver)
			if got != tt.expected {
				t.Errorf("expected %t, got %t", tt.expected, got)
			}
		})
	}
}

func TestSemVerRegex(t *testing.T) {
	tests := []struct {
		ver      string
		expected bool
	}{
		{"1.2.3", true},
		{"v1.2.3", true},
		{"v1.2.300", true},
		{"v1.2.3-rc", true},
		{"v1.2.3-beta.1", true},
		{"v8", false},
		{"12.3", false},
		{"abcdefg-10", false},
	}

	for _, tt := range tests {
		t.Run(tt.ver, func(t *testing.T) {
			got := SemVerRegex.MatchString(tt.ver)
			if got != tt.expected {
				t.Errorf("expected %t, got %t", tt.expected, got)
			}
		})
	}
}
