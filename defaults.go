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

	// defaultWorkerSwapTimeout bounds how long a server deploy waits for
	// server-starter to retire the previous worker before the health check
	// probes. It is separate from --health-timeout so that raising the probe
	// budget does not also stretch this wait.
	defaultWorkerSwapTimeout = 30 * time.Second

	// workerSwapPollInterval is how often the server-starter status file is
	// re-read while waiting for the swap.
	workerSwapPollInterval = 200 * time.Millisecond

	// defaultProxyIdleTimeout is the default idle timeout for TCP proxy
	// connections. --proxy-idle-timeout overrides it; 0 disables the timeout.
	defaultProxyIdleTimeout = 5 * time.Minute

	// defaultAdminPort is the port the admin API listens on when --admin-port
	// is not given. 17539 has no meaning beyond being unlikely to collide.
	// Both the server (startAdminAPI) and the client ("dewy container list")
	// side of the API must agree on it.
	defaultAdminPort = 17539

	// adminPortMaxAttempts is how many consecutive ports are tried from the
	// configured one. The server increments on bind failure so several dewy
	// instances can share a host; the client scans the same range to find
	// them.
	adminPortMaxAttempts = 10
)
