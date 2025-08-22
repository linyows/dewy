package registry

import (
	"runtime"
	"strings"
)

var (
	// TestArch overrides runtime.GOARCH when set (for testing).
	TestArch string
	// TestOS overrides runtime.GOOS when set (for testing).
	TestOS string
)

// getArch returns TestArch if set, otherwise runtime.GOARCH.
func getArch() string {
	if TestArch != "" {
		return TestArch
	}
	return runtime.GOARCH
}

// getOS returns TestOS if set, otherwise runtime.GOOS.
func getOS() string {
	if TestOS != "" {
		return TestOS
	}
	return runtime.GOOS
}

// MatchArtifactByPlatform finds the first artifact name that matches current OS and architecture.
func MatchArtifactByPlatform(artifactNames []string) (string, bool) {
	arch := getArch()
	os := getOS()

	archMatches := []string{arch}
	if arch == "amd64" {
		archMatches = append(archMatches, "x86_64")
	}

	osMatches := []string{os}
	if os == "darwin" {
		osMatches = append(osMatches, "macos")
	}

	for _, name := range artifactNames {
		if isArchiveFile(name) && matchesPlatform(name, archMatches, osMatches) {
			return name, true
		}
	}

	return "", false
}

// matchesPlatform checks if artifact name contains both arch and OS patterns.
func matchesPlatform(artifactName string, archMatches, osMatches []string) bool {
	n := strings.ToLower(artifactName)

	// Check architecture match
	archFound := false
	for _, arch := range archMatches {
		if strings.Contains(n, arch) {
			archFound = true
			break
		}
	}
	if !archFound {
		return false
	}

	// Check OS match
	osFound := false
	for _, os := range osMatches {
		if strings.Contains(n, os) {
			osFound = true
			break
		}
	}

	return osFound
}

// isArchiveFile checks if the filename is a supported archive format.
func isArchiveFile(filename string) bool {
	name := strings.ToLower(filename)

	// Supported archive extensions based on kvs.ExtractArchive
	supportedExtensions := []string{
		".tar.gz", ".tgz",
		".tar.bz2", ".tbz2",
		".tar.xz", ".txz",
		".tar",
		".zip",
	}

	for _, ext := range supportedExtensions {
		if strings.HasSuffix(name, ext) {
			return true
		}
	}

	return false
}
