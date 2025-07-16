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

func TestFindLatestSemVer(t *testing.T) {
	tests := []struct {
		name            string
		versionNames    []string
		allowPreRelease bool
		expectedVer     string
		expectedName    string
		expectError     bool
	}{
		{
			name:            "find latest stable version",
			versionNames:    []string{"v1.0.0", "v1.2.0", "v1.1.0", "v2.0.0"},
			allowPreRelease: false,
			expectedVer:     "v2.0.0",
			expectedName:    "v2.0.0",
			expectError:     false,
		},
		{
			name:            "find latest with pre-release allowed",
			versionNames:    []string{"v1.0.0", "v1.2.0", "v2.0.0-rc.1", "v1.3.0"},
			allowPreRelease: true,
			expectedVer:     "v2.0.0-rc.1",
			expectedName:    "v2.0.0-rc.1",
			expectError:     false,
		},
		{
			name:            "find latest with pre-release not allowed",
			versionNames:    []string{"v1.0.0", "v1.2.0", "v2.0.0-rc.1", "v1.3.0"},
			allowPreRelease: false,
			expectedVer:     "v1.3.0",
			expectedName:    "v1.3.0",
			expectError:     false,
		},
		{
			name:            "mixed format versions",
			versionNames:    []string{"1.0.0", "v1.2.0", "2.0.0", "v1.1.0"},
			allowPreRelease: false,
			expectedVer:     "2.0.0",
			expectedName:    "2.0.0",
			expectError:     false,
		},
		{
			name:            "no valid versions",
			versionNames:    []string{"latest", "main", "invalid"},
			allowPreRelease: false,
			expectedVer:     "",
			expectedName:    "",
			expectError:     true,
		},
		{
			name:            "empty version list",
			versionNames:    []string{},
			allowPreRelease: false,
			expectedVer:     "",
			expectedName:    "",
			expectError:     true,
		},
		{
			name:            "pre-release versions only with pre-release not allowed",
			versionNames:    []string{"v1.0.0-alpha", "v1.0.0-beta", "v1.0.0-rc"},
			allowPreRelease: false,
			expectedVer:     "",
			expectedName:    "",
			expectError:     true,
		},
		{
			name:            "pre-release versions only with pre-release allowed",
			versionNames:    []string{"v1.0.0-alpha", "v1.0.0-beta", "v1.0.0-rc.1"},
			allowPreRelease: true,
			expectedVer:     "v1.0.0-rc.1",
			expectedName:    "v1.0.0-rc.1",
			expectError:     false,
		},
		{
			name:            "complex version ordering",
			versionNames:    []string{"v1.10.0", "v1.2.0", "v1.9.0", "v2.0.0-alpha", "v1.11.0"},
			allowPreRelease: false,
			expectedVer:     "v1.11.0",
			expectedName:    "v1.11.0",
			expectError:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotVer, gotName, err := FindLatestSemVer(tt.versionNames, tt.allowPreRelease)

			if tt.expectError {
				if err == nil {
					t.Errorf("FindLatestSemVer() expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("FindLatestSemVer() unexpected error: %v", err)
				return
			}

			if gotVer.String() != tt.expectedVer {
				t.Errorf("FindLatestSemVer() version = %v, want %v", gotVer.String(), tt.expectedVer)
			}

			if gotName != tt.expectedName {
				t.Errorf("FindLatestSemVer() name = %v, want %v", gotName, tt.expectedName)
			}
		})
	}
}
