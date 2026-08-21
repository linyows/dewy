package dewy

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/linyows/dewy/container"
	"github.com/linyows/dewy/registry"
)

// deployContainer performs the actual container deployment using rolling
// update strategy. The runtime must be the same instance resolved by
// resolveContainerState (and used for the image pull) so login state and
// runtime configuration stay consistent across the per-tick deploy. Returns
// the number of successfully deployed containers and any error encountered.
func (d *Dewy) deployContainer(ctx context.Context, res *registry.CurrentResponse, runtime *container.Runtime) (int, error) {
	if d.config.Container == nil {
		return 0, fmt.Errorf("container config is nil")
	}
	if runtime == nil {
		return 0, fmt.Errorf("container runtime is nil")
	}

	// Extract image reference from artifact URL
	// Format: img://registry/repo:tag
	imageRef := strings.TrimPrefix(res.ArtifactURL, "img://")
	appName := d.appName()

	// Resolve port mappings (auto-detect ContainerPort==0 from image EXPOSE).
	resolvedMappings, err := runtime.ResolvePortMappings(ctx, imageRef, d.config.Container.PortMappings)
	if err != nil {
		return 0, fmt.Errorf("failed to resolve port mappings: %w", err)
	}

	// Create health check function (telemetry-aware, stays in dewy package)
	healthCheck := d.createHealthCheckFunc(runtime, resolvedMappings)

	// Deploy via container runtime, with the dewy proxy as the BackendUpdater.
	report, err := runtime.Deploy(ctx, container.RollingDeployOptions{
		ImageRef:     imageRef,
		AppName:      appName,
		Version:      res.Tag,
		Replicas:     d.config.Container.Replicas,
		PortMappings: resolvedMappings,
		Command:      d.config.Container.Command,
		ExtraArgs:    d.config.Container.ExtraArgs,
		HealthCheck:  healthCheck,
	}, (*proxyBackendUpdater)(d))
	if err != nil {
		return 0, err
	}

	// Reap containers that crashed on their own since the last deploy. The
	// rolling update above only removes the running containers it replaced;
	// without this, exited replicas pile up across deploys. Best-effort — a
	// failure here must not fail an otherwise-successful deploy.
	if n, err := runtime.RemoveExited(ctx, appName); err != nil {
		d.logger.Warn("Failed to reap exited containers", slog.String("error", err.Error()))
	} else if n > 0 {
		d.logger.Info("Reaped exited containers", slog.Int("count", n))
	}

	// Replica counts are reported asynchronously by the container observer
	// (registered in Start), which reads the live runtime state rather than
	// tracking deltas here — a delta counter drifts when a container dies
	// outside a deploy.
	return len(report.Results), nil
}

// createHealthCheckFunc creates a health check function based on configuration.
// Health check is performed on the first port mapping.
func (d *Dewy) createHealthCheckFunc(rt *container.Runtime, resolvedMappings []container.PortMapping) container.HealthCheckFunc {
	if d.config.Container.HealthPath == "" {
		d.logger.Info("Health check disabled - container will start without health verification")
		return nil
	}

	if len(resolvedMappings) == 0 {
		d.logger.Warn("No port mappings configured, health check disabled")
		return nil
	}

	// Use first port mapping for health check
	firstMapping := resolvedMappings[0]

	return func(ctx context.Context, containerID string) error {
		mappedPort, err := rt.GetMappedPort(ctx, containerID, firstMapping.ContainerPort)
		if err != nil {
			return fmt.Errorf("failed to get mapped port for health check: %w", err)
		}

		healthURL := fmt.Sprintf("http://localhost:%d%s", mappedPort, d.config.Container.HealthPath)
		return d.probeHealth(ctx, healthURL, d.healthCheckTimeout())
	}
}

// healthCheckTimeout returns the overall budget for verifying a single
// container, honouring --health-timeout.
func (d *Dewy) healthCheckTimeout() time.Duration {
	if d.config.Container != nil && d.config.Container.HealthTimeout > 0 {
		return d.config.Container.HealthTimeout
	}
	return defaultHealthCheckTotalTimeout
}

// probeHealth polls healthURL until it answers with a success status or the
// budget runs out. The budget covers every attempt including the back-off
// between them, so an operator raising --health-timeout buys more attempts
// rather than a longer wait on a single hung request.
func (d *Dewy) probeHealth(ctx context.Context, healthURL string, budget time.Duration) error {
	deadlineCtx, cancel := context.WithTimeout(ctx, budget)
	defer cancel()

	client := &http.Client{Timeout: defaultHealthCheckProbeTimeout}
	attempts := 0

	for {
		attempts++
		if d.telemetry != nil && d.telemetry.Enabled() {
			d.telemetry.Metrics().HealthChecksTotal.Add(ctx, 1)
		}

		err := probeOnce(deadlineCtx, client, healthURL)
		if err == nil {
			d.logger.Debug("Health check passed",
				slog.String("url", healthURL),
				slog.Int("attempts", attempts))
			return nil
		}

		if d.telemetry != nil && d.telemetry.Enabled() {
			d.telemetry.Metrics().HealthCheckFailures.Add(ctx, 1)
		}
		d.logger.Debug("Health check attempt failed",
			slog.String("url", healthURL),
			slog.Int("attempt", attempts),
			slog.String("error", err.Error()))

		select {
		case <-deadlineCtx.Done():
			return fmt.Errorf("health check failed after %d attempts within %s: %w",
				attempts, budget, err)
		case <-time.After(defaultHealthCheckDelay):
		}
	}
}

// probeOnce performs a single health check request. A 2xx or 3xx response is
// treated as healthy.
func probeOnce(ctx context.Context, client *http.Client, healthURL string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, healthURL, nil)
	if err != nil {
		return err
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 400 {
		return fmt.Errorf("unhealthy status %d", resp.StatusCode)
	}
	return nil
}

// stopManagedContainers stops all containers managed by this dewy instance.
func (d *Dewy) stopManagedContainers(ctx context.Context) error {
	if d.containerRuntime == nil {
		return nil
	}

	d.logger.Info("Stopping managed containers")
	_, _, err := d.containerRuntime.StopManagedContainers(ctx, d.appName())
	return err
}
