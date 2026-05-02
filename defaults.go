package dewy

import "time"

// Defaults for time-bounded behaviors that are not currently exposed via the
// CLI. They live here rather than scattered as inline literals so the
// trade-offs are discoverable in one place.
//
// All values are package-private; expose via Config / CLI only when an
// operator actually has a reason to override.
const (
	// defaultArtifactGracePeriod is the window during which a missing
	// artifact (typically because CI is still uploading after the release
	// was tagged) is treated as "skip this tick" rather than an error.
	defaultArtifactGracePeriod = 30 * time.Minute

	// defaultHealthCheckTimeout is the per-request HTTP timeout used by the
	// container health check probe.
	defaultHealthCheckTimeout = 5 * time.Second

	// defaultHealthCheckRetries is the number of probe attempts before a
	// container is considered unhealthy.
	defaultHealthCheckRetries = 5

	// defaultHealthCheckDelay is the back-off between probe attempts.
	defaultHealthCheckDelay = 2 * time.Second

	// defaultAdminReadHeaderTimeout caps how long the admin HTTP server
	// waits for request headers; mitigates Slowloris.
	defaultAdminReadHeaderTimeout = 5 * time.Second
)
