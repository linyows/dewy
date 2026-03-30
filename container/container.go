package container

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"
)

// supportedRuntimes is the allowlist of container runtime commands.
var supportedRuntimes = map[string]bool{
	"docker": true,
	"podman": true,
}

var (
	// ErrContainerNotFound is returned when a container is not found.
	ErrContainerNotFound = errors.New("container not found")
	// ErrRuntimeNotFound is returned when the container runtime is not found.
	ErrRuntimeNotFound = errors.New("container runtime not found")
)

// New creates a new container runtime for the specified command (e.g., "docker", "podman").
func New(runtime string, logger *slog.Logger, drainTime time.Duration) (*Runtime, error) {
	if !supportedRuntimes[runtime] {
		return nil, fmt.Errorf("unsupported runtime %q: must be one of docker, podman", runtime)
	}
	return newCLIRuntime(runtime, logger, drainTime)
}

// RunOptions contains options for running a container.
type RunOptions struct {
	Image        string
	Name         string
	AppName      string // Application name for default naming
	ReplicaIndex int    // Replica index for naming (0-based)
	Labels       map[string]string
	Ports        []string // Port mappings in format "host:container" or "127.0.0.1::container" for random localhost port
	Detach       bool
	Command      []string // Command and arguments to pass to container
	ExtraArgs    []string // Extra docker run arguments (from -- separator)
}

// Container represents container information.
type Container struct {
	ID      string
	Name    string
	Image   string
	Status  string
	Labels  map[string]string
	Created time.Time
}

// Info represents detailed container information for admin API.
type Info struct {
	ID         string            `json:"id"`
	Name       string            `json:"name"`
	Image      string            `json:"image"`
	Status     string            `json:"status"`
	IPPort     string            `json:"ip_port"`     // Mapped host address (e.g., "127.0.0.1:49152")
	StartedAt  time.Time         `json:"started_at"`  // When container was started
	DeployedAt time.Time         `json:"deployed_at"` // When deployed by dewy
	Labels     map[string]string `json:"labels"`
}

// ImageInfo represents container image information.
type ImageInfo struct {
	ID         string
	Repository string
	Tag        string
	Created    time.Time
	Size       int64
}

// DeployOptions contains options for deploying a container.
type DeployOptions struct {
	ImageRef      string
	AppName       string
	ContainerPort int      // Container port to expose (will be mapped to random localhost port)
	Ports         []string // Explicit port mappings in format "host:container" (e.g., "8080:8080")
	Command       []string // Command and arguments to pass to container
	ExtraArgs     []string // Extra docker run arguments (from -- separator)
	HealthCheck   HealthCheckFunc
}

// HealthCheckFunc is a function type for health checking.
type HealthCheckFunc func(ctx context.Context, containerID string) error

// DeployContainerCallback is called after health check passes but before stopping old container.
type DeployContainerCallback func(containerID string) error
