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

// Runtime implements Runtime interface using a container CLI (docker or podman).
type Runtime struct {
	cmd                string
	logger             *slog.Logger
	drainTime          time.Duration
	loggedInRegistries map[string]bool
}

// Forbidden options that conflict with Dewy management or pose security risks.
var forbiddenOptions = []string{
	"-d", "--detach",
	"-it",
	"-i", "--interactive",
	"-t", "--tty",
	"-l", "--label",
	"-p", "--publish",
	"--privileged",
	"--pid",
	"--cap-add",
	"--security-opt",
	"--device",
	"--userns",
	"--cgroupns",
}

// newCLIRuntime creates a new Runtime with the specified command name.
func newCLIRuntime(cmd string, logger *slog.Logger, drainTime time.Duration) (*Runtime, error) {
	if _, err := exec.LookPath(cmd); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrRuntimeNotFound, err)
	}

	return &Runtime{
		cmd:                cmd,
		logger:             logger,
		drainTime:          drainTime,
		loggedInRegistries: make(map[string]bool),
	}, nil
}

// extractRegistry extracts the registry host from an image reference.
// For images without explicit registry (e.g., "nginx:latest"), returns "docker.io".
// For images with registry (e.g., "ghcr.io/owner/repo:tag"), returns the registry host.
func extractRegistry(imageRef string) string {
	// Remove tag or digest
	ref := imageRef
	if idx := strings.LastIndex(ref, "@"); idx != -1 {
		ref = ref[:idx]
	}
	if idx := strings.LastIndex(ref, ":"); idx != -1 {
		// Check if this is a port number (e.g., localhost:5000/image)
		slashIdx := strings.LastIndex(ref[:idx], "/")
		if slashIdx == -1 || !strings.Contains(ref[slashIdx:idx], ".") {
			ref = ref[:idx]
		}
	}

	// Check if the first part contains a dot or colon (indicating a registry)
	parts := strings.SplitN(ref, "/", 2)
	if len(parts) == 1 {
		// No slash, it's a Docker Hub official image (e.g., "nginx")
		return "docker.io"
	}

	firstPart := parts[0]
	if strings.Contains(firstPart, ".") || strings.Contains(firstPart, ":") || firstPart == "localhost" {
		return firstPart
	}

	// No registry specified (e.g., "library/nginx"), use Docker Hub
	return "docker.io"
}

// getCredentials returns username and password for the given registry from environment variables.
func getCredentials(registry string) (username, password string) {
	// GitHub Container Registry
	if strings.Contains(registry, "ghcr.io") {
		if token := os.Getenv("GITHUB_TOKEN"); token != "" {
			return "token", token
		}
	}

	// AWS ECR - check for ECR-specific credentials first
	if strings.Contains(registry, ".ecr.") && strings.Contains(registry, ".amazonaws.com") {
		if token := os.Getenv("AWS_ECR_PASSWORD"); token != "" {
			return "AWS", token
		}
	}

	// Google Artifact Registry / Container Registry
	if strings.Contains(registry, "gcr.io") || strings.Contains(registry, "-docker.pkg.dev") {
		if token := os.Getenv("GCR_TOKEN"); token != "" {
			return "_json_key", token
		}
	}

	// Generic credentials (fallback)
	username = os.Getenv("DOCKER_USERNAME")
	password = os.Getenv("DOCKER_PASSWORD")

	return username, password
}

// isAuthError checks if the error message indicates an authentication failure.
func isAuthError(output string) bool {
	lowerOutput := strings.ToLower(output)
	authIndicators := []string{
		"unauthorized",
		"authentication required",
		"denied",
		"access forbidden",
		"not authorized",
		"login required",
	}
	for _, indicator := range authIndicators {
		if strings.Contains(lowerOutput, indicator) {
			return true
		}
	}
	return false
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

// hasUserOption checks if --user or -u option is present in args.
func hasUserOption(args []string) bool {
	for _, arg := range args {
		if arg == "--user" || strings.HasPrefix(arg, "--user=") {
			return true
		}
		if arg == "-u" || strings.HasPrefix(arg, "-u=") {
			return true
		}
		// Handle combined short options like -u1000 (without =)
		if len(arg) > 2 && arg[0] == '-' && arg[1] == 'u' && arg[2] != '-' {
			return true
		}
	}
	return false
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

// Login authenticates with the specified registry using credentials from environment variables.
func (r *Runtime) Login(ctx context.Context, registry string) error {
	username, password := getCredentials(registry)
	if username == "" || password == "" {
		r.logger.Debug("No credentials found for registry", slog.String("registry", registry))
		return nil
	}

	r.logger.Info("Logging in to registry", slog.String("registry", registry))

	// Use --password-stdin for security
	// #nosec G204 - args are constructed internally from validated inputs
	cmd := exec.CommandContext(ctx, r.cmd, "login", "-u", username, "--password-stdin", registry)
	cmd.Stdin = strings.NewReader(password)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s login failed for %s: %w: %s", r.cmd, registry, err, string(output))
	}

	r.loggedInRegistries[registry] = true
	r.logger.Info("Successfully logged in to registry", slog.String("registry", registry))

	return nil
}

// execCommand executes a command without returning output.
func (r *Runtime) execCommand(ctx context.Context, args ...string) error {
	// #nosec G204 - args are constructed internally from validated inputs
	cmd := exec.CommandContext(ctx, r.cmd, args...)
	r.logger.Debug("Executing command",
		slog.String("cmd", r.cmd),
		slog.Any("args", args))

	output, err := cmd.CombinedOutput()
	if err != nil {
		r.logger.Error("Command failed",
			slog.String("cmd", r.cmd),
			slog.Any("args", args),
			slog.String("output", string(output)),
			slog.String("error", err.Error()))
		return fmt.Errorf("%s %s failed: %w: %s",
			r.cmd, strings.Join(args, " "), err, string(output))
	}

	return nil
}

// execCommandOutput executes a command and returns the output.
func (r *Runtime) execCommandOutput(ctx context.Context, args ...string) (string, error) {
	// #nosec G204 - args are constructed internally from validated inputs
	cmd := exec.CommandContext(ctx, r.cmd, args...)
	r.logger.Debug("Executing command",
		slog.String("cmd", r.cmd),
		slog.Any("args", args))

	output, err := cmd.Output()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			r.logger.Error("Command failed",
				slog.String("cmd", r.cmd),
				slog.Any("args", args),
				slog.String("stderr", string(exitErr.Stderr)),
				slog.String("error", err.Error()))
			return "", fmt.Errorf("%s %s failed: %w: %s",
				r.cmd, strings.Join(args, " "), err, string(exitErr.Stderr))
		}
		return "", err
	}

	return strings.TrimSpace(string(output)), nil
}

// Pull pulls an image from the registry.
// If the image already exists locally, it will still attempt to pull to get the latest version.
// Automatically handles authentication: logs in on first pull and retries on auth errors.
func (r *Runtime) Pull(ctx context.Context, imageRef string) error {
	// Check if image exists locally first
	_, localErr := r.execCommandOutput(ctx, "image", "inspect", imageRef)
	if localErr == nil {
		r.logger.Info("Image already exists locally, pulling to check for updates",
			slog.String("image", imageRef))
	}

	// Extract registry and attempt login if not already logged in
	registry := extractRegistry(imageRef)
	if !r.loggedInRegistries[registry] {
		if err := r.Login(ctx, registry); err != nil {
			r.logger.Warn("Initial login failed, will attempt pull anyway",
				slog.String("registry", registry),
				slog.String("error", err.Error()))
		}
	}

	// Always try to pull to get the latest version
	r.logger.Info("Pulling image", slog.String("image", imageRef))
	output, pullErr := r.pullImage(ctx, imageRef)

	// If pull fails with auth error, retry login and pull again
	if pullErr != nil && isAuthError(output) {
		r.logger.Info("Authentication error detected, attempting re-login",
			slog.String("registry", registry))

		delete(r.loggedInRegistries, registry)
		if loginErr := r.Login(ctx, registry); loginErr != nil {
			r.logger.Error("Re-login failed",
				slog.String("registry", registry),
				slog.String("error", loginErr.Error()))
		} else {
			r.logger.Info("Retrying pull after re-login", slog.String("image", imageRef))
			_, pullErr = r.pullImage(ctx, imageRef)
		}
	}

	// If pull fails but image exists locally, we can use the local image
	if pullErr != nil && localErr == nil {
		r.logger.Warn("Failed to pull image, but local image exists - using local version",
			slog.String("image", imageRef),
			slog.String("pull_error", pullErr.Error()))
		return nil
	}

	return pullErr
}

// pullImage executes pull and returns output and error.
func (r *Runtime) pullImage(ctx context.Context, imageRef string) (string, error) {
	// #nosec G204 - args are constructed internally from validated inputs
	cmd := exec.CommandContext(ctx, r.cmd, "pull", imageRef)
	r.logger.Debug("Executing command",
		slog.String("cmd", r.cmd),
		slog.Any("args", []string{"pull", imageRef}))

	output, err := cmd.CombinedOutput()
	if err != nil {
		r.logger.Error("Pull failed",
			slog.String("image", imageRef),
			slog.String("output", string(output)),
			slog.String("error", err.Error()))
		return string(output), fmt.Errorf("%s pull %s failed: %w: %s", r.cmd, imageRef, err, string(output))
	}

	return string(output), nil
}

// Run starts a new container and returns the container ID.
func (r *Runtime) Run(ctx context.Context, opts RunOptions) (string, error) {
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

	// Build run command
	args := []string{"run"}

	if opts.Detach {
		args = append(args, "-d")
	}

	args = append(args, "--name", containerName)

	for key, value := range opts.Labels {
		args = append(args, "--label", fmt.Sprintf("%s=%s", key, value))
	}

	for _, port := range opts.Ports {
		args = append(args, "-p", port)
	}

	args = append(args, filteredArgs...)

	if !hasUserOption(filteredArgs) {
		args = append(args, "--user", fmt.Sprintf("%d:%d", os.Getuid(), os.Getgid()))
	}

	args = append(args, opts.Image)
	args = append(args, opts.Command...)

	r.logger.Info("Starting container",
		slog.String("name", containerName),
		slog.String("image", opts.Image))

	containerID, err := r.execCommandOutput(ctx, args...)
	if err != nil {
		return "", err
	}

	return containerID, nil
}

// Stop stops a running container gracefully.
func (r *Runtime) Stop(ctx context.Context, containerID string, timeout time.Duration) error {
	r.logger.Info("Stopping container gracefully",
		slog.String("container", containerID),
		slog.Duration("timeout", timeout))

	timeoutSec := int(timeout.Seconds())
	return r.execCommand(ctx, "stop", fmt.Sprintf("--time=%d", timeoutSec), containerID)
}

// Remove removes a container.
func (r *Runtime) Remove(ctx context.Context, containerID string) error {
	r.logger.Info("Removing container", slog.String("container", containerID))
	return r.execCommand(ctx, "rm", containerID)
}

// FindContainerByLabel finds a container by labels.
func (r *Runtime) FindContainerByLabel(ctx context.Context, labels map[string]string) (string, error) {
	args := []string{"ps", "-q"}

	for key, value := range labels {
		args = append(args, "--filter", fmt.Sprintf("label=%s=%s", key, value))
	}

	output, err := r.execCommandOutput(ctx, args...)
	if err != nil {
		return "", err
	}

	if output == "" {
		return "", ErrContainerNotFound
	}

	lines := strings.Split(output, "\n")
	return lines[0], nil
}

// FindContainersByLabel finds all containers matching the given labels.
func (r *Runtime) FindContainersByLabel(ctx context.Context, labels map[string]string) ([]string, error) {
	args := []string{"ps", "-q"}

	for key, value := range labels {
		args = append(args, "--filter", fmt.Sprintf("label=%s=%s", key, value))
	}

	output, err := r.execCommandOutput(ctx, args...)
	if err != nil {
		return nil, err
	}

	if output == "" {
		return []string{}, nil
	}

	lines := strings.Split(output, "\n")
	containers := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			containers = append(containers, line)
		}
	}

	r.logger.Debug("Found containers by label",
		slog.Any("labels", labels),
		slog.Int("count", len(containers)))

	return containers, nil
}

// UpdateLabel updates a container's label.
func (r *Runtime) UpdateLabel(ctx context.Context, containerID, key, value string) error {
	r.logger.Debug("Label update skipped (not supported by container runtimes)",
		slog.String("container", containerID),
		slog.String("key", key),
		slog.String("value", value))

	// Docker/Podman don't support updating labels on running containers
	// Labels are immutable after container creation
	return nil
}

// DeployContainerWithCallback performs Blue-Green deployment with a callback.
func (r *Runtime) DeployContainerWithCallback(ctx context.Context, opts DeployOptions, callback DeployContainerCallback) (string, error) {
	return r.deployContainerInternal(ctx, opts, callback)
}

// DeployContainer performs Blue-Green deployment.
func (r *Runtime) DeployContainer(ctx context.Context, opts DeployOptions) error {
	_, err := r.deployContainerInternal(ctx, opts, nil)
	return err
}

// deployContainerInternal is the internal implementation of container deployment.
func (r *Runtime) deployContainerInternal(ctx context.Context, opts DeployOptions, callback DeployContainerCallback) (string, error) {
	// 1. Pull new image
	r.logger.Info("Pulling new image", slog.String("image", opts.ImageRef))
	if err := r.Pull(ctx, opts.ImageRef); err != nil {
		return "", fmt.Errorf("pull failed: %w", err)
	}

	// 2. Find current container
	currentID, err := r.FindContainerByLabel(ctx, map[string]string{
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

	if strings.Contains(opts.ImageRef, ":") {
		parts := strings.Split(opts.ImageRef, ":")
		labels["dewy.version"] = parts[len(parts)-1]
	}

	ports := opts.Ports

	if opts.ContainerPort > 0 {
		localhostPort := fmt.Sprintf("127.0.0.1::%d", opts.ContainerPort)
		ports = append(ports, localhostPort)
		r.logger.Debug("Adding localhost-only port mapping",
			slog.String("mapping", localhostPort))
	}

	newID, err := r.Run(ctx, RunOptions{
		Image:        opts.ImageRef,
		AppName:      opts.AppName,
		ReplicaIndex: 0,
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
		r.logger.Info("Waiting for container to start...")
		time.Sleep(3 * time.Second)

		r.logger.Info("Health checking new container")
		if err := opts.HealthCheck(ctx, newID); err != nil {
			r.logger.Error("Health check failed, rolling back")
			if stopErr := r.Stop(ctx, newID, 5*time.Second); stopErr != nil {
				r.logger.Error("Failed to stop container during rollback", slog.String("error", stopErr.Error()))
			}
			if removeErr := r.Remove(ctx, newID); removeErr != nil {
				r.logger.Error("Failed to remove container during rollback", slog.String("error", removeErr.Error()))
			}
			return "", fmt.Errorf("health check failed: %w", err)
		}
	}

	// 5.5. Execute callback after health check passes but before stopping old container
	if callback != nil {
		r.logger.Debug("Executing deployment callback")
		if err := callback(newID); err != nil {
			r.logger.Error("Deployment callback failed, rolling back", slog.String("error", err.Error()))
			if stopErr := r.Stop(ctx, newID, 5*time.Second); stopErr != nil {
				r.logger.Error("Failed to stop container during rollback", slog.String("error", stopErr.Error()))
			}
			if removeErr := r.Remove(ctx, newID); removeErr != nil {
				r.logger.Error("Failed to remove container during rollback", slog.String("error", removeErr.Error()))
			}
			return "", fmt.Errorf("deployment callback failed: %w", err)
		}
	}

	// 6. Stop old container
	if currentID != "" {
		r.logger.Info("Stopping old container to complete traffic switch",
			slog.String("note", "Existing connections will reconnect to new container"))
		if stopErr := r.Stop(ctx, currentID, 10*time.Second); stopErr != nil {
			r.logger.Error("Failed to stop old container", slog.String("error", stopErr.Error()))
		}
		if removeErr := r.Remove(ctx, currentID); removeErr != nil {
			r.logger.Error("Failed to remove old container", slog.String("error", removeErr.Error()))
		}
	}

	r.logger.Info("Deployment completed successfully", slog.String("container", newID))
	return newID, nil
}

// GetMappedPort returns the host port mapped to the container port.
func (r *Runtime) GetMappedPort(ctx context.Context, containerID string, containerPort int) (int, error) {
	portSpec := fmt.Sprintf("%d/tcp", containerPort)
	output, err := r.execCommandOutput(ctx, "port", containerID, portSpec)
	if err != nil {
		return 0, fmt.Errorf("failed to get mapped port: %w", err)
	}

	if output == "" {
		return 0, fmt.Errorf("no port mapping found for container port %d", containerPort)
	}

	// Output format: "0.0.0.0:32768" or "0.0.0.0:32768\n:::32768"
	parts := strings.Split(output, "\n")
	if len(parts) == 0 {
		return 0, fmt.Errorf("unexpected port output format: %s", output)
	}

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
func (r *Runtime) GetRunningContainerWithImage(ctx context.Context, imageRef, appName string) (string, error) {
	output, err := r.execCommandOutput(ctx, "ps",
		"--filter", fmt.Sprintf("ancestor=%s", imageRef),
		"--filter", fmt.Sprintf("label=dewy.app=%s", appName),
		"--filter", "status=running",
		"--format", "{{.ID}}")

	if err != nil {
		return "", fmt.Errorf("failed to list running containers: %w", err)
	}

	if output == "" {
		r.logger.Debug("No running container found with image", slog.String("image", imageRef))
		return "", nil
	}

	containerIDs := strings.Split(output, "\n")
	containerID := strings.TrimSpace(containerIDs[0])

	r.logger.Debug("Found running container with image",
		slog.String("image", imageRef),
		slog.String("container", containerID))
	return containerID, nil
}

// ListImages returns a list of images matching the given repository.
func (r *Runtime) ListImages(ctx context.Context, repository string) ([]ImageInfo, error) {
	format := "{{.ID}}|{{.Repository}}|{{.Tag}}|{{.CreatedAt}}|{{.Size}}"
	output, err := r.execCommandOutput(ctx, "images", "--format", format, repository)
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
			r.logger.Warn("Unexpected image format", slog.String("line", line))
			continue
		}

		var created time.Time
		createdStr := strings.TrimSpace(parts[3])

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
			Size:       0,
		})
	}

	r.logger.Debug("Listed images",
		slog.String("repository", repository),
		slog.Int("count", len(images)))

	return images, nil
}

// RemoveImage removes an image by ID.
func (r *Runtime) RemoveImage(ctx context.Context, imageID string) error {
	r.logger.Info("Removing image", slog.String("image", imageID))

	err := r.execCommand(ctx, "rmi", "--force", imageID)
	if err != nil {
		return fmt.Errorf("failed to remove image: %w", err)
	}

	return nil
}

// containerInspect represents the structure of container inspect JSON output.
type containerInspect struct {
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
		ExposedPorts map[string]struct{} `json:"ExposedPorts"`
	} `json:"Config"`
	NetworkSettings struct {
		Ports map[string][]struct {
			HostIP   string `json:"HostIp"`
			HostPort string `json:"HostPort"`
		} `json:"Ports"`
	} `json:"NetworkSettings"`
}

// imageInspect represents the structure of image inspect JSON output.
type imageInspect struct {
	ID     string `json:"Id"`
	Config struct {
		ExposedPorts map[string]struct{} `json:"ExposedPorts"`
	} `json:"Config"`
}

// GetContainerInfo returns detailed information about a container.
func (r *Runtime) GetContainerInfo(ctx context.Context, containerID string, containerPort int) (*Info, error) {
	output, err := r.execCommandOutput(ctx, "inspect", containerID)
	if err != nil {
		return nil, fmt.Errorf("failed to inspect container: %w", err)
	}

	var inspects []containerInspect
	if err := json.Unmarshal([]byte(output), &inspects); err != nil {
		return nil, fmt.Errorf("failed to parse inspect output: %w", err)
	}

	if len(inspects) == 0 {
		return nil, ErrContainerNotFound
	}

	inspect := inspects[0]

	startedAt, err := time.Parse(time.RFC3339Nano, inspect.State.StartedAt)
	if err != nil {
		r.logger.Warn("Failed to parse StartedAt timestamp",
			slog.String("container", containerID),
			slog.String("timestamp", inspect.State.StartedAt))
		startedAt = time.Time{}
	}

	deployedAt := startedAt
	if deployedAtStr, ok := inspect.Config.Labels["dewy.deployed_at"]; ok {
		if t, err := time.Parse(time.RFC3339, deployedAtStr); err == nil {
			deployedAt = t
		}
	}

	ipPort := ""
	if containerPort > 0 {
		portSpec := fmt.Sprintf("%d/tcp", containerPort)
		if portBindings, ok := inspect.NetworkSettings.Ports[portSpec]; ok && len(portBindings) > 0 {
			ipPort = fmt.Sprintf("%s:%s", portBindings[0].HostIP, portBindings[0].HostPort)
		}
	} else {
		for _, portBindings := range inspect.NetworkSettings.Ports {
			if len(portBindings) > 0 {
				ipPort = fmt.Sprintf("%s:%s", portBindings[0].HostIP, portBindings[0].HostPort)
				break
			}
		}
	}

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
func (r *Runtime) ListContainersByLabels(ctx context.Context, labels map[string]string, containerPort int) ([]*Info, error) {
	containerIDs, err := r.FindContainersByLabel(ctx, labels)
	if err != nil {
		return nil, fmt.Errorf("failed to find containers: %w", err)
	}

	infos := make([]*Info, 0, len(containerIDs))
	for _, containerID := range containerIDs {
		info, err := r.GetContainerInfo(ctx, containerID, containerPort)
		if err != nil {
			r.logger.Warn("Failed to get container info",
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
func (r *Runtime) GetImageExposedPorts(ctx context.Context, imageRef string) ([]int, error) {
	output, err := r.execCommandOutput(ctx, "image", "inspect", imageRef)
	if err != nil {
		return nil, fmt.Errorf("failed to inspect image: %w", err)
	}

	var inspects []imageInspect
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

	ports := make([]int, 0, len(inspect.Config.ExposedPorts))
	for portSpec := range inspect.Config.ExposedPorts {
		if !strings.HasSuffix(portSpec, "/tcp") {
			continue
		}

		portStr := strings.TrimSuffix(portSpec, "/tcp")
		port, err := strconv.Atoi(portStr)
		if err != nil {
			r.logger.Warn("Failed to parse exposed port",
				slog.String("port_spec", portSpec),
				slog.String("error", err.Error()))
			continue
		}

		ports = append(ports, port)
	}

	sort.Ints(ports)

	r.logger.Debug("Detected exposed ports from image",
		slog.String("image", imageRef),
		slog.Any("ports", ports))

	return ports, nil
}
