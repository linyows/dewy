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
	// Check if image exists locally first (using direct exec to avoid ERROR log for expected miss)
	// #nosec G204 - imageRef is validated by caller
	inspectCmd := exec.CommandContext(ctx, r.cmd, "image", "inspect", imageRef)
	r.logger.Debug("Executing command",
		slog.String("cmd", r.cmd),
		slog.Any("args", []string{"image", "inspect", imageRef}))
	localErr := inspectCmd.Run()
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

// imageRepositoryFromRef derives the repository part of an image reference.
// It removes any digest component (after '@') and then strips a tag (after ':')
// only if the colon appears after the last slash, to avoid confusing registry ports
// with tag separators.
func imageRepositoryFromRef(imageRef string) string {
	ref := imageRef

	// Strip digest if present (e.g., "repo@sha256:abcdef...")
	if at := strings.Index(ref, "@"); at != -1 {
		ref = ref[:at]
	}

	lastSlash := strings.LastIndex(ref, "/")
	lastColon := strings.LastIndex(ref, ":")

	if lastSlash == -1 {
		// No slash: any colon is a tag separator
		if lastColon != -1 {
			return ref[:lastColon]
		}
		return ref
	}

	// Only treat colon as tag separator if it appears after the last slash
	if lastColon > lastSlash {
		return ref[:lastColon]
	}

	return ref
}

// CleanupOldImages removes old container images, keeping only the most recent ones.
func (r *Runtime) CleanupOldImages(ctx context.Context, imageRef string, keepCount int) error {
	repository := imageRepositoryFromRef(imageRef)

	images, err := r.ListImages(ctx, repository)
	if err != nil {
		return fmt.Errorf("failed to list images: %w", err)
	}

	if len(images) <= keepCount {
		r.logger.Debug("No old images to clean up",
			slog.String("repository", repository),
			slog.Int("count", len(images)),
			slog.Int("keep", keepCount))
		return nil
	}

	// Sort images by creation time (newest first)
	sort.Slice(images, func(i, j int) bool {
		return images[i].Created.After(images[j].Created)
	})

	// Remove old images (keep only the most recent keepCount)
	for i, img := range images {
		if i < keepCount {
			r.logger.Debug("Keeping image",
				slog.String("id", img.ID),
				slog.String("tag", img.Tag),
				slog.Time("created", img.Created))
			continue
		}

		r.logger.Info("Removing old image",
			slog.String("id", img.ID),
			slog.String("tag", img.Tag),
			slog.Time("created", img.Created))

		if err := r.RemoveImage(ctx, img.ID); err != nil {
			r.logger.Warn("Failed to remove image",
				slog.String("id", img.ID),
				slog.String("error", err.Error()))
			continue
		}
	}

	return nil
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
