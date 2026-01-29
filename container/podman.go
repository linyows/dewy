package container

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Podman implements Runtime interface using Podman CLI.
type Podman struct {
	cmd       string
	logger    *slog.Logger
	drainTime time.Duration
}

// NewPodman creates a new Podman runtime.
func NewPodman(logger *slog.Logger, drainTime time.Duration) (*Podman, error) {
	cmd := "podman"
	if _, err := exec.LookPath(cmd); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrRuntimeNotFound, err)
	}

	return &Podman{
		cmd:       cmd,
		logger:    logger,
		drainTime: drainTime,
	}, nil
}

// execCommand executes a podman command without returning output.
func (p *Podman) execCommand(ctx context.Context, args ...string) error {
	// #nosec G204 - args are constructed internally from validated inputs
	cmd := exec.CommandContext(ctx, p.cmd, args...)
	p.logger.Debug("Executing podman command",
		slog.String("cmd", p.cmd),
		slog.Any("args", args))

	output, err := cmd.CombinedOutput()
	if err != nil {
		p.logger.Error("Podman command failed",
			slog.String("cmd", p.cmd),
			slog.Any("args", args),
			slog.String("output", string(output)),
			slog.String("error", err.Error()))
		return fmt.Errorf("podman %s failed: %w: %s",
			strings.Join(args, " "), err, string(output))
	}

	return nil
}

// execCommandOutput executes a podman command and returns the output.
func (p *Podman) execCommandOutput(ctx context.Context, args ...string) (string, error) {
	// #nosec G204 - args are constructed internally from validated inputs
	cmd := exec.CommandContext(ctx, p.cmd, args...)
	p.logger.Debug("Executing podman command",
		slog.String("cmd", p.cmd),
		slog.Any("args", args))

	output, err := cmd.Output()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			p.logger.Error("Podman command failed",
				slog.String("cmd", p.cmd),
				slog.Any("args", args),
				slog.String("stderr", string(exitErr.Stderr)),
				slog.String("error", err.Error()))
			return "", fmt.Errorf("podman %s failed: %w: %s",
				strings.Join(args, " "), err, string(exitErr.Stderr))
		}
		return "", err
	}

	return strings.TrimSpace(string(output)), nil
}

// Pull pulls an image from the registry.
// If the image already exists locally, it will still attempt to pull to get the latest version.
func (p *Podman) Pull(ctx context.Context, imageRef string) error {
	// Check if image exists locally first
	_, err := p.execCommandOutput(ctx, "image", "inspect", imageRef)
	if err == nil {
		// Image exists locally
		p.logger.Info("Image already exists locally, pulling to check for updates",
			slog.String("image", imageRef))
	}

	// Always try to pull to get the latest version
	// For local-only images (not in a registry), this will fail but that's expected
	p.logger.Info("Pulling image", slog.String("image", imageRef))
	pullErr := p.execCommand(ctx, "pull", imageRef)

	// If pull fails but image exists locally, we can use the local image
	if pullErr != nil && err == nil {
		p.logger.Warn("Failed to pull image, but local image exists - using local version",
			slog.String("image", imageRef),
			slog.String("pull_error", pullErr.Error()))
		return nil // Use local image
	}

	return pullErr
}

// Run starts a new container and returns the container ID.
func (p *Podman) Run(ctx context.Context, opts RunOptions) (string, error) {
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

	// Build podman run command
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

	// Add default --user if not specified by user
	if !hasUserOption(filteredArgs) {
		args = append(args, "--user", fmt.Sprintf("%d:%d", os.Getuid(), os.Getgid()))
	}

	// Image
	args = append(args, opts.Image)

	// Command and arguments
	args = append(args, opts.Command...)

	p.logger.Info("Starting container",
		slog.String("name", containerName),
		slog.String("image", opts.Image))

	containerID, err := p.execCommandOutput(ctx, args...)
	if err != nil {
		return "", err
	}

	return containerID, nil
}

// Stop stops a running container gracefully.
func (p *Podman) Stop(ctx context.Context, containerID string, timeout time.Duration) error {
	p.logger.Info("Stopping container gracefully",
		slog.String("container", containerID),
		slog.Duration("timeout", timeout))

	timeoutSec := int(timeout.Seconds())
	return p.execCommand(ctx, "stop", fmt.Sprintf("--time=%d", timeoutSec), containerID)
}

// Remove removes a container.
func (p *Podman) Remove(ctx context.Context, containerID string) error {
	p.logger.Info("Removing container", slog.String("container", containerID))
	return p.execCommand(ctx, "rm", containerID)
}

// FindContainerByLabel finds a container by labels.
func (p *Podman) FindContainerByLabel(ctx context.Context, labels map[string]string) (string, error) {
	args := []string{"ps", "-q"}

	for key, value := range labels {
		args = append(args, "--filter", fmt.Sprintf("label=%s=%s", key, value))
	}

	output, err := p.execCommandOutput(ctx, args...)
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
func (p *Podman) FindContainersByLabel(ctx context.Context, labels map[string]string) ([]string, error) {
	args := []string{"ps", "-q"}

	for key, value := range labels {
		args = append(args, "--filter", fmt.Sprintf("label=%s=%s", key, value))
	}

	output, err := p.execCommandOutput(ctx, args...)
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

	p.logger.Debug("Found containers by label",
		slog.Any("labels", labels),
		slog.Int("count", len(containers)))

	return containers, nil
}

// UpdateLabel updates a container's label.
func (p *Podman) UpdateLabel(ctx context.Context, containerID, key, value string) error {
	p.logger.Debug("Label update skipped (not supported by Podman)",
		slog.String("container", containerID),
		slog.String("key", key),
		slog.String("value", value))

	// Podman doesn't support updating labels on running containers
	// Labels are immutable after container creation
	return nil
}

// DeployContainerWithCallback performs Blue-Green deployment with a callback.
func (p *Podman) DeployContainerWithCallback(ctx context.Context, opts DeployOptions, callback DeployContainerCallback) (string, error) {
	return p.deployContainerInternal(ctx, opts, callback)
}

// DeployContainer performs Blue-Green deployment.
func (p *Podman) DeployContainer(ctx context.Context, opts DeployOptions) error {
	_, err := p.deployContainerInternal(ctx, opts, nil)
	return err
}

// deployContainerInternal is the internal implementation of container deployment.
func (p *Podman) deployContainerInternal(ctx context.Context, opts DeployOptions, callback DeployContainerCallback) (string, error) {
	// 1. Pull new image
	p.logger.Info("Pulling new image", slog.String("image", opts.ImageRef))
	if err := p.Pull(ctx, opts.ImageRef); err != nil {
		return "", fmt.Errorf("pull failed: %w", err)
	}

	// 2. Find current container
	currentID, err := p.FindContainerByLabel(ctx, map[string]string{
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
	// Format: "127.0.0.1::containerPort" - Podman will assign a random host port on localhost only
	if opts.ContainerPort > 0 {
		localhostPort := fmt.Sprintf("127.0.0.1::%d", opts.ContainerPort)
		ports = append(ports, localhostPort)
		p.logger.Debug("Adding localhost-only port mapping",
			slog.String("mapping", localhostPort))
	}

	newID, err := p.Run(ctx, RunOptions{
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
		p.logger.Info("Waiting for container to start...")
		time.Sleep(3 * time.Second)

		p.logger.Info("Health checking new container")
		if err := opts.HealthCheck(ctx, newID); err != nil {
			p.logger.Error("Health check failed, rolling back")
			if stopErr := p.Stop(ctx, newID, 5*time.Second); stopErr != nil {
				p.logger.Error("Failed to stop container during rollback", slog.String("error", stopErr.Error()))
			}
			if removeErr := p.Remove(ctx, newID); removeErr != nil {
				p.logger.Error("Failed to remove container during rollback", slog.String("error", removeErr.Error()))
			}
			return "", fmt.Errorf("health check failed: %w", err)
		}
	}

	// 5.5. Execute callback after health check passes but before stopping old container
	if callback != nil {
		p.logger.Debug("Executing deployment callback")
		if err := callback(newID); err != nil {
			p.logger.Error("Deployment callback failed, rolling back", slog.String("error", err.Error()))
			if stopErr := p.Stop(ctx, newID, 5*time.Second); stopErr != nil {
				p.logger.Error("Failed to stop container during rollback", slog.String("error", stopErr.Error()))
			}
			if removeErr := p.Remove(ctx, newID); removeErr != nil {
				p.logger.Error("Failed to remove container during rollback", slog.String("error", removeErr.Error()))
			}
			return "", fmt.Errorf("deployment callback failed: %w", err)
		}
	}

	// 6. For Blue-Green deployment, both containers now share the alias
	// We stop the old container directly without disconnecting from network first
	// This causes existing connections to fail fast and retry (connecting to new container)
	// rather than hanging indefinitely
	if currentID != "" {
		p.logger.Info("Stopping old container to complete traffic switch",
			slog.String("note", "Existing connections will reconnect to new container"))
		// Stop old container with a short timeout to force connection migration
		if stopErr := p.Stop(ctx, currentID, 10*time.Second); stopErr != nil {
			p.logger.Error("Failed to stop old container", slog.String("error", stopErr.Error()))
		}
		if removeErr := p.Remove(ctx, currentID); removeErr != nil {
			p.logger.Error("Failed to remove old container", slog.String("error", removeErr.Error()))
		}
	}

	p.logger.Info("Deployment completed successfully", slog.String("container", newID))
	return newID, nil
}

// GetMappedPort returns the host port mapped to the container port.
func (p *Podman) GetMappedPort(ctx context.Context, containerID string, containerPort int) (int, error) {
	portSpec := fmt.Sprintf("%d/tcp", containerPort)
	output, err := p.execCommandOutput(ctx, "port", containerID, portSpec)
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

// GetRunningContainerWithImage checks if a container is running with the specified image and app name.
// It returns the container ID if found, or an empty string if not found.
func (p *Podman) GetRunningContainerWithImage(ctx context.Context, imageRef, appName string) (string, error) {
	// Get list of running containers with the specified ancestor (image) and app label
	// Format: podman ps --filter ancestor=<image> --filter label=dewy.app=<appName> --filter status=running --format "{{.ID}}"
	output, err := p.execCommandOutput(ctx, "ps",
		"--filter", fmt.Sprintf("ancestor=%s", imageRef),
		"--filter", fmt.Sprintf("label=dewy.app=%s", appName),
		"--filter", "status=running",
		"--format", "{{.ID}}")

	if err != nil {
		return "", fmt.Errorf("failed to list running containers: %w", err)
	}

	if output == "" {
		p.logger.Debug("No running container found with image", slog.String("image", imageRef))
		return "", nil
	}

	// If multiple containers are found, take the first one
	containerIDs := strings.Split(output, "\n")
	containerID := strings.TrimSpace(containerIDs[0])

	p.logger.Debug("Found running container with image",
		slog.String("image", imageRef),
		slog.String("container", containerID))
	return containerID, nil
}

// ListImages returns a list of images matching the given repository.
func (p *Podman) ListImages(ctx context.Context, repository string) ([]ImageInfo, error) {
	// Format: podman images --format "{{.ID}}|{{.Repository}}|{{.Tag}}|{{.CreatedAt}}|{{.Size}}" <repository>
	format := "{{.ID}}|{{.Repository}}|{{.Tag}}|{{.CreatedAt}}|{{.Size}}"
	output, err := p.execCommandOutput(ctx, "images", "--format", format, repository)
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
			p.logger.Warn("Unexpected image format", slog.String("line", line))
			continue
		}

		// Parse creation time - Podman returns various formats
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

	p.logger.Debug("Listed images",
		slog.String("repository", repository),
		slog.Int("count", len(images)))

	return images, nil
}

// RemoveImage removes an image by ID.
func (p *Podman) RemoveImage(ctx context.Context, imageID string) error {
	p.logger.Info("Removing image", slog.String("image", imageID))

	// Use --force to remove even if there are stopped containers using it
	err := p.execCommand(ctx, "rmi", "--force", imageID)
	if err != nil {
		return fmt.Errorf("failed to remove image: %w", err)
	}

	return nil
}

// podmanInspect represents the structure of podman inspect JSON output.
type podmanInspect struct {
	ID      string `json:"Id"`
	Name    string `json:"Name"`
	Created string `json:"Created"`
	State   struct {
		Status    string `json:"Status"`
		StartedAt string `json:"StartedAt"`
	} `json:"State"`
	Config struct {
		Image        string              `json:"Image"`
		Labels       map[string]string   `json:"Labels"`
		ExposedPorts map[string]struct{} `json:"ExposedPorts"` // e.g., {"80/tcp": {}, "443/tcp": {}}
	} `json:"Config"`
	NetworkSettings struct {
		Ports map[string][]struct {
			HostIP   string `json:"HostIp"`
			HostPort string `json:"HostPort"`
		} `json:"Ports"`
	} `json:"NetworkSettings"`
}

// podmanImageInspect represents the structure of podman image inspect JSON output.
type podmanImageInspect struct {
	ID     string `json:"Id"`
	Config struct {
		ExposedPorts map[string]struct{} `json:"ExposedPorts"` // e.g., {"80/tcp": {}, "443/tcp": {}}
	} `json:"Config"`
}

// GetContainerInfo returns detailed information about a container.
func (p *Podman) GetContainerInfo(ctx context.Context, containerID string, containerPort int) (*Info, error) {
	output, err := p.execCommandOutput(ctx, "inspect", containerID)
	if err != nil {
		return nil, fmt.Errorf("failed to inspect container: %w", err)
	}

	var inspects []podmanInspect
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
		p.logger.Warn("Failed to parse StartedAt timestamp",
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
	if containerPort > 0 {
		// If container port is specified, use it
		portSpec := fmt.Sprintf("%d/tcp", containerPort)
		if portBindings, ok := inspect.NetworkSettings.Ports[portSpec]; ok && len(portBindings) > 0 {
			ipPort = fmt.Sprintf("%s:%s", portBindings[0].HostIP, portBindings[0].HostPort)
		}
	} else {
		// If container port is not specified (0), find first available port mapping
		// This handles the case where --port was specified without container port (auto-detect)
		for _, portBindings := range inspect.NetworkSettings.Ports {
			if len(portBindings) > 0 {
				ipPort = fmt.Sprintf("%s:%s", portBindings[0].HostIP, portBindings[0].HostPort)
				break
			}
		}
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
func (p *Podman) ListContainersByLabels(ctx context.Context, labels map[string]string, containerPort int) ([]*Info, error) {
	containerIDs, err := p.FindContainersByLabel(ctx, labels)
	if err != nil {
		return nil, fmt.Errorf("failed to find containers: %w", err)
	}

	infos := make([]*Info, 0, len(containerIDs))
	for _, containerID := range containerIDs {
		info, err := p.GetContainerInfo(ctx, containerID, containerPort)
		if err != nil {
			p.logger.Warn("Failed to get container info",
				slog.String("container", containerID),
				slog.String("error", err.Error()))
			continue
		}
		infos = append(infos, info)
	}

	return infos, nil
}

// GetImageExposedPorts returns the list of exposed ports from an image.
// Returns port numbers (e.g., [80, 443]) sorted in ascending order.
func (p *Podman) GetImageExposedPorts(ctx context.Context, imageRef string) ([]int, error) {
	output, err := p.execCommandOutput(ctx, "image", "inspect", imageRef)
	if err != nil {
		return nil, fmt.Errorf("failed to inspect image: %w", err)
	}

	var inspects []podmanImageInspect
	if err := json.Unmarshal([]byte(output), &inspects); err != nil {
		return nil, fmt.Errorf("failed to parse image inspect output: %w", err)
	}

	if len(inspects) == 0 {
		return nil, fmt.Errorf("image not found: %s", imageRef)
	}

	inspect := inspects[0]
	if len(inspect.Config.ExposedPorts) == 0 {
		return []int{}, nil
	}

	// Parse exposed ports
	ports := make([]int, 0, len(inspect.Config.ExposedPorts))
	for portSpec := range inspect.Config.ExposedPorts {
		// portSpec format: "80/tcp", "443/tcp", "53/udp", etc.
		// We only support TCP ports for now
		if !strings.HasSuffix(portSpec, "/tcp") {
			continue
		}

		portStr := strings.TrimSuffix(portSpec, "/tcp")
		port, err := strconv.Atoi(portStr)
		if err != nil {
			p.logger.Warn("Failed to parse exposed port",
				slog.String("port_spec", portSpec),
				slog.String("error", err.Error()))
			continue
		}

		ports = append(ports, port)
	}

	// Sort ports
	sort.Ints(ports)

	p.logger.Debug("Detected exposed ports from image",
		slog.String("image", imageRef),
		slog.Any("ports", ports))

	return ports, nil
}
