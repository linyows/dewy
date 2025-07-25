package registry

import (
	"testing"
)

func TestMatchesPlatform(t *testing.T) {
	tests := []struct {
		name         string
		artifactName string
		archMatches  []string
		osMatches    []string
		expected     bool
	}{
		// Basic matching
		{
			name:         "both arch and os match",
			artifactName: "myapp-linux-amd64.tar.gz",
			archMatches:  []string{"amd64"},
			osMatches:    []string{"linux"},
			expected:     true,
		},
		{
			name:         "arch match with x86_64 alias",
			artifactName: "myapp-linux-x86_64.tar.gz",
			archMatches:  []string{"amd64", "x86_64"},
			osMatches:    []string{"linux"},
			expected:     true,
		},
		{
			name:         "os match with macos alias",
			artifactName: "myapp-macos-amd64.tar.gz",
			archMatches:  []string{"amd64"},
			osMatches:    []string{"darwin", "macos"},
			expected:     true,
		},

		// Case sensitivity
		{
			name:         "case insensitive - uppercase",
			artifactName: "MyApp-LINUX-AMD64.tar.gz",
			archMatches:  []string{"amd64"},
			osMatches:    []string{"linux"},
			expected:     true,
		},
		{
			name:         "case insensitive - mixed case",
			artifactName: "myapp-Linux-amd64.tar.gz",
			archMatches:  []string{"amd64"},
			osMatches:    []string{"linux"},
			expected:     true,
		},

		// Separator variations
		{
			name:         "underscore separator",
			artifactName: "myapp_linux_amd64.tar.gz",
			archMatches:  []string{"amd64"},
			osMatches:    []string{"linux"},
			expected:     true,
		},
		{
			name:         "mixed separators",
			artifactName: "myapp.linux-amd64.tar.gz",
			archMatches:  []string{"amd64"},
			osMatches:    []string{"linux"},
			expected:     true,
		},
		{
			name:         "no separators",
			artifactName: "myapplinuxamd64.tar.gz",
			archMatches:  []string{"amd64"},
			osMatches:    []string{"linux"},
			expected:     true,
		},

		// Version patterns
		{
			name:         "version in filename",
			artifactName: "myapp_v1.2.3-linux-amd64.tar.gz",
			archMatches:  []string{"amd64"},
			osMatches:    []string{"linux"},
			expected:     true,
		},
		{
			name:         "semantic version",
			artifactName: "myapp-1.2.3-linux-amd64.tar.gz",
			archMatches:  []string{"amd64"},
			osMatches:    []string{"linux"},
			expected:     true,
		},

		// File extensions
		{
			name:         "no extension",
			artifactName: "myapp-linux-amd64",
			archMatches:  []string{"amd64"},
			osMatches:    []string{"linux"},
			expected:     true,
		},
		{
			name:         "zip extension",
			artifactName: "myapp-linux-amd64.zip",
			archMatches:  []string{"amd64"},
			osMatches:    []string{"linux"},
			expected:     true,
		},

		// Missing components
		{
			name:         "arch missing",
			artifactName: "myapp-linux.tar.gz",
			archMatches:  []string{"amd64"},
			osMatches:    []string{"linux"},
			expected:     false,
		},
		{
			name:         "os missing",
			artifactName: "myapp-amd64.tar.gz",
			archMatches:  []string{"amd64"},
			osMatches:    []string{"linux"},
			expected:     false,
		},

		// Wrong components
		{
			name:         "wrong arch",
			artifactName: "myapp-linux-arm64.tar.gz",
			archMatches:  []string{"amd64"},
			osMatches:    []string{"linux"},
			expected:     false,
		},
		{
			name:         "wrong os",
			artifactName: "myapp-windows-amd64.tar.gz",
			archMatches:  []string{"amd64"},
			osMatches:    []string{"linux"},
			expected:     false,
		},

		// Edge cases
		{
			name:         "empty artifact name",
			artifactName: "",
			archMatches:  []string{"amd64"},
			osMatches:    []string{"linux"},
			expected:     false,
		},
		{
			name:         "complex filename with multiple dashes",
			artifactName: "my-complex-app-name-v1.0.0-linux-amd64.tar.gz",
			archMatches:  []string{"amd64"},
			osMatches:    []string{"linux"},
			expected:     true,
		},

		// Multiple matches
		{
			name:         "multiple arch matches - first wins",
			artifactName: "myapp-linux-amd64.tar.gz",
			archMatches:  []string{"arm64", "amd64", "386"},
			osMatches:    []string{"linux"},
			expected:     true,
		},
		{
			name:         "multiple os matches - first wins",
			artifactName: "myapp-darwin-amd64.tar.gz",
			archMatches:  []string{"amd64"},
			osMatches:    []string{"linux", "windows", "darwin"},
			expected:     true,
		},

		// Real-world examples
		{
			name:         "GitHub release style",
			artifactName: "dewy_linux_amd64",
			archMatches:  []string{"amd64", "x86_64"},
			osMatches:    []string{"linux"},
			expected:     true,
		},
		{
			name:         "GoReleaser style with x86_64",
			artifactName: "myapp_Linux_x86_64.tar.gz",
			archMatches:  []string{"amd64", "x86_64"},
			osMatches:    []string{"linux"},
			expected:     true,
		},
		{
			name:         "Docker style",
			artifactName: "myapp-linux-amd64.tar.gz",
			archMatches:  []string{"amd64", "x86_64"},
			osMatches:    []string{"linux"},
			expected:     true,
		},
		{
			name:         "macOS with alias",
			artifactName: "myapp-macos-amd64.tar.gz",
			archMatches:  []string{"amd64", "x86_64"},
			osMatches:    []string{"darwin", "macos"},
			expected:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := matchesPlatform(tt.artifactName, tt.archMatches, tt.osMatches)

			if got != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, got)
			}
		})
	}
}

func TestMatchArtifactByPlatform(t *testing.T) {
	// Save original values
	origArch := TestArch
	origOS := TestOS
	defer func() {
		TestArch = origArch
		TestOS = origOS
	}()

	tests := []struct {
		name          string
		arch          string
		os            string
		artifactNames []string
		expectedName  string
		expectedFound bool
	}{
		{
			name: "amd64 linux integration test",
			arch: "amd64",
			os:   "linux",
			artifactNames: []string{
				"myapp-windows-amd64.zip",
				"myapp-linux-amd64.tar.gz",
				"myapp-darwin-amd64.tar.gz",
			},
			expectedName:  "myapp-linux-amd64.tar.gz",
			expectedFound: true,
		},
		{
			name: "amd64 darwin integration test with alias",
			arch: "amd64",
			os:   "darwin",
			artifactNames: []string{
				"myapp-linux-amd64.tar.gz",
				"myapp-macos-x86_64.tar.gz",
				"myapp-windows-amd64.zip",
			},
			expectedName:  "myapp-macos-x86_64.tar.gz",
			expectedFound: true,
		},
		{
			name: "no match found",
			arch: "amd64",
			os:   "linux",
			artifactNames: []string{
				"myapp-windows-amd64.zip",
				"myapp-darwin-arm64.tar.gz",
			},
			expectedName:  "",
			expectedFound: false,
		},
		{
			name:          "empty artifact list",
			arch:          "amd64",
			os:            "linux",
			artifactNames: []string{},
			expectedName:  "",
			expectedFound: false,
		},
		{
			name: "first match wins",
			arch: "amd64",
			os:   "linux",
			artifactNames: []string{
				"first-linux-amd64.tar.gz",
				"second-linux-amd64.tar.gz",
			},
			expectedName:  "first-linux-amd64.tar.gz",
			expectedFound: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set test architecture and OS
			TestArch = tt.arch
			TestOS = tt.os

			gotName, gotFound := MatchArtifactByPlatform(tt.artifactNames)

			if gotFound != tt.expectedFound {
				t.Errorf("expected found=%v, got found=%v", tt.expectedFound, gotFound)
			}

			if gotName != tt.expectedName {
				t.Errorf("expected name=%s, got name=%s", tt.expectedName, gotName)
			}
		})
	}
}
