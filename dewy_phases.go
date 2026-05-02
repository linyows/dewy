package dewy

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/linyows/dewy/artifact"
	"github.com/linyows/dewy/container"
	"github.com/linyows/dewy/registry"
)

// makeRunContext is the single point at which Run / RunContainer derive their
// per-tick context. Centralized so future PRs can swap the parent (e.g. to
// thread Start's context through, or to add deadlines).
func (d *Dewy) makeRunContext() (context.Context, context.CancelFunc) {
	return context.WithCancel(context.Background())
}

// ----- server/assets path phases ------------------------------------------------

// resolveCurrent fetches the latest from the registry and applies the two
// "skip without error" filters (artifact-not-found grace period and slot
// mismatch). A (nil, nil) return means "skip this tick"; the caller should
// return nil up to Start so the scheduler does not surface a false error.
func (d *Dewy) resolveCurrent(ctx context.Context) (*registry.CurrentResponse, error) {
	res, err := d.registry.Current(ctx)
	if err != nil {
		// Within grace period (e.g. CI is still uploading the artifact)
		// we return (nil, nil) to suppress the alert.
		var artifactNotFoundErr *registry.ArtifactNotFoundError
		if errors.As(err, &artifactNotFoundErr) {
			gracePeriod := 30 * time.Minute
			if artifactNotFoundErr.IsWithinGracePeriod(gracePeriod) {
				d.logger.Debug("Artifact not found within grace period",
					slog.String("message", artifactNotFoundErr.Message),
					slog.Duration("grace_period", gracePeriod))
				return nil, nil
			}
		}
		d.logger.Error("Current failure", slog.String("error", err.Error()))
		return nil, err
	}

	if !(registry.SlotMatcher{Expected: d.config.Slot}).Matches(res.Slot) {
		d.logger.Debug("Deploy skipped: slot mismatch",
			slog.String("expected_slot", d.config.Slot),
			slog.String("actual_slot", res.Slot),
			slog.String("tag", res.Tag))
		return nil, nil
	}

	return res, nil
}

// cacheState carries Run state across phases.
type cacheState struct {
	key          string
	foundInCache bool
	skip         bool // true means deploy is a no-op for this tick
}

// resolveCacheState inspects the local cache to decide whether the artifact
// for res is already staged and whether a redeploy can be skipped entirely.
//
// Skip semantics match the original Run() body:
//   - SERVER + cached version matches + server is running -> skip
//   - ASSETS + cached version matches -> skip
//   - SERVER + cached version matches + server NOT running (crashed/booting) ->
//     fall through to redeploy from cache (foundInCache stays true).
func (d *Dewy) resolveCacheState(_ context.Context, res *registry.CurrentResponse) (cacheState, error) {
	st := cacheState{key: d.cachekeyName(res)}

	currentkeyValue, _ := d.cache.Read(currentkeyName)
	list, err := d.cache.List()
	if err != nil {
		return st, err
	}

	for _, key := range list {
		if key != st.key {
			continue
		}
		st.foundInCache = true

		if string(currentkeyValue) == st.key {
			switch d.config.Command {
			case SERVER:
				d.RLock()
				running := d.isServerRunning
				d.RUnlock()
				if running {
					d.logger.Debug("Deploy skipped")
					st.skip = true
					return st, nil
				}
				// Server is down: fall through to redeploy from cache.
			case ASSETS:
				d.logger.Debug("Deploy skipped")
				st.skip = true
				return st, nil
			}
		} else {
			// Take ownership of the current pointer.
			if err := d.cache.Write(currentkeyName, []byte(st.key)); err != nil {
				return st, err
			}
		}

		// Ensure the artifact bytes are present in local staging for
		// ExtractArchive. Cloud backends populate the local stage on Read;
		// the file backend reads from disk where it already lives.
		if _, err := d.cache.Read(st.key); err != nil {
			return st, fmt.Errorf("failed to load cached artifact: %w", err)
		}
		break
	}

	return st, nil
}

// downloadAndCache fetches the artifact bytes from upstream and writes them
// to the cache. No-op when the artifact is already staged locally.
func (d *Dewy) downloadAndCache(ctx context.Context, res *registry.CurrentResponse, st cacheState) error {
	if st.foundInCache {
		return nil
	}

	buf := new(bytes.Buffer)
	if d.artifact == nil {
		a, err := artifact.New(ctx, res.ArtifactURL, d.logger.Slog())
		if err != nil {
			return fmt.Errorf("failed artifact.New: %w", err)
		}
		d.artifact = a
	}
	err := d.artifact.Download(ctx, &limitedWriter{W: buf, N: MaxArtifactSize})
	d.artifact = nil
	if err != nil {
		return fmt.Errorf("failed artifact.Download: %w", err)
	}

	if err := d.cache.Write(st.key, buf.Bytes()); err != nil {
		return fmt.Errorf("failed cache.Write cachekeyName: %w", err)
	}
	if err := d.cache.Write(currentkeyName, []byte(st.key)); err != nil {
		return fmt.Errorf("failed cache.Write currentkeyName: %w", err)
	}
	d.logger.Info("Cached artifact", slog.String("cache_key", st.key))
	return nil
}

// applyDeployment sends the "downloaded" notification and runs the deploy
// lifecycle (before-hook + extract + symlink swap + after-hook lives inside
// d.deploy).
func (d *Dewy) applyDeployment(ctx context.Context, res *registry.CurrentResponse, key string) error {
	msg := fmt.Sprintf("Downloaded artifact for `%s`", res.Tag)
	d.logger.Info("Download notification", slog.String("message", msg))
	d.notifier.Send(ctx, msg)

	return d.deploy(key)
}

// promoteAndReport finalizes a server/assets deploy: saves the version,
// (re)starts the server for SERVER mode, reports to the registry, and prunes
// old releases. Errors from Report and keepReleases are logged but do not
// cause the run to fail, matching the original behavior.
func (d *Dewy) promoteAndReport(ctx context.Context, res *registry.CurrentResponse) error {
	d.Lock()
	d.cVer = res.Tag
	d.Unlock()

	if d.config.Command == SERVER {
		if err := d.startOrRestartServer(ctx); err != nil {
			return err
		}
	}

	d.reportDeployment(ctx, res)

	d.logger.Info("Keep releases", slog.Int("count", keepReleases))
	if err := d.keepReleases(); err != nil {
		d.logger.Error("Keep releases failure", slog.String("error", err.Error()))
	}

	return nil
}

// startOrRestartServer brings the local server process up: starts it if it
// is down, restarts it if it is already running. Notifications are sent on
// success only — the caller surfaces the error.
func (d *Dewy) startOrRestartServer(ctx context.Context) error {
	d.RLock()
	running := d.isServerRunning
	d.RUnlock()

	var err error
	if running {
		err = d.restartServer()
		if err == nil {
			msg := fmt.Sprintf("Server restarted for `%s`", d.cVer)
			if len(d.config.Starter.Ports()) == 0 {
				msg += " without port"
			}
			d.logger.Info("Restart notification", slog.String("message", msg))
			d.notifier.SendImportant(ctx, msg)
		}
	} else {
		err = d.startServer()
		if err == nil {
			msg := fmt.Sprintf("Server started for `%s`", d.cVer)
			if len(d.config.Starter.Ports()) == 0 {
				msg += " without port"
			}
			d.logger.Info("Start notification", slog.String("message", msg))
			d.notifier.SendImportant(ctx, msg)
		}
	}
	if err != nil {
		d.logger.Error("Server failure", slog.String("error", err.Error()))
		return err
	}
	return nil
}

// reportDeployment reports the deployment to the registry unless the dewy
// instance has report shipping disabled. Errors are logged but not returned.
func (d *Dewy) reportDeployment(ctx context.Context, res *registry.CurrentResponse) {
	if d.disableReport {
		return
	}
	d.logger.Debug("Report shipping")
	if err := d.registry.Report(ctx, &registry.ReportRequest{
		ID:      res.ID,
		Tag:     res.Tag,
		Command: d.config.Command.String(),
	}); err != nil {
		d.logger.Error("Report shipping failure", slog.String("error", err.Error()))
	}
}

// ----- container path phases ----------------------------------------------------

// resolveContainerCurrent fetches the latest image and applies slot
// filtering. Unlike resolveCurrent, the container path does not apply the
// artifact-not-found grace period: OCI registries do not surface
// ArtifactNotFoundError with a publish time.
func (d *Dewy) resolveContainerCurrent(ctx context.Context) (*registry.CurrentResponse, error) {
	res, err := d.registry.Current(ctx)
	if err != nil {
		d.logger.Error("Failed to get current image", slog.String("error", err.Error()))
		return nil, err
	}

	d.logger.Debug("Found latest image",
		slog.String("tag", res.Tag),
		slog.String("digest", res.ID),
		slog.String("url", res.ArtifactURL),
		slog.String("slot", res.Slot))

	if !(registry.SlotMatcher{Expected: d.config.Slot}).Matches(res.Slot) {
		d.logger.Debug("Deploy skipped: slot mismatch",
			slog.String("expected_slot", d.config.Slot),
			slog.String("actual_slot", res.Slot),
			slog.String("tag", res.Tag))
		return nil, nil
	}

	return res, nil
}

// containerState carries RunContainer state across phases.
type containerState struct {
	imageRef string
	appName  string
	runtime  *container.Runtime
	skip     bool // image is already running; deploy is a no-op
}

// resolveContainerState parses the artifact URL into an image reference,
// brings up a container.Runtime, and checks whether the requested image is
// already running. skip=true short-circuits the rest of the run.
func (d *Dewy) resolveContainerState(ctx context.Context, res *registry.CurrentResponse) (containerState, error) {
	st := containerState{
		imageRef: strings.TrimPrefix(res.ArtifactURL, "img://"),
	}

	st.appName = d.config.Container.Name
	if st.appName == "" {
		// Use repository name as app name.
		parts := strings.Split(st.imageRef, "/")
		if len(parts) > 0 {
			lastPart := parts[len(parts)-1]
			st.appName = strings.Split(lastPart, ":")[0]
		}
	}

	rt, err := container.New(d.config.Container.Runtime, d.logger.Slog(), d.config.Container.DrainTime)
	if err != nil {
		return st, fmt.Errorf("failed to create container runtime: %w", err)
	}
	st.runtime = rt
	d.containerRuntime = rt

	runningID, err := rt.GetRunningContainerWithImage(ctx, st.imageRef, st.appName)
	if err != nil {
		// Continue with deployment even if the running-check fails.
		d.logger.Warn("Failed to check running containers", slog.String("error", err.Error()))
		return st, nil
	}
	if runningID != "" {
		d.logger.Debug("Container with this image is already running, skipping deployment",
			slog.String("version", res.Tag),
			slog.String("container", runningID))
		st.skip = true
	}
	return st, nil
}

// pullContainerImage pulls the OCI image via the runtime-backed artifact and
// notifies on success. The runtime in st must be non-nil.
func (d *Dewy) pullContainerImage(ctx context.Context, res *registry.CurrentResponse, st containerState) error {
	if d.artifact == nil {
		a, err := artifact.New(ctx, res.ArtifactURL, d.logger.Slog(), artifact.WithPuller(st.runtime))
		if err != nil {
			return fmt.Errorf("failed artifact.New: %w", err)
		}
		d.artifact = a
	}

	buf := new(bytes.Buffer)
	err := d.artifact.Download(ctx, buf)
	d.artifact = nil
	if err != nil {
		return fmt.Errorf("failed to pull image: %w", err)
	}

	msg := fmt.Sprintf("Pulled image for `%s`", res.Tag)
	d.logger.Info("Pull notification", slog.String("message", msg))
	d.notifier.Send(ctx, msg)
	return nil
}

// applyContainerDeployment runs the before-hook and the rolling deployment,
// records telemetry, and returns the number of replicas successfully
// deployed. The after-hook runs in promoteContainerAndReport so it only fires
// once the deploy is considered final.
func (d *Dewy) applyContainerDeployment(ctx context.Context, res *registry.CurrentResponse) (int, error) {
	beforeResult, beforeErr := d.execHook(d.config.BeforeDeployHook)
	if beforeResult != nil {
		d.notifier.SendHookResult(ctx, "Before Deploy", beforeResult)
	}
	if beforeErr != nil {
		d.logger.Error("Before deploy hook failure", slog.String("error", beforeErr.Error()))
	}

	deployStart := time.Now()
	deployedCount, err := d.deployContainer(ctx, res)
	if err != nil {
		d.logger.Error("Container deployment failed",
			slog.Int("deployed", deployedCount),
			slog.String("error", err.Error()))
		if d.telemetry != nil && d.telemetry.Enabled() {
			d.telemetry.Metrics().DeploymentErrors.Add(ctx, 1)
		}
		return deployedCount, err
	}
	if d.telemetry != nil && d.telemetry.Enabled() {
		m := d.telemetry.Metrics()
		m.DeploymentsTotal.Add(ctx, 1)
		m.DeploymentDuration.Record(ctx, time.Since(deployStart).Seconds())
	}
	return deployedCount, nil
}

// promoteContainerAndReport finalizes a container deploy: saves cVer, runs
// the after-hook, reports to the registry, sends the success notification,
// and prunes old images. Failures of the post-deploy steps are logged but
// not returned, matching the original behavior.
func (d *Dewy) promoteContainerAndReport(ctx context.Context, res *registry.CurrentResponse, deployedCount int, imageRef string) error {
	d.Lock()
	d.cVer = res.Tag
	d.Unlock()

	afterResult, afterErr := d.execHook(d.config.AfterDeployHook)
	if afterResult != nil {
		d.notifier.SendHookResult(ctx, "After Deploy", afterResult)
	}
	if afterErr != nil {
		d.logger.Error("After deploy hook failure", slog.String("error", afterErr.Error()))
	}

	d.reportDeployment(ctx, res)

	totalReplicas := d.config.Container.Replicas
	if totalReplicas <= 0 {
		totalReplicas = 1
	}
	msg := fmt.Sprintf("Container deployed successfully: `%d/%d` replicas of `%s`", deployedCount, totalReplicas, d.cVer)
	d.logger.Info("Container deployed successfully",
		slog.String("version", d.cVer),
		slog.Int("replicas", deployedCount),
		slog.Int("total", totalReplicas))
	d.notifier.SendImportant(ctx, msg)

	d.logger.Info("Keep images", slog.Int("count", keepReleases))
	if err := d.cleanupOldImages(ctx, imageRef); err != nil {
		d.logger.Error("Keep images failure", slog.String("error", err.Error()))
	}
	return nil
}
