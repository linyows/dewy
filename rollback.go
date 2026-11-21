package dewy

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/netip"
	"strings"
	"time"

	"github.com/linyows/dewy/registry"
)

// blockedkeyName is the cache key holding the cache key of the release that
// failed its post-deploy health check. While it is set, that exact release is
// skipped on every subsequent tick, so a broken version is downloaded and
// deployed once rather than on every poll.
//
// The marker names a "tag--artifact" cache key, which does not change when the
// same tag is republished with different contents. Three things clear it:
// publishing a different version and seeing it deploy, running without
// --health-path (the marker is only consulted while health verification is
// on), and deleting the "blocked" entry from the cache store.
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

// serverHealthURL builds the probe URL from the lowest configured port and
// --health-path. It returns an empty string when the server runs without a
// port, which is a supported configuration for job workers.
//
// The port list is deduplicated and sorted numerically while the flags are
// parsed, so the first entry is the lowest port number rather than the first
// one given on the command line.
//
// A port carries a "host:port" form when --host was given. The probe goes to
// that address, because a server bound to a specific interface does not
// accept connections on 127.0.0.1. An unspecified address binds every
// interface, so it is probed through the loopback of its family: 127.0.0.1
// for 0.0.0.0 and ::1 for ::. An IPv6 zone is percent-encoded as a URL
// requires ("[fe80::1%25eth0]").
func (d *Dewy) serverHealthURL() string {
	if d.config.Server == nil {
		return ""
	}
	if len(d.config.Server.Addrs) == 0 {
		return ""
	}
	port := d.config.Server.Addrs[0]

	host := "127.0.0.1"
	if h, p, err := net.SplitHostPort(port); err == nil {
		port = p
		if ip, err := netip.ParseAddr(h); err == nil {
			switch {
			case !ip.IsUnspecified():
				host = strings.Replace(ip.String(), "%", "%25", 1)
			case ip.Is6():
				host = "::1"
			}
		}
	}

	return fmt.Sprintf("http://%s%s", net.JoinHostPort(host, port), healthPath(d.config.Health.Path))
}

// healthPath returns --health-path with a leading slash.
func healthPath(path string) string {
	if !strings.HasPrefix(path, "/") {
		return "/" + path
	}
	return path
}

// serverHealthTimeout returns the overall probe budget for the server
// command, honoring --health-timeout.
func (d *Dewy) serverHealthTimeout() time.Duration {
	if d.config.Health.Timeout > 0 {
		return d.config.Health.Timeout
	}
	return defaultHealthCheckTotalTimeout
}

// rollbackServer handles a release that did not come up or did not pass its
// health check. It records the version as blocked so the next tick does not
// deploy it again, then restores the previous release directory and restarts
// the server unless --no-rollback was given.
//
// When the restart itself failed (errNotSwitched), the previous process never
// stopped and still runs the previous release, so restoring the directory is
// enough and the server is not restarted.
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

	notSwitched := errors.Is(cause, errNotSwitched)

	if d.config.Health.NoRollback {
		d.logger.Warn("Rollback disabled by --no-rollback", slog.String("tag", res.Tag))
		if notSwitched {
			d.notifier.SendImportant(ctx, failed+". Rollback is disabled, so the release stays in place while the previous process keeps serving")
			return
		}
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

	if notSwitched {
		d.recordRollback(ctx)
		d.notifier.SendImportant(ctx, failed+fmt.Sprintf(". Rolled back to `%s`; its process never stopped serving", prevTag))
		return
	}

	if err := d.restoreServer(ctx); err != nil {
		d.logger.Error("Server failure during rollback", slog.String("error", err.Error()))
		d.notifier.SendImportant(ctx, failed+fmt.Sprintf(". Rolled back to `%s` but bringing the server up failed: %s", prevTag, err))
		return
	}
	d.recordRollback(ctx)

	d.notifier.SendImportant(ctx, failed+fmt.Sprintf(". Rolled back to `%s`", prevTag))
}

// restoreServer brings the managed server up on the release the rollback just
// restored. A running server is restarted; a server that never came up is
// started.
//
// The distinction matters because a deploy can fail before the server exists:
// startServer returns an error when a port cannot be bound or the application
// exits at once, and restartServer then has no running server to replace, so
// calling it would leave nothing running.
func (d *Dewy) restoreServer(ctx context.Context) error {
	d.RLock()
	running := d.isServerRunning
	d.RUnlock()

	if !running {
		return d.startServer(ctx)
	}
	if err := d.restartServer(ctx); err != nil {
		return err
	}
	d.recordServerRestart(ctx, "rollback")
	return nil
}

// recoverBlockedServer starts the managed server when it is down while a
// version is blocked.
//
// The blocked skip returns before the redeploy-from-cache path that used to
// bring a stopped server back up, so without this a server that dies while a
// version is blocked would never be restarted. The release directory and the
// "current" symlink already point at the release to run - the rollback
// restored them - so only the process is missing.
func (d *Dewy) recoverBlockedServer(ctx context.Context, currentKey string) {
	if d.config.Command != SERVER {
		return
	}

	d.RLock()
	running := d.isServerRunning
	d.RUnlock()
	if running {
		return
	}

	d.Lock()
	if d.cVer == "" {
		d.cVer = tagFromCacheKey(currentKey)
	}
	tag := d.cVer
	d.Unlock()

	d.logger.Warn("Starting the stopped server while a version is blocked",
		slog.String("version", tag))

	if err := d.startServer(ctx); err != nil {
		d.logger.Error("Server failure", slog.String("error", err.Error()))
		d.notifier.SendError(ctx, err)
		return
	}
	d.notifier.SendImportant(ctx, fmt.Sprintf("Server started for `%s` while a newer version is blocked", tag))
}

// clearBlockedVersion removes the blocked marker after a release passes its
// health check, so a host that recovers is not left skipping versions.
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
