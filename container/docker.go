package container

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os/exec"
	"strings"
	"time"
)

// Docker implements Runtime interface using Docker CLI.
type Docker struct {
	cmd       string
	logger    *slog.Logger
	drainTime time.Duration
}

// NewDocker creates a new Docker runtime.
func NewDocker(logger *slog.Logger, drainTime time.Duration) (*Docker, error) {
	cmd := "docker"
	if _, err := exec.LookPath(cmd); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrRuntimeNotFound, err)
	}

	return &Docker{
		cmd:       cmd,
		logger:    logger,
		drainTime: drainTime,
	}, nil
}

// execCommand executes a docker command without returning output.
func (d *Docker) execCommand(ctx context.Context, args ...string) error {
	cmd := exec.CommandContext(ctx, d.cmd, args...)
	d.logger.Debug("Executing docker command",
		slog.String("cmd", d.cmd),
		slog.Any("args", args))

	output, err := cmd.CombinedOutput()
	if err != nil {
		d.logger.Error("Docker command failed",
			slog.String("cmd", d.cmd),
			slog.Any("args", args),
			slog.String("output", string(output)),
			slog.String("error", err.Error()))
		return fmt.Errorf("docker %s failed: %w: %s",
			strings.Join(args, " "), err, string(output))
	}

	return nil
}

// execCommandOutput executes a docker command and returns the output.
func (d *Docker) execCommandOutput(ctx context.Context, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, d.cmd, args...)
	d.logger.Debug("Executing docker command",
		slog.String("cmd", d.cmd),
		slog.Any("args", args))

	output, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			d.logger.Error("Docker command failed",
				slog.String("cmd", d.cmd),
				slog.Any("args", args),
				slog.String("stderr", string(exitErr.Stderr)),
				slog.String("error", err.Error()))
			return "", fmt.Errorf("docker %s failed: %w: %s",
				strings.Join(args, " "), err, string(exitErr.Stderr))
		}
		return "", err
	}

	return strings.TrimSpace(string(output)), nil
}

// Pull pulls an image from the registry.
// If the image already exists locally, it will still attempt to pull to get the latest version.
func (d *Docker) Pull(ctx context.Context, imageRef string) error {
	// Check if image exists locally first
	_, err := d.execCommandOutput(ctx, "image", "inspect", imageRef)
	if err == nil {
		// Image exists locally
		d.logger.Info("Image already exists locally, pulling to check for updates",
			slog.String("image", imageRef))
	}

	// Always try to pull to get the latest version
	// For local-only images (not in a registry), this will fail but that's expected
	d.logger.Info("Pulling image", slog.String("image", imageRef))
	pullErr := d.execCommand(ctx, "pull", imageRef)

	// If pull fails but image exists locally, we can use the local image
	if pullErr != nil && err == nil {
		d.logger.Warn("Failed to pull image, but local image exists - using local version",
			slog.String("image", imageRef),
			slog.String("pull_error", pullErr.Error()))
		return nil // Use local image
	}

	return pullErr
}

// Run starts a new container and returns the container ID.
func (d *Docker) Run(ctx context.Context, opts RunOptions) (string, error) {
	args := []string{"run"}

	if opts.Detach {
		args = append(args, "-d")
	}

	if opts.Name != "" {
		args = append(args, "--name", opts.Name)
	}

	if opts.Network != "" {
		args = append(args, "--network", opts.Network)
		if opts.NetworkAlias != "" {
			args = append(args, "--network-alias", opts.NetworkAlias)
		}
	}

	// Environment variables
	for _, env := range opts.Env {
		args = append(args, "-e", env)
	}

	// Volumes
	for _, vol := range opts.Volumes {
		args = append(args, "-v", vol)
	}

	// Ports
	for _, port := range opts.Ports {
		args = append(args, "-p", port)
	}

	// Labels
	for key, value := range opts.Labels {
		args = append(args, "--label", fmt.Sprintf("%s=%s", key, value))
	}

	args = append(args, opts.Image)

	d.logger.Info("Starting container",
		slog.String("name", opts.Name),
		slog.String("image", opts.Image))

	containerID, err := d.execCommandOutput(ctx, args...)
	if err != nil {
		return "", err
	}

	return containerID, nil
}

// Stop stops a running container gracefully.
func (d *Docker) Stop(ctx context.Context, containerID string, timeout time.Duration) error {
	d.logger.Info("Stopping container gracefully",
		slog.String("container", containerID),
		slog.Duration("timeout", timeout))

	timeoutSec := int(timeout.Seconds())
	return d.execCommand(ctx, "stop", fmt.Sprintf("--time=%d", timeoutSec), containerID)
}

// Remove removes a container.
func (d *Docker) Remove(ctx context.Context, containerID string) error {
	d.logger.Info("Removing container", slog.String("container", containerID))
	return d.execCommand(ctx, "rm", containerID)
}

// NetworkConnect connects a container to a network with an alias.
func (d *Docker) NetworkConnect(ctx context.Context, network, containerID, alias string) error {
	args := []string{"network", "connect"}

	if alias != "" {
		args = append(args, "--alias", alias)
	}

	args = append(args, network, containerID)

	d.logger.Info("Connecting container to network",
		slog.String("container", containerID),
		slog.String("network", network),
		slog.String("alias", alias))

	return d.execCommand(ctx, args...)
}

// NetworkDisconnect disconnects a container from a network.
func (d *Docker) NetworkDisconnect(ctx context.Context, network, containerID string) error {
	d.logger.Info("Disconnecting container from network",
		slog.String("container", containerID),
		slog.String("network", network))

	// Ignore errors (container may already be disconnected)
	err := d.execCommand(ctx, "network", "disconnect", network, containerID)
	if err != nil {
		d.logger.Warn("Failed to disconnect container, may already be disconnected",
			slog.String("error", err.Error()))
	}
	return nil
}

// FindContainerByLabel finds a container by labels.
func (d *Docker) FindContainerByLabel(ctx context.Context, labels map[string]string) (string, error) {
	args := []string{"ps", "-q"}

	for key, value := range labels {
		args = append(args, "--filter", fmt.Sprintf("label=%s=%s", key, value))
	}

	output, err := d.execCommandOutput(ctx, args...)
	if err != nil {
		return "", err
	}

	if output == "" {
		return "", ErrContainerNotFound
	}

	// If multiple containers are found, return the first one
	lines := strings.Split(output, "\n")
	return lines[0], nil
}

// GetContainerIP gets the IP address of a container in a specific network.
func (d *Docker) GetContainerIP(ctx context.Context, containerID, network string) (string, error) {
	// Use index function to handle network names with special characters (like hyphens)
	format := fmt.Sprintf("{{(index .NetworkSettings.Networks \"%s\").IPAddress}}", network)
	output, err := d.execCommandOutput(ctx, "inspect", "--format", format, containerID)
	if err != nil {
		return "", fmt.Errorf("failed to get container IP: %w", err)
	}

	if output == "" {
		return "", fmt.Errorf("container not connected to network %s", network)
	}

	return output, nil
}

// UpdateLabel updates a container's label.
func (d *Docker) UpdateLabel(ctx context.Context, containerID, key, value string) error {
	d.logger.Debug("Label update skipped (not supported by Docker)",
		slog.String("container", containerID),
		slog.String("key", key),
		slog.String("value", value))

	// Docker doesn't support updating labels on running containers
	// Labels are immutable after container creation
	return nil
}

// DeployContainer performs Blue-Green deployment.
func (d *Docker) DeployContainer(ctx context.Context, opts DeployOptions) error {
	// 1. Pull new image
	d.logger.Info("Pulling new image", slog.String("image", opts.ImageRef))
	if err := d.Pull(ctx, opts.ImageRef); err != nil {
		return fmt.Errorf("pull failed: %w", err)
	}

	// 2. Find current container (Blue)
	currentID, err := d.FindContainerByLabel(ctx, map[string]string{
		"dewy.role": "current",
		"dewy.app":  opts.AppName,
	})
	if err != nil && !errors.Is(err, ErrContainerNotFound) {
		return err
	}

	// 3. Start new container (Green)
	greenName := fmt.Sprintf("%s-green-%d", opts.AppName, time.Now().Unix())
	// Start with base labels
	// Since Docker doesn't support updating labels after creation,
	// we set role="current" from the start. The old container will be removed.
	labels := map[string]string{
		"dewy.role": "current",
		"dewy.app":  opts.AppName,
	}

	// Add version label if we can extract it from image ref
	if strings.Contains(opts.ImageRef, ":") {
		parts := strings.Split(opts.ImageRef, ":")
		labels["dewy.version"] = parts[len(parts)-1]
	}

	// For initial deployment, set network and alias at start time
	// For Blue-Green, don't connect to network yet (will connect after health check)
	network := ""
	networkAlias := ""
	ports := opts.Ports
	if currentID == "" {
		// Initial deployment
		network = opts.Network
		networkAlias = opts.NetworkAlias
	} else {
		// For Blue-Green deployment, don't connect to network or map ports yet
		// The new container will be connected to network after health check
		ports = nil
	}

	greenID, err := d.Run(ctx, RunOptions{
		Image:        opts.ImageRef,
		Name:         greenName,
		Network:      network,
		NetworkAlias: networkAlias,
		Env:          opts.Env,
		Volumes:      opts.Volumes,
		Ports:        ports,
		Labels:       labels,
		Detach:       true,
	})
	if err != nil {
		return fmt.Errorf("start green container failed: %w", err)
	}

	// 4. For Blue-Green deployment, connect to network first (without alias) for health checking
	if currentID != "" {
		d.logger.Info("Connecting green container to network for health check")
		if err := d.NetworkConnect(ctx, opts.Network, greenID, ""); err != nil {
			d.Remove(ctx, greenID)
			return fmt.Errorf("network connect failed: %w", err)
		}
	}

	// 5. Health check
	if opts.HealthCheck != nil {
		// Give the container a moment to fully start up before health checking
		// This is especially important for Blue-Green deployment where the container
		// is connected to the network but needs time to start the application
		d.logger.Info("Waiting for container to start...")
		time.Sleep(3 * time.Second)

		d.logger.Info("Health checking new container")
		if err := opts.HealthCheck(ctx, greenID); err != nil {
			d.logger.Error("Health check failed, rolling back")
			if currentID != "" {
				d.NetworkDisconnect(ctx, opts.Network, greenID)
			}
			d.Stop(ctx, greenID, 5*time.Second)
			d.Remove(ctx, greenID)
			return fmt.Errorf("health check failed: %w", err)
		}
	}

	// 6. For Blue-Green deployment, add network alias after health check
	// For initial deployment, alias was already set during Run
	if currentID != "" {
		d.logger.Info("Adding network alias to green container")
		// Disconnect and reconnect with alias
		d.NetworkDisconnect(ctx, opts.Network, greenID)
		if err := d.NetworkConnect(ctx, opts.Network, greenID, opts.NetworkAlias); err != nil {
			// Rollback
			d.Stop(ctx, greenID, 5*time.Second)
			d.Remove(ctx, greenID)
			return fmt.Errorf("network alias failed: %w", err)
		}
	}

	// 7. Remove blue from network (no new requests will come)
	if currentID != "" {
		d.logger.Info("Removing old container from network")
		d.NetworkDisconnect(ctx, opts.Network, currentID)
	}

	// 8. Drain period: wait for existing connections to complete
	d.logger.Info("Waiting for drain period", slog.Duration("duration", d.drainTime))
	select {
	case <-time.After(d.drainTime):
		// Normal completion
	case <-ctx.Done():
		return ctx.Err()
	}

	// 9. Update green label to "current"
	d.UpdateLabel(ctx, greenID, "dewy.role", "current")

	// 10. Stop and remove old container gracefully
	if currentID != "" {
		d.logger.Info("Stopping old container gracefully")
		d.Stop(ctx, currentID, 30*time.Second)
		d.Remove(ctx, currentID)
	}

	d.logger.Info("Deployment completed successfully", slog.String("container", greenID))
	return nil
}
