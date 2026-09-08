package dewy

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"strings"
	"time"

	"github.com/linyows/dewy/registry"
)

// blockedkeyName is the cache key holding the cache key of the release that
// failed its post-deploy health check. While it is set, that exact release is
// skipped on every subsequent tick, so a broken version is downloaded and
// deployed once rather than on every poll. Any other version clears it.
const blockedkeyName = "blocked"

// verifyServerHealth probes the managed server after a start or restart and
// returns an error when it does not answer in time. It is a no-op unless the
// server command is running with --health-path and at least one port.
func (d *Dewy) verifyServerHealth(ctx context.Context) error {
	if d.config.Command != SERVER || d.config.Health.Path == "" {
		return nil
	}

	u := d.serverHealthURL()
	if u == "" {
		d.logger.Warn("Health check skipped: --health-path is set but no port is configured")
		return nil
	}

	d.logger.Debug("Verifying deployment", slog.String("url", u))
	return d.probeHealth(ctx, u, d.serverHealthTimeout())
}

// serverHealthURL builds the probe URL from the first configured port and
// --health-path. It returns an empty string when the server runs without a
// port, which is a supported configuration for job workers.
func (d *Dewy) serverHealthURL() string {
	if d.config.Starter == nil {
		return ""
	}
	ports := d.config.Starter.Ports()
	if len(ports) == 0 {
		return ""
	}

	// A port spec may carry a host part ("127.0.0.1:8000"); the probe always
	// goes to the loopback address, so only the port number is used.
	port := ports[0]
	if _, p, err := net.SplitHostPort(port); err == nil {
		port = p
	}

	path := d.config.Health.Path
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}

	return fmt.Sprintf("http://127.0.0.1:%s%s", port, path)
}

// serverHealthTimeout returns the overall probe budget for the server
// command, honoring --health-timeout.
func (d *Dewy) serverHealthTimeout() time.Duration {
	if d.config.Health.Timeout > 0 {
		return d.config.Health.Timeout
	}
	return defaultHealthCheckTotalTimeout
}

// rollbackServer handles a release that started but did not pass its health
// check. It records the version as blocked so the next tick does not deploy
// it again, then restores the previous release directory and restarts the
// server unless --no-rollback was given.
//
// Every step is best-effort and logged rather than returned: the caller
// already has the failure that triggered the rollback, and reporting a
// secondary error would hide it.
func (d *Dewy) rollbackServer(ctx context.Context, res *registry.CurrentResponse, st cacheState, cause error) {
	d.logger.Error("Deployment verification failed",
		slog.String("tag", res.Tag),
		slog.String("error", cause.Error()))

	if err := d.cache.Write(blockedkeyName, []byte(st.key)); err != nil {
		d.logger.Error("Failed to record the failed version",
			slog.String("cache_key", st.key),
			slog.String("error", err.Error()))
	}

	failed := fmt.Sprintf("Health check failed for `%s`: %s", res.Tag, cause)

	if d.config.Health.NoRollback {
		d.logger.Warn("Rollback disabled by --no-rollback", slog.String("tag", res.Tag))
		d.notifier.SendImportant(ctx, failed+". Rollback is disabled, so the release stays in place")
		return
	}

	if st.prevRelease == "" || st.prevKey == "" || st.prevKey == st.key {
		d.logger.Warn("No previous release to roll back to", slog.String("tag", res.Tag))
		d.notifier.SendImportant(ctx, failed+". There is no previous release to roll back to")
		return
	}

	prevTag := tagFromCacheKey(st.prevKey)
	d.logger.Info("Rolling back",
		slog.String("from", res.Tag),
		slog.String("to", prevTag),
		slog.String("release", st.prevRelease))

	if err := d.swapSymlink(st.prevRelease); err != nil {
		d.logger.Error("Rollback failure", slog.String("error", err.Error()))
		d.notifier.SendImportant(ctx, failed+fmt.Sprintf(". Rollback to `%s` failed: %s", prevTag, err))
		return
	}
	if err := d.cache.Write(currentkeyName, []byte(st.prevKey)); err != nil {
		d.logger.Error("Failed to restore the current cache key", slog.String("error", err.Error()))
	}
	d.notifier.OnDeploy(st.prevRelease)

	d.Lock()
	d.cVer = prevTag
	d.Unlock()

	if err := d.restartServer(); err != nil {
		d.logger.Error("Restart failure during rollback", slog.String("error", err.Error()))
		d.notifier.SendImportant(ctx, failed+fmt.Sprintf(". Rolled back to `%s` but the restart failed: %s", prevTag, err))
		return
	}
	d.recordServerRestart(ctx, "rollback")
	d.recordRollback(ctx)

	d.notifier.SendImportant(ctx, failed+fmt.Sprintf(". Rolled back to `%s`", prevTag))
}

// clearBlockedVersion removes the blocked marker after a release passes its
// health check. Without this, a version that failed once and was later fixed
// under the same tag would stay blocked forever.
func (d *Dewy) clearBlockedVersion() {
	if _, err := d.cache.Read(blockedkeyName); err != nil {
		return
	}
	if err := d.cache.Delete(blockedkeyName); err != nil {
		d.logger.Warn("Failed to clear the blocked version", slog.String("error", err.Error()))
	}
}

// blockedVersion returns the cache key of the release that last failed its
// health check, or an empty string when there is none.
func (d *Dewy) blockedVersion() string {
	v, err := d.cache.Read(blockedkeyName)
	if err != nil {
		return ""
	}
	return string(v)
}

// tagFromCacheKey recovers the tag from a "tag--artifact" cache key.
func tagFromCacheKey(key string) string {
	tag, _, _ := strings.Cut(key, "--")
	return tag
}

// recordRollback counts one automatic rollback.
func (d *Dewy) recordRollback(ctx context.Context) {
	if !d.telemetryOn() {
		return
	}
	d.telemetry.Metrics().DeploymentRollbacks.Add(ctx, 1, d.commandAttr())
}
