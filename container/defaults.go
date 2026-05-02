package container

import "time"

// Defaults for container-runtime timing. Kept in one place so operators
// auditing the rolling-deploy cadence do not have to grep for time literals
// scattered across runtime.go.
const (
	// defaultStartupGrace is the pause between starting a container and
	// running its first health probe. Gives the application time to bind
	// its port and begin serving without us misclassifying boot latency
	// as failure.
	defaultStartupGrace = 3 * time.Second

	// defaultStopTimeoutOld is how long we wait for an old (pre-deploy)
	// container to drain before sending it KILL. Pairs with the
	// drain-time setting and is intentionally generous.
	defaultStopTimeoutOld = 10 * time.Second

	// defaultStopTimeoutFailed is how long we wait for a container that
	// failed health checks or was caught up in a rollback to stop. Shorter
	// than the old-container value because the container has not been
	// taking traffic.
	defaultStopTimeoutFailed = 5 * time.Second
)
