package container

import (
	"context"
	"encoding/json"
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

//nolint:godot // Forbidden options that conflict with Dewy management
var forbiddenOptions = []string{
	"-d", "--detach",
	"-it",
	"-i", "--interactive",
	"-t", "--tty",
	"-l", "--label",
	"-p", "--publish",
}

// NewDocker creates a new Docker runtime.
func NewDocker(logger *slog.Logger, drainTime time.Duration) (*Docker, error) {
	cmd := "docker"
	if _, err := exec.LookPath(cmd); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrRuntimeNotFound, err)
	}

	return &Docker{
		cmd:       cmd,
		logger:    logger,
		drainTime: drainTime,
	}, nil
}

// validateExtraArgs checks if any forbidden options are present in extra args.
func validateExtraArgs(args []string) error {
	for _, arg := range args {
		for _, forbidden := range forbiddenOptions {
			if arg == forbidden || strings.HasPrefix(arg, forbidden+"=") {
				return fmt.Errorf("option %s conflicts with Dewy management and cannot be used", forbidden)
			}
		}
	}
	return nil
}

// extractNameOption extracts --name option from args and returns the name and filtered args.
func extractNameOption(args []string) (string, []string) {
	var name string
	filtered := []string{}

	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--name" && i+1 < len(args) {
			name = args[i+1]
			i++ // Skip next argument
		} else if strings.HasPrefix(arg, "--name=") {
			name = arg[7:] // Skip "--name="
		} else {
			filtered = append(filtered, arg)
		}
	}

	return name, filtered
}

// execCommand executes a docker command without returning output.
func (d *Docker) execCommand(ctx context.Context, args ...string) error {
	// #nosec G204 - args are constructed internally from validated inputs
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
	// #nosec G204 - args are constructed internally from validated inputs
	cmd := exec.CommandContext(ctx, d.cmd, args...)
	d.logger.Debug("Executing docker command",
		slog.String("cmd", d.cmd),
		slog.Any("args", args))

	output, err := cmd.Output()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
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
	// Validate extra args first
	if err := validateExtraArgs(opts.ExtraArgs); err != nil {
		return "", err
	}

	// Extract --name from extra args
	userName, filteredArgs := extractNameOption(opts.ExtraArgs)

	// Determine container name
	baseName := opts.AppName
	if userName != "" {
		baseName = userName
	}

	// Generate container name with timestamp and replica index
	timestamp := time.Now().Unix()
	containerName := fmt.Sprintf("%s-%d-%d", baseName, timestamp, opts.ReplicaIndex)

	// Build docker run command
	args := []string{"run"}

	// Always detach
	if opts.Detach {
		args = append(args, "-d")
	}

	// Container name
	args = append(args, "--name", containerName)

	// Labels (managed by Dewy)
	for key, value := range opts.Labels {
		args = append(args, "--label", fmt.Sprintf("%s=%s", key, value))
	}

	// Ports (managed by Dewy)
	for _, port := range opts.Ports {
		args = append(args, "-p", port)
	}

	// User-specified extra args (filtered, --name removed)
	args = append(args, filteredArgs...)

	// Image
	args = append(args, opts.Image)

	// Command and arguments
	args = append(args, opts.Command...)

	d.logger.Info("Starting container",
		slog.String("name", containerName),
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

// FindContainersByLabel finds all containers matching the given labels.
func (d *Docker) FindContainersByLabel(ctx context.Context, labels map[string]string) ([]string, error) {
	args := []string{"ps", "-q"}

	for key, value := range labels {
		args = append(args, "--filter", fmt.Sprintf("label=%s=%s", key, value))
	}

	output, err := d.execCommandOutput(ctx, args...)
	if err != nil {
		return nil, err
	}

	if output == "" {
		return []string{}, nil
	}

	// Split output by newlines and filter empty strings
	lines := strings.Split(output, "\n")
	containers := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			containers = append(containers, line)
		}
	}

	d.logger.Debug("Found containers by label",
		slog.Any("labels", labels),
		slog.Int("count", len(containers)))

	return containers, nil
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

// DeployContainerCallback is called after health check passes but before stopping old container.
type DeployContainerCallback func(containerID string) error

// DeployContainerWithCallback performs Blue-Green deployment with a callback.
func (d *Docker) DeployContainerWithCallback(ctx context.Context, opts DeployOptions, callback DeployContainerCallback) (string, error) {
	return d.deployContainerInternal(ctx, opts, callback)
}

// DeployContainer performs Blue-Green deployment.
func (d *Docker) DeployContainer(ctx context.Context, opts DeployOptions) error {
	_, err := d.deployContainerInternal(ctx, opts, nil)
	return err
}

// deployContainerInternal is the internal implementation of container deployment.
func (d *Docker) deployContainerInternal(ctx context.Context, opts DeployOptions, callback DeployContainerCallback) (string, error) {
	// 1. Pull new image
	d.logger.Info("Pulling new image", slog.String("image", opts.ImageRef))
	if err := d.Pull(ctx, opts.ImageRef); err != nil {
		return "", fmt.Errorf("pull failed: %w", err)
	}

	// 2. Find current container
	currentID, err := d.FindContainerByLabel(ctx, map[string]string{
		"dewy.managed": "true",
		"dewy.app":     opts.AppName,
	})
	if err != nil && !errors.Is(err, ErrContainerNotFound) {
		return "", err
	}

	// 3. Start new container
	labels := map[string]string{
		"dewy.managed": "true",
		"dewy.app":     opts.AppName,
	}

	// Add version label if we can extract it from image ref
	if strings.Contains(opts.ImageRef, ":") {
		parts := strings.Split(opts.ImageRef, ":")
		labels["dewy.version"] = parts[len(parts)-1]
	}

	ports := opts.Ports

	// If ContainerPort is specified, add localhost-only port mapping
	// Format: "127.0.0.1::containerPort" - Docker will assign a random host port on localhost only
	if opts.ContainerPort > 0 {
		localhostPort := fmt.Sprintf("127.0.0.1::%d", opts.ContainerPort)
		ports = append(ports, localhostPort)
		d.logger.Debug("Adding localhost-only port mapping",
			slog.String("mapping", localhostPort))
	}

	newID, err := d.Run(ctx, RunOptions{
		Image:        opts.ImageRef,
		AppName:      opts.AppName,
		ReplicaIndex: 0, // Single container deployment (for backward compatibility)
		Ports:        ports,
		Labels:       labels,
		Detach:       true,
		Command:      opts.Command,
		ExtraArgs:    opts.ExtraArgs,
	})
	if err != nil {
		return "", fmt.Errorf("start new container failed: %w", err)
	}

	// 5. Health check
	if opts.HealthCheck != nil {
		// Give the container a moment to fully start up before health checking
		d.logger.Info("Waiting for container to start...")
		time.Sleep(3 * time.Second)

		d.logger.Info("Health checking new container")
		if err := opts.HealthCheck(ctx, newID); err != nil {
			d.logger.Error("Health check failed, rolling back")
			if stopErr := d.Stop(ctx, newID, 5*time.Second); stopErr != nil {
				d.logger.Error("Failed to stop container during rollback", slog.String("error", stopErr.Error()))
			}
			if removeErr := d.Remove(ctx, newID); removeErr != nil {
				d.logger.Error("Failed to remove container during rollback", slog.String("error", removeErr.Error()))
			}
			return "", fmt.Errorf("health check failed: %w", err)
		}
	}

	// 5.5. Execute callback after health check passes but before stopping old container
	if callback != nil {
		d.logger.Debug("Executing deployment callback")
		if err := callback(newID); err != nil {
			d.logger.Error("Deployment callback failed, rolling back", slog.String("error", err.Error()))
			if stopErr := d.Stop(ctx, newID, 5*time.Second); stopErr != nil {
				d.logger.Error("Failed to stop container during rollback", slog.String("error", stopErr.Error()))
			}
			if removeErr := d.Remove(ctx, newID); removeErr != nil {
				d.logger.Error("Failed to remove container during rollback", slog.String("error", removeErr.Error()))
			}
			return "", fmt.Errorf("deployment callback failed: %w", err)
		}
	}

	// 6. For Blue-Green deployment, both containers now share the alias
	// We stop the old container directly without disconnecting from network first
	// This causes existing connections to fail fast and retry (connecting to new container)
	// rather than hanging indefinitely
	if currentID != "" {
		d.logger.Info("Stopping old container to complete traffic switch",
			slog.String("note", "Existing connections will reconnect to new container"))
		// Stop old container with a short timeout to force connection migration
		if stopErr := d.Stop(ctx, currentID, 10*time.Second); stopErr != nil {
			d.logger.Error("Failed to stop old container", slog.String("error", stopErr.Error()))
		}
		if removeErr := d.Remove(ctx, currentID); removeErr != nil {
			d.logger.Error("Failed to remove old container", slog.String("error", removeErr.Error()))
		}
	}

	d.logger.Info("Deployment completed successfully", slog.String("container", newID))
	return newID, nil
}

// GetMappedPort returns the host port mapped to the container port.
func (d *Docker) GetMappedPort(ctx context.Context, containerID string, containerPort int) (int, error) {
	portSpec := fmt.Sprintf("%d/tcp", containerPort)
	output, err := d.execCommandOutput(ctx, "port", containerID, portSpec)
	if err != nil {
		return 0, fmt.Errorf("failed to get mapped port: %w", err)
	}

	if output == "" {
		return 0, fmt.Errorf("no port mapping found for container port %d", containerPort)
	}

	// Output format: "0.0.0.0:32768" or "0.0.0.0:32768\n:::32768"
	// Extract the port number from the first mapping
	parts := strings.Split(output, "\n")
	if len(parts) == 0 {
		return 0, fmt.Errorf("unexpected port output format: %s", output)
	}

	// Parse "0.0.0.0:32768" to extract "32768"
	firstMapping := strings.TrimSpace(parts[0])
	colonIdx := strings.LastIndex(firstMapping, ":")
	if colonIdx == -1 {
		return 0, fmt.Errorf("unexpected port format: %s", firstMapping)
	}

	portStr := firstMapping[colonIdx+1:]
	port, err := fmt.Sscanf(portStr, "%d", new(int))
	if err != nil || port != 1 {
		return 0, fmt.Errorf("failed to parse port number from %s: %w", portStr, err)
	}

	var hostPort int
	n, err := fmt.Sscanf(portStr, "%d", &hostPort)
	if err != nil {
		return 0, fmt.Errorf("fmt.Sscanf failed: n=%d, %w", n, err)
	}

	return hostPort, nil
}

// GetRunningContainerWithImage checks if a container is running with the specified image.
// It returns the container ID if found, or an empty string if not found.
func (d *Docker) GetRunningContainerWithImage(ctx context.Context, imageRef string) (string, error) {
	// Get list of running containers with the specified ancestor (image)
	// Format: docker ps --filter ancestor=<image> --filter status=running --format "{{.ID}}"
	output, err := d.execCommandOutput(ctx, "ps",
		"--filter", fmt.Sprintf("ancestor=%s", imageRef),
		"--filter", "status=running",
		"--format", "{{.ID}}")

	if err != nil {
		return "", fmt.Errorf("failed to list running containers: %w", err)
	}

	if output == "" {
		d.logger.Debug("No running container found with image", slog.String("image", imageRef))
		return "", nil
	}

	// If multiple containers are found, take the first one
	containerIDs := strings.Split(output, "\n")
	containerID := strings.TrimSpace(containerIDs[0])

	d.logger.Debug("Found running container with image",
		slog.String("image", imageRef),
		slog.String("container", containerID))
	return containerID, nil
}

// ListImages returns a list of images matching the given repository.
func (d *Docker) ListImages(ctx context.Context, repository string) ([]ImageInfo, error) {
	// Format: docker images --format "{{.ID}}|{{.Repository}}|{{.Tag}}|{{.CreatedAt}}|{{.Size}}" <repository>
	format := "{{.ID}}|{{.Repository}}|{{.Tag}}|{{.CreatedAt}}|{{.Size}}"
	output, err := d.execCommandOutput(ctx, "images", "--format", format, repository)
	if err != nil {
		return nil, fmt.Errorf("failed to list images: %w", err)
	}

	if output == "" {
		return []ImageInfo{}, nil
	}

	lines := strings.Split(output, "\n")
	images := make([]ImageInfo, 0, len(lines))

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		parts := strings.Split(line, "|")
		if len(parts) != 5 {
			d.logger.Warn("Unexpected image format", slog.String("line", line))
			continue
		}

		// Parse creation time - Docker returns various formats
		// Example: "2025-01-13 10:30:45 +0900 JST"
		var created time.Time
		createdStr := strings.TrimSpace(parts[3])

		// Try multiple time formats
		timeFormats := []string{
			"2006-01-02 15:04:05 -0700 MST",
			"2006-01-02 15:04:05 -0700",
			time.RFC3339,
		}

		for _, format := range timeFormats {
			if t, err := time.Parse(format, createdStr); err == nil {
				created = t
				break
			}
		}

		images = append(images, ImageInfo{
			ID:         strings.TrimSpace(parts[0]),
			Repository: strings.TrimSpace(parts[1]),
			Tag:        strings.TrimSpace(parts[2]),
			Created:    created,
			Size:       0, // Size parsing is complex, we don't need it for sorting
		})
	}

	d.logger.Debug("Listed images",
		slog.String("repository", repository),
		slog.Int("count", len(images)))

	return images, nil
}

// RemoveImage removes an image by ID.
func (d *Docker) RemoveImage(ctx context.Context, imageID string) error {
	d.logger.Info("Removing image", slog.String("image", imageID))

	// Use --force to remove even if there are stopped containers using it
	err := d.execCommand(ctx, "rmi", "--force", imageID)
	if err != nil {
		return fmt.Errorf("failed to remove image: %w", err)
	}

	return nil
}

// dockerInspect represents the structure of docker inspect JSON output.
type dockerInspect struct {
	ID      string `json:"Id"`
	Name    string `json:"Name"`
	Created string `json:"Created"`
	State   struct {
		Status    string `json:"Status"`
		StartedAt string `json:"StartedAt"`
	} `json:"State"`
	Config struct {
		Image  string            `json:"Image"`
		Labels map[string]string `json:"Labels"`
	} `json:"Config"`
	NetworkSettings struct {
		Ports map[string][]struct {
			HostIP   string `json:"HostIp"`
			HostPort string `json:"HostPort"`
		} `json:"Ports"`
	} `json:"NetworkSettings"`
}

// GetContainerInfo returns detailed information about a container.
func (d *Docker) GetContainerInfo(ctx context.Context, containerID string, containerPort int) (*Info, error) {
	output, err := d.execCommandOutput(ctx, "inspect", containerID)
	if err != nil {
		return nil, fmt.Errorf("failed to inspect container: %w", err)
	}

	var inspects []dockerInspect
	if err := json.Unmarshal([]byte(output), &inspects); err != nil {
		return nil, fmt.Errorf("failed to parse inspect output: %w", err)
	}

	if len(inspects) == 0 {
		return nil, ErrContainerNotFound
	}

	inspect := inspects[0]

	// Parse timestamps
	startedAt, err := time.Parse(time.RFC3339Nano, inspect.State.StartedAt)
	if err != nil {
		d.logger.Warn("Failed to parse StartedAt timestamp",
			slog.String("container", containerID),
			slog.String("timestamp", inspect.State.StartedAt))
		// Use zero time as fallback - will display as "0001-01-01 00:00:00" which indicates invalid/missing timestamp
		startedAt = time.Time{}
	}

	// Get deployed_at from labels if available
	deployedAt := startedAt
	if deployedAtStr, ok := inspect.Config.Labels["dewy.deployed_at"]; ok {
		if t, err := time.Parse(time.RFC3339, deployedAtStr); err == nil {
			deployedAt = t
		}
	}

	// Find the mapped port
	ipPort := ""
	portSpec := fmt.Sprintf("%d/tcp", containerPort)
	if portBindings, ok := inspect.NetworkSettings.Ports[portSpec]; ok && len(portBindings) > 0 {
		ipPort = fmt.Sprintf("%s:%s", portBindings[0].HostIP, portBindings[0].HostPort)
	}

	// Remove leading slash from container name
	name := strings.TrimPrefix(inspect.Name, "/")

	return &Info{
		ID:         inspect.ID,
		Name:       name,
		Image:      inspect.Config.Image,
		Status:     inspect.State.Status,
		IPPort:     ipPort,
		StartedAt:  startedAt,
		DeployedAt: deployedAt,
		Labels:     inspect.Config.Labels,
	}, nil
}

// ListContainersByLabels returns detailed information about containers matching the given labels.
func (d *Docker) ListContainersByLabels(ctx context.Context, labels map[string]string, containerPort int) ([]*Info, error) {
	containerIDs, err := d.FindContainersByLabel(ctx, labels)
	if err != nil {
		return nil, fmt.Errorf("failed to find containers: %w", err)
	}

	infos := make([]*Info, 0, len(containerIDs))
	for _, containerID := range containerIDs {
		info, err := d.GetContainerInfo(ctx, containerID, containerPort)
		if err != nil {
			d.logger.Warn("Failed to get container info",
				slog.String("container", containerID),
				slog.String("error", err.Error()))
			continue
		}
		infos = append(infos, info)
	}

	return infos, nil
}
