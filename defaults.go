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

	// defaultHealthCheckProbeTimeout is the per-request HTTP timeout used by
	// the container health check probe. The overall budget across all
	// attempts comes from ContainerConfig.HealthTimeout (--health-timeout).
	defaultHealthCheckProbeTimeout = 5 * time.Second

	// defaultHealthCheckTotalTimeout is the overall health check budget used
	// when ContainerConfig.HealthTimeout is unset. The CLI always sets it, so
	// this only applies to a Config built programmatically.
	defaultHealthCheckTotalTimeout = 30 * time.Second

	// defaultHealthCheckDelay is the back-off between probe attempts.
	defaultHealthCheckDelay = 2 * time.Second

	// defaultAdminReadHeaderTimeout caps how long the admin HTTP server
	// waits for request headers; mitigates Slowloris.
	defaultAdminReadHeaderTimeout = 5 * time.Second
)
