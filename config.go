package dewy

import (
	"time"

	starter "github.com/linyows/server-starter"
)

// Command for CLI.
type Command int

const (
	// SERVER command.
	SERVER Command = iota
	// ASSETS command.
	ASSETS
	// IMAGE command.
	IMAGE
)

// String to string for Command.
func (c Command) String() string {
	switch c {
	case SERVER:
		return "server"
	case ASSETS:
		return "assets"
	case IMAGE:
		return "image"
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
}

// ContainerConfig struct for container command.
type ContainerConfig struct {
	Name          string
	Network       string
	NetworkAlias  string
	ContainerPort int
	Env           []string
	Volumes       []string
	HealthPath    string
	HealthTimeout time.Duration
	DrainTime     time.Duration
	Runtime       string // "docker" or "podman"
	Proxy         bool   // Enable reverse proxy
	ProxyPort     int    // Proxy port (default: 80)
	ProxyImage    string // Proxy image (default: "caddy:2-alpine")
}

// Config struct.
type Config struct {
	Command          Command
	Registry         string
	Notifier         string
	Cache            CacheConfig
	Starter          starter.Config
	Container        *ContainerConfig
	BeforeDeployHook string
	AfterDeployHook  string
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
