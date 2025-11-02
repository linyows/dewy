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

	// NetworkConnect connects a container to a network with alias.
	NetworkConnect(ctx context.Context, network, containerID, alias string) error

	// NetworkDisconnect disconnects a container from a network.
	NetworkDisconnect(ctx context.Context, network, containerID string) error

	// FindContainerByLabel finds a container by labels.
	FindContainerByLabel(ctx context.Context, labels map[string]string) (string, error)

	// UpdateLabel updates a container's label.
	UpdateLabel(ctx context.Context, containerID, key, value string) error
}

// RunOptions contains options for running a container.
type RunOptions struct {
	Image        string
	Name         string
	Network      string
	NetworkAlias string // Network alias for the container in the network
	Env          []string
	Volumes      []string
	Labels       map[string]string
	Ports        []string // Port mappings in format "host:container" (e.g., "8080:8080")
	Detach       bool
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

// DeployOptions contains options for deploying a container.
type DeployOptions struct {
	ImageRef     string
	AppName      string
	Network      string
	NetworkAlias string
	Env          []string
	Volumes      []string
	Ports        []string // Port mappings in format "host:container" (e.g., "8080:8080")
	HealthCheck  HealthCheckFunc
}

// HealthCheckFunc is a function type for health checking.
type HealthCheckFunc func(ctx context.Context, containerID string) error
