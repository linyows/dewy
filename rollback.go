package dewy

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strconv"
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
func (d *Dewy) serverHealthURL() string {
	if d.config.Starter == nil {
		return ""
	}
	ports := d.config.Starter.Ports()
	if len(ports) == 0 {
		return ""
	}

	port := ports[0]

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
// startServer returns an error when server-starter cannot be created at all
// (an occupied port, for instance), and restartServer would then send SIGHUP
// to a process with no starter loop to receive it, leaving nothing running
// while reporting a successful rollback.
func (d *Dewy) restoreServer(ctx context.Context) error {
	d.RLock()
	running := d.isServerRunning
	d.RUnlock()

	if !running {
		return d.startServer()
	}
	if err := d.restartServer(); err != nil {
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

	if err := d.startServer(); err != nil {
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

// starterStatus reports the worker generations server-starter currently has
// alive. It writes one "<generation>:<pid>" line per worker: the current one
// plus any old workers it has not stopped yet.
//
// A missing or half-written file yields no generations rather than an error;
// the caller polls, so a truncated read is retried on the next pass.
func starterStatus(path string) []int {
	if path == "" {
		return nil
	}
	data, err := os.ReadFile(path) //nolint:gosec // G304: the path is dewy's own status file
	if err != nil {
		return nil
	}

	var generations []int
	for line := range strings.SplitSeq(string(data), "\n") {
		gen, _, ok := strings.Cut(strings.TrimSpace(line), ":")
		if !ok {
			continue
		}
		n, err := strconv.Atoi(gen)
		if err != nil {
			continue
		}
		generations = append(generations, n)
	}
	return generations
}

// latestGeneration returns the highest worker generation server-starter
// reports, or -1 when it reports none.
func (d *Dewy) latestGeneration() int {
	latest := -1
	for _, gen := range starterStatus(d.starterStatusFile()) {
		if gen > latest {
			latest = gen
		}
	}
	return latest
}

// starterStatusFile returns the path the managed server-starter writes its
// worker status to, or an empty string when none is configured.
func (d *Dewy) starterStatusFile() string {
	if d.config.Starter == nil {
		return ""
	}
	return d.config.Starter.StatusFile()
}

// awaitWorkerSwap waits until server-starter reports exactly one worker and
// its generation is newer than before.
//
// Without it the health check is meaningless on a restart. server-starter
// answers SIGHUP by spawning a new worker beside the old one, both sharing the
// inherited listening socket, and only stops the old one once the new one has
// survived its startup window. A probe sent in that window is as likely to be
// answered by the release being replaced as by the new one, so a release that
// never came up would still be judged healthy.
//
// It reports whether the swap was observed. A false return means the status
// file never reached that state - it is not configured, server-starter is not
// writing it, or an old worker is ignoring its stop signal - and the caller
// probes anyway rather than failing a deploy over a missing status file.
func (d *Dewy) awaitWorkerSwap(ctx context.Context, before int, budget time.Duration) bool {
	if d.starterStatusFile() == "" {
		return false
	}

	deadline, cancel := context.WithTimeout(ctx, budget)
	defer cancel()

	for {
		generations := starterStatus(d.starterStatusFile())
		if len(generations) == 1 && generations[0] > before {
			d.logger.Debug("Worker swap complete",
				slog.Int("generation", generations[0]))
			return true
		}

		select {
		case <-deadline.Done():
			d.logger.Warn("Timed out waiting for the worker swap; the health check may reach the previous release",
				slog.Int("previous_generation", before),
				slog.Any("live_generations", generations),
				slog.Duration("waited", budget))
			return false
		case <-time.After(workerSwapPollInterval):
		}
	}
}
