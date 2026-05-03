package container

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"
)

// Deploy performs a rolling deployment of containers.
// It starts new containers one by one, runs health checks, updates backends
// via the BackendUpdater hook, and then removes old containers. Pass nil for
// updater when no proxy interaction is needed.
func (r *Runtime) Deploy(ctx context.Context, opts RollingDeployOptions, updater BackendUpdater) (*DeployReport, error) {
	if updater == nil {
		updater = noopBackendUpdater{}
	}
	replicas := opts.Replicas
	if replicas <= 0 {
		replicas = 1
	}

	r.logger.Info("Starting container deployment",
		slog.Int("replicas", replicas))

	// Find existing containers
	existingContainers, err := r.FindContainersByLabel(ctx, map[string]string{
		"dewy.managed": "true",
		"dewy.app":     opts.AppName,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to find existing containers: %w", err)
	}

	r.logger.Info("Found existing containers",
		slog.Int("count", len(existingContainers)))

	// Rolling update: start new containers one by one
	results := make([]DeployResult, 0, replicas)

	for i := 0; i < replicas; i++ {
		r.logger.Info("Starting new container",
			slog.String("image", opts.ImageRef),
			slog.Int("replica", i+1),
			slog.Int("total", replicas))

		result, err := r.startAndCheck(ctx, opts, i)
		if err != nil {
			r.logger.Error("Failed to start container, rolling back",
				slog.Int("replica", i+1),
				slog.String("error", err.Error()))
			r.rollback(ctx, results, updater)
			return nil, err
		}

		// Add all port mappings to proxy backends
		for proxyPort, mappedPort := range result.MappedPorts {
			if err := updater.AddBackend("localhost", mappedPort, proxyPort); err != nil {
				r.logger.Error("Failed to add proxy backend",
					slog.Int("proxy_port", proxyPort),
					slog.Int("mapped_port", mappedPort),
					slog.String("error", err.Error()))
				// Include current result in rollback to clean up its container and backends
				r.rollback(ctx, append(results, result), updater)
				return nil, err
			}
		}

		results = append(results, result)

		r.logger.Info("Container added to load balancer",
			slog.String("container", result.ContainerID),
			slog.Int("port_mappings", len(result.MappedPorts)))
	}

	// Remove old containers one by one
	removedCount := 0
	for i, oldContainerID := range existingContainers {
		r.logger.Info("Removing old container",
			slog.Int("index", i+1),
			slog.Int("total", len(existingContainers)),
			slog.String("container", oldContainerID))

		// Remove from proxy backends
		for _, mapping := range opts.PortMappings {
			oldPort, err := r.GetMappedPort(ctx, oldContainerID, mapping.ContainerPort)
			if err == nil {
				if err := updater.RemoveBackend("localhost", oldPort, mapping.ProxyPort); err != nil {
					r.logger.Warn("Failed to remove old backend from proxy",
						slog.Int("proxy_port", mapping.ProxyPort),
						slog.Int("mapped_port", oldPort),
						slog.String("error", err.Error()))
				}
			}
		}

		// Stop and remove old container
		if err := r.Stop(ctx, oldContainerID, defaultStopTimeoutOld); err != nil {
			r.logger.Error("Failed to stop old container",
				slog.String("container", oldContainerID),
				slog.String("error", err.Error()))
		}
		if err := r.Remove(ctx, oldContainerID); err != nil {
			r.logger.Error("Failed to remove old container",
				slog.String("container", oldContainerID),
				slog.String("error", err.Error()))
		}
		removedCount++
	}

	r.logger.Info("Container deployment completed",
		slog.Int("new_containers", len(results)),
		slog.Int("removed_containers", removedCount))

	return &DeployReport{
		Results:      results,
		RemovedCount: removedCount,
	}, nil
}

// startAndCheck starts a single container, resolves port mappings, and runs health check.
func (r *Runtime) startAndCheck(ctx context.Context, opts RollingDeployOptions, replicaIndex int) (DeployResult, error) {
	// Prepare port mappings for localhost-only access (deduplicate container ports)
	uniqueContainerPorts := make(map[int]bool)
	var ports []string
	for _, mapping := range opts.PortMappings {
		if !uniqueContainerPorts[mapping.ContainerPort] {
			uniqueContainerPorts[mapping.ContainerPort] = true
			ports = append(ports, fmt.Sprintf("127.0.0.1::%d", mapping.ContainerPort))
		}
	}

	// Start container
	containerID, err := r.Run(ctx, RunOptions{
		Image:        opts.ImageRef,
		AppName:      opts.AppName,
		ReplicaIndex: replicaIndex,
		Ports:        ports,
		Labels: map[string]string{
			"dewy.managed":     "true",
			"dewy.app":         opts.AppName,
			"dewy.deployed_at": time.Now().Format(time.RFC3339),
		},
		Detach:    true,
		Command:   opts.Command,
		ExtraArgs: opts.ExtraArgs,
	})
	if err != nil {
		return DeployResult{}, fmt.Errorf("failed to start container: %w", err)
	}

	// Get all mapped ports (cache to avoid duplicate lookups for same container port)
	containerPortToMapped := make(map[int]int)
	mappedPorts := make(map[int]int) // map[proxyPort]mappedPort
	for _, mapping := range opts.PortMappings {
		if mappedPort, exists := containerPortToMapped[mapping.ContainerPort]; exists {
			mappedPorts[mapping.ProxyPort] = mappedPort
			continue
		}

		mappedPort, err := r.GetMappedPort(ctx, containerID, mapping.ContainerPort)
		if err != nil {
			rErr := r.Remove(ctx, containerID)
			return DeployResult{}, errors.Join(
				fmt.Errorf("failed to get mapped port for container port %d: %w", mapping.ContainerPort, err),
				fmt.Errorf("runtime remove failed: %w", rErr),
			)
		}
		containerPortToMapped[mapping.ContainerPort] = mappedPort
		mappedPorts[mapping.ProxyPort] = mappedPort
	}

	r.logger.Info("Container started",
		slog.String("container", containerID),
		slog.Any("port_mappings", mappedPorts))

	// Perform health check if configured
	if opts.HealthCheck != nil {
		// Give the container a moment to start
		time.Sleep(defaultStartupGrace)

		r.logger.Info("Performing health check", slog.String("container", containerID))
		if err := opts.HealthCheck(ctx, containerID); err != nil {
			sErr := r.Stop(ctx, containerID, defaultStopTimeoutFailed)
			rErr := r.Remove(ctx, containerID)
			return DeployResult{}, errors.Join(
				fmt.Errorf("health check failed: %w", err),
				fmt.Errorf("runtime stop failed: %w", sErr),
				fmt.Errorf("runtime remove failed: %w", rErr),
			)
		}
	}

	return DeployResult{
		ContainerID:  containerID,
		MappedPorts:  mappedPorts,
		ReplicaIndex: replicaIndex,
	}, nil
}

// rollback removes all newly deployed containers and their proxy backends.
// updater is assumed non-nil — callers (Deploy) substitute the noop updater
// before calling.
func (r *Runtime) rollback(ctx context.Context, results []DeployResult, updater BackendUpdater) {
	r.logger.Info("Rolling back containers", slog.Int("count", len(results)))

	// Remove from proxy backends first
	for _, result := range results {
		for proxyPort, mappedPort := range result.MappedPorts {
			if err := updater.RemoveBackend("localhost", mappedPort, proxyPort); err != nil {
				r.logger.Warn("Failed to remove backend during rollback",
					slog.Int("proxy_port", proxyPort),
					slog.Int("mapped_port", mappedPort),
					slog.String("error", err.Error()))
			}
		}
	}

	// Stop and remove containers
	for _, result := range results {
		if err := r.Stop(ctx, result.ContainerID, defaultStopTimeoutFailed); err != nil {
			r.logger.Error("Failed to stop container during rollback",
				slog.String("container", result.ContainerID),
				slog.String("error", err.Error()))
		}
		if err := r.Remove(ctx, result.ContainerID); err != nil {
			r.logger.Error("Failed to remove container during rollback",
				slog.String("container", result.ContainerID),
				slog.String("error", err.Error()))
		}
	}
}

// StopManagedContainers stops and removes all containers with dewy.managed=true and matching app name.
func (r *Runtime) StopManagedContainers(ctx context.Context, appName string) (int, int, error) {
	labels := map[string]string{
		"dewy.managed": "true",
	}
	if appName != "" {
		labels["dewy.app"] = appName
	}

	containerIDs, err := r.FindContainersByLabel(ctx, labels)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to find managed containers: %w", err)
	}

	if len(containerIDs) == 0 {
		r.logger.Debug("No managed containers found to stop")
		return 0, 0, nil
	}

	r.logger.Info("Found managed containers to stop", slog.Int("count", len(containerIDs)))

	timeout := defaultStopTimeoutOld
	stopped := 0
	removed := 0

	for _, containerID := range containerIDs {
		if err := r.Stop(ctx, containerID, timeout); err != nil {
			r.logger.Error("Failed to stop container",
				slog.String("container", containerID),
				slog.String("error", err.Error()))
			continue
		}

		r.logger.Info("Managed container stopped",
			slog.String("container", containerID))
		stopped++

		if err := r.Remove(ctx, containerID); err != nil {
			r.logger.Warn("Failed to remove container",
				slog.String("container", containerID),
				slog.String("error", err.Error()))
		} else {
			r.logger.Info("Managed container removed",
				slog.String("container", containerID))
			removed++
		}
	}

	r.logger.Info("Cleanup completed",
		slog.Int("stopped", stopped),
		slog.Int("removed", removed),
		slog.Int("total", len(containerIDs)))

	return stopped, removed, nil
}

// ResolvePortMappings resolves port mappings by auto-detecting container ports from image EXPOSE.
// ContainerPort == 0 means auto-detect. If auto-detect is needed, the image must expose exactly one port.
func (r *Runtime) ResolvePortMappings(ctx context.Context, imageRef string, mappings []PortMapping) ([]PortMapping, error) {
	if len(mappings) == 0 {
		return nil, fmt.Errorf("no port mappings configured")
	}

	// Check if any mapping needs auto-detection
	needsAutoDetect := false
	for _, mapping := range mappings {
		if mapping.ContainerPort == 0 {
			needsAutoDetect = true
			break
		}
	}

	if !needsAutoDetect {
		r.logger.Debug("All port mappings are explicit",
			slog.Int("count", len(mappings)))
		return mappings, nil
	}

	// Auto-detect exposed ports from image
	exposedPorts, err := r.GetImageExposedPorts(ctx, imageRef)
	if err != nil {
		return nil, fmt.Errorf("failed to detect exposed ports: %w", err)
	}

	r.logger.Info("Detected exposed ports from image",
		slog.String("image", imageRef),
		slog.Any("ports", exposedPorts))

	if len(exposedPorts) == 0 {
		return nil, fmt.Errorf("container does not expose any ports. Please specify port mappings explicitly using --port proxy:container")
	}

	if len(exposedPorts) > 1 {
		return nil, fmt.Errorf("container exposes multiple ports %v. Please specify port mappings explicitly using --port proxy:container", exposedPorts)
	}

	detectedPort := exposedPorts[0]
	resolved := make([]PortMapping, len(mappings))
	for i, mapping := range mappings {
		if mapping.ContainerPort == 0 {
			resolved[i] = PortMapping{
				ProxyPort:     mapping.ProxyPort,
				ContainerPort: detectedPort,
			}
			r.logger.Info("Auto-detected container port for proxy",
				slog.Int("proxy_port", mapping.ProxyPort),
				slog.Int("container_port", detectedPort))
		} else {
			resolved[i] = mapping
		}
	}

	return resolved, nil
}
