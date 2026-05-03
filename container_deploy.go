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
		Replicas:     d.config.Container.Replicas,
		PortMappings: resolvedMappings,
		Command:      d.config.Container.Command,
		ExtraArgs:    d.config.Container.ExtraArgs,
		HealthCheck:  healthCheck,
	}, (*proxyBackendUpdater)(d))
	if err != nil {
		return 0, err
	}

	// Record telemetry: net change = new containers - removed containers
	if d.telemetry != nil && d.telemetry.Enabled() {
		delta := int64(len(report.Results)) - int64(report.RemovedCount)
		d.telemetry.Metrics().ContainerReplicas.Add(ctx, delta)
	}

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
		client := &http.Client{Timeout: defaultHealthCheckTimeout}

		retries := defaultHealthCheckRetries
		for i := range retries {
			if d.telemetry != nil && d.telemetry.Enabled() {
				d.telemetry.Metrics().HealthChecksTotal.Add(ctx, 1)
			}
			resp, err := client.Get(healthURL)
			if err == nil {
				resp.Body.Close()
				if resp.StatusCode >= 200 && resp.StatusCode < 400 {
					d.logger.Debug("Health check passed",
						slog.String("url", healthURL),
						slog.Int("status", resp.StatusCode))
					return nil
				}
			}
			if d.telemetry != nil && d.telemetry.Enabled() {
				d.telemetry.Metrics().HealthCheckFailures.Add(ctx, 1)
			}
			if i < retries-1 {
				time.Sleep(defaultHealthCheckDelay)
			}
		}
		return fmt.Errorf("health check failed after %d retries", retries)
	}
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
