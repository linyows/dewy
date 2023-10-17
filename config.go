package dewy

import (
	"os"

	starter "github.com/lestrrat-go/server-starter"
)

// Command for CLI.
type Command int

const (
	// SERVER command.
	SERVER Command = iota
	// ASSETS command.
	ASSETS
)

// String to string for Command.
func (c Command) String() string {
	switch c {
	case SERVER:
		return "server"
	case ASSETS:
		return "assets"
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

// Config struct.
type Config struct {
	Command          Command
	Registry         string
	ArtifactName     string
	PreRelease       bool
	Cache            CacheConfig
	Starter          starter.Config
	BeforeDeployHook string
	AfterDeployHook  string
}

// OverrideWithEnv overrides by environments.
func (c *Config) OverrideWithEnv() {
	// Support env GITHUB_ENDPOINT.
	if e := os.Getenv("GITHUB_ENDPOINT"); e != "" {
		os.Setenv("GITHUB_API_URL", e)
	}

	if a := os.Getenv("GITHUB_ARTIFACT"); a != "" {
		c.ArtifactName = a
	}
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
