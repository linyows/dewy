package container

import (
	"context"
	"errors"
	"time"
)

var (
	// ErrContainerNotFound is returned when a container is not found.
	ErrContainerNotFound = errors.New("container not found")
	// ErrRuntimeNotFound is returned when the container runtime is not found.
	ErrRuntimeNotFound = errors.New("container runtime not found")
)

// Runtime is the interface that wraps container runtime operations.
type Runtime interface {
	// Pull pulls the image from registry.
	Pull(ctx context.Context, imageRef string) error

	// Run starts a new container and returns the container ID.
	Run(ctx context.Context, opts RunOptions) (string, error)

	// Stop stops a running container gracefully.
	Stop(ctx context.Context, containerID string, timeout time.Duration) error

	// Remove removes a container.
	Remove(ctx context.Context, containerID string) error

	// FindContainerByLabel finds a container by labels.
	FindContainerByLabel(ctx context.Context, labels map[string]string) (string, error)

	// FindContainersByLabel finds all containers matching the given labels.
	FindContainersByLabel(ctx context.Context, labels map[string]string) ([]string, error)

	// UpdateLabel updates a container's label.
	UpdateLabel(ctx context.Context, containerID, key, value string) error

	// GetMappedPort returns the host port mapped to the container port.
	GetMappedPort(ctx context.Context, containerID string, containerPort int) (int, error)

	// ListImages returns a list of images matching the given repository.
	ListImages(ctx context.Context, repository string) ([]ImageInfo, error)

	// RemoveImage removes an image by ID.
	RemoveImage(ctx context.Context, imageID string) error

	// GetContainerInfo returns detailed information about a container.
	GetContainerInfo(ctx context.Context, containerID string, containerPort int) (*ContainerInfo, error)

	// ListContainersByLabels returns detailed information about containers matching the given labels.
	ListContainersByLabels(ctx context.Context, labels map[string]string, containerPort int) ([]*ContainerInfo, error)
}

// RunOptions contains options for running a container.
type RunOptions struct {
	Image        string
	Name         string
	AppName      string   // Application name for default naming
	ReplicaIndex int      // Replica index for naming (0-based)
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

// ContainerInfo represents detailed container information for admin API.
type ContainerInfo struct {
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
