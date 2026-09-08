package dewy

import (
	"time"

	"github.com/linyows/dewy/container"
	starter "github.com/linyows/server-starter"
)

// Command for CLI.
type Command int

const (
	// SERVER command.
	SERVER Command = iota
	// ASSETS command.
	ASSETS
	// CONTAINER command.
	CONTAINER
)

// String to string for Command.
func (c Command) String() string {
	switch c {
	case SERVER:
		return "server"
	case ASSETS:
		return "assets"
	case CONTAINER:
		return "container"
	default:
		return "unknown"
	}
}

// CacheType for cache type.
type CacheType int

const (
	// NONE cache type.
	NONE CacheType = iota
	// FILE cache type.
	FILE
)

// String to string for CacheType.
func (c CacheType) String() string {
	switch c {
	case NONE:
		return "none"
	case FILE:
		return "file"
	default:
		return "unknown"
	}
}

// CacheConfig struct.
type CacheConfig struct {
	Type       CacheType
	Expiration int
	// URL selects a cache backend by scheme.
	// Examples: "" (default file), "file:///path/to/cache",
	// "s3://<region>/<bucket>/<prefix>", "gs://<bucket>/<prefix>".
	URL string
}

// ContainerConfig struct for container command.
type ContainerConfig struct {
	Name             string
	PortMappings     []container.PortMapping // Port mappings between proxy and container (ContainerPort==0 means auto-detect from image EXPOSE)
	Replicas         int                     // Number of container replicas to run (default: 1)
	Command          []string                // Command and arguments to pass to container
	ExtraArgs        []string                // Extra docker run arguments from -- separator
	HealthPath       string
	HealthTimeout    time.Duration
	DrainTime        time.Duration
	Runtime          string        // "docker" or "podman"
	ProxyIdleTimeout time.Duration // Idle timeout for TCP proxy connections (0 = disabled)
}

// HealthConfig configures the post-deploy health check for the server
// command. The container command keeps its own copy of these settings in
// ContainerConfig because it probes each replica through the runtime rather
// than a fixed local port.
type HealthConfig struct {
	// Path is the HTTP path probed on the first configured port after the
	// managed server starts or restarts. An empty Path disables the check.
	Path string
	// Timeout is the overall budget for the probe, covering every attempt.
	// Zero falls back to defaultHealthCheckTotalTimeout.
	Timeout time.Duration
	// NoRollback keeps the new release in place when the probe fails. The
	// failed version is still recorded so it is not deployed again, and the
	// failure is still notified.
	NoRollback bool
}

// Config struct.
type Config struct {
	Command          Command
	Registry         string
	Notifier         string
	Port             int // Port for HTTP server (used by both server and container commands)
	AdminPort        int // Port for admin API (container command only, default: 17539)
	Cache            CacheConfig
	Starter          starter.Config
	Container        *ContainerConfig
	BeforeDeployHook string
	AfterDeployHook  string
	Health           HealthConfig // Post-deploy health check (server command)
	Slot             string       // Deployment slot for blue/green deployment (e.g., "blue", "green")
	// Channel restricts deployments to versions in one release channel, named
	// by the pre-release identifier of the tag (e.g. "canary" for
	// v1.2.3-canary.1). "stable" tracks final releases only. Empty tracks
	// every version the pre-release setting admits.
	Channel string
	CalVer  string // CalVer format for version identification (e.g., "YYYY.0M.MICRO")
	// MaxBackoffInterval bounds how far consecutive failures may stretch the
	// polling interval. Zero keeps it fixed.
	MaxBackoffInterval time.Duration
	*Info
}

// DefaultConfig returns default Config.
func DefaultConfig() Config {
	return Config{
		Cache: CacheConfig{
			Type:       FILE,
			Expiration: 10,
		},
	}
}
