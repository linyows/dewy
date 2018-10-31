package dewy

import (
	"os"
	"path"

	starter "github.com/lestrrat-go/server-starter"
)

// Command for CLI
type Command int

const (
	// SERVER command
	SERVER Command = iota
	// ASSETS command
	ASSETS
)

// String to string for Command
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

// CacheType for cache type
type CacheType int

const (
	// NONE cache type
	NONE CacheType = iota
	// FILE cache type
	FILE
)

// String to string for CacheType
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

// CacheConfig struct
type CacheConfig struct {
	Type       CacheType
	Expiration int
}

// RepositoryProvider for repository provider
type RepositoryProvider int

const (
	// GITHUB repository provider
	GITHUB RepositoryProvider = iota
)

// String to string for RepositoryProvider
func (r RepositoryProvider) String() string {
	switch r {
	case GITHUB:
		return "github.com"
	default:
		return "unknown"
	}
}

// RepositoryConfig struct
type RepositoryConfig struct {
	Provider RepositoryProvider
	Owner    string
	Name     string
	Token    string
	Endpoint string
	Artifact string
}

// String to string for RepositoryConfig
func (r RepositoryConfig) String() string {
	return path.Join(r.Provider.String(), r.Owner, r.Name)
}

// Config struct
type Config struct {
	Command    Command
	Repository RepositoryConfig
	Cache      CacheConfig
	Starter    starter.Config
}

// OverrideWithEnv overrides by environments
func (c *Config) OverrideWithEnv() {
	if c.Repository.Provider == GITHUB {
		githubToken := os.Getenv("GITHUB_TOKEN")
		if githubToken != "" {
			c.Repository.Token = githubToken
		}
		githubEndpoint := os.Getenv("GITHUB_ENDPOINT")
		if githubEndpoint != "" {
			c.Repository.Endpoint = githubEndpoint
		}
		githubArtifact := os.Getenv("GITHUB_ARTIFACT")
		if githubArtifact != "" {
			c.Repository.Artifact = githubArtifact
		}
	}
}

// DefaultConfig returns default Config
func DefaultConfig() Config {
	return Config{
		Cache: CacheConfig{
			Type:       FILE,
			Expiration: 10,
		},
	}
}
