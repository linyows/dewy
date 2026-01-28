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
		// Build metadata tests
		{"v1.2.3+blue", true},
		{"v1.2.3+green", true},
		{"v1.2.3+build.123", true},
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
		// Build metadata tests
		{"v1.2.3+blue", true},
		{"v1.2.3+green", true},
		{"v1.2.3+build.123", true},
		{"v1.2.3-rc.1+blue", true},
		{"v1.2.3-beta+green", true},
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

func TestParseSemVerBuildMetadata(t *testing.T) {
	tests := []struct {
		ver           string
		expectedBuild string
		expectedPre   string
	}{
		{"v1.2.3", "", ""},
		{"v1.2.3+blue", "blue", ""},
		{"v1.2.3+green", "green", ""},
		{"v1.2.3-rc.1", "", "rc.1"},
		{"v1.2.3-rc.1+blue", "blue", "rc.1"},
		{"v1.2.3-beta+green", "green", "beta"},
		{"v1.2.3+build.123", "build.123", ""},
	}

	for _, tt := range tests {
		t.Run(tt.ver, func(t *testing.T) {
			sv := ParseSemVer(tt.ver)
			if sv == nil {
				t.Fatalf("ParseSemVer(%s) returned nil", tt.ver)
			}
			if sv.BuildMetadata != tt.expectedBuild {
				t.Errorf("BuildMetadata = %q, want %q", sv.BuildMetadata, tt.expectedBuild)
			}
			if sv.PreRelease != tt.expectedPre {
				t.Errorf("PreRelease = %q, want %q", sv.PreRelease, tt.expectedPre)
			}
		})
	}
}

func TestSemVerString(t *testing.T) {
	tests := []struct {
		ver      string
		expected string
	}{
		{"v1.2.3", "v1.2.3"},
		{"v1.2.3+blue", "v1.2.3+blue"},
		{"v1.2.3-rc.1", "v1.2.3-rc.1"},
		{"v1.2.3-rc.1+blue", "v1.2.3-rc.1+blue"},
		{"1.2.3+green", "1.2.3+green"},
	}

	for _, tt := range tests {
		t.Run(tt.ver, func(t *testing.T) {
			sv := ParseSemVer(tt.ver)
			if sv == nil {
				t.Fatalf("ParseSemVer(%s) returned nil", tt.ver)
			}
			if sv.String() != tt.expected {
				t.Errorf("String() = %q, want %q", sv.String(), tt.expected)
			}
		})
	}
}

func TestFindLatestSemVerWithSlot(t *testing.T) {
	tests := []struct {
		name            string
		versionNames    []string
		slot            string
		allowPreRelease bool
		expectedVer     string
		expectedName    string
		expectError     bool
	}{
		{
			name:            "find latest with slot blue",
			versionNames:    []string{"v1.0.0+blue", "v1.1.0+green", "v1.2.0+blue", "v1.3.0+green"},
			slot:            "blue",
			allowPreRelease: false,
			expectedVer:     "v1.2.0+blue",
			expectedName:    "v1.2.0+blue",
			expectError:     false,
		},
		{
			name:            "find latest with slot green",
			versionNames:    []string{"v1.0.0+blue", "v1.1.0+green", "v1.2.0+blue", "v1.3.0+green"},
			slot:            "green",
			allowPreRelease: false,
			expectedVer:     "v1.3.0+green",
			expectedName:    "v1.3.0+green",
			expectError:     false,
		},
		{
			name:            "find latest without slot (any)",
			versionNames:    []string{"v1.0.0+blue", "v1.1.0+green", "v1.2.0+blue", "v1.3.0+green"},
			slot:            "",
			allowPreRelease: false,
			expectedVer:     "v1.3.0+green",
			expectedName:    "v1.3.0+green",
			expectError:     false,
		},
		{
			name:            "find latest with slot and pre-release",
			versionNames:    []string{"v1.0.0+blue", "v1.1.0-rc.1+blue", "v1.0.5+blue"},
			slot:            "blue",
			allowPreRelease: true,
			expectedVer:     "v1.1.0-rc.1+blue",
			expectedName:    "v1.1.0-rc.1+blue",
			expectError:     false,
		},
		{
			name:            "no matching slot",
			versionNames:    []string{"v1.0.0+blue", "v1.1.0+blue"},
			slot:            "green",
			allowPreRelease: false,
			expectedVer:     "",
			expectedName:    "",
			expectError:     true,
		},
		{
			name:            "mixed with and without build metadata",
			versionNames:    []string{"v1.0.0", "v1.1.0+blue", "v1.2.0", "v1.3.0+blue"},
			slot:            "blue",
			allowPreRelease: false,
			expectedVer:     "v1.3.0+blue",
			expectedName:    "v1.3.0+blue",
			expectError:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotVer, gotName, err := FindLatestSemVerWithSlot(tt.versionNames, tt.slot, tt.allowPreRelease)

			if tt.expectError {
				if err == nil {
					t.Errorf("FindLatestSemVerWithSlot() expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("FindLatestSemVerWithSlot() unexpected error: %v", err)
				return
			}

			if gotVer.String() != tt.expectedVer {
				t.Errorf("FindLatestSemVerWithSlot() version = %v, want %v", gotVer.String(), tt.expectedVer)
			}

			if gotName != tt.expectedName {
				t.Errorf("FindLatestSemVerWithSlot() name = %v, want %v", gotName, tt.expectedName)
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
