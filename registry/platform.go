package registry

import "runtime"

var (
	// TestArch overrides runtime.GOARCH when set (for testing)
	TestArch string
	// TestOS overrides runtime.GOOS when set (for testing)
	TestOS string
)

// getArch returns TestArch if set, otherwise runtime.GOARCH
func getArch() string {
	if TestArch != "" {
		return TestArch
	}
	return runtime.GOARCH
}

// getOS returns TestOS if set, otherwise runtime.GOOS
func getOS() string {
	if TestOS != "" {
		return TestOS
	}
	return runtime.GOOS
}