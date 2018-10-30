package dewy

import (
	"os"
	"path"

	starter "github.com/lestrrat-go/server-starter"
)

// Command
type Command int

const (
	SERVER Command = iota
	ASSETS
)

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

// CacheType
type CacheType int

const (
	NONE CacheType = iota
	FILE
)

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

// CacheConfig
type CacheConfig struct {
	Type       CacheType
	Expiration int
}

// RepositoryProvider
type RepositoryProvider int

const (
	GITHUB RepositoryProvider = iota
)

func (r RepositoryProvider) String() string {
	switch r {
	case GITHUB:
		return "github.com"
	default:
		return "unknown"
	}
}

// RepositoryConfig
type RepositoryConfig struct {
	Provider RepositoryProvider
	Owner    string
	Name     string
	Token    string
	Endpoint string
	Artifact string
}

func (r RepositoryConfig) String() string {
	return path.Join(r.Provider.String(), r.Owner, r.Name)
}

// Config
type Config struct {
	Command    Command
	Repository RepositoryConfig
	Cache      CacheConfig
	Starter    starter.Config
}

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

func DefaultConfig() Config {
	return Config{
		Cache: CacheConfig{
			Type:       FILE,
			Expiration: 10,
		},
	}
}
