package dewy

import (
	"fmt"
	"os"
	"path"

	starter "github.com/lestrrat-go/server-starter"
)

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

type CacheConfig struct {
	Type       CacheType
	Expiration int
}

type RepositoryProvider int

const (
	GITHUB RepositoryProvider = iota
)

func (r RepositoryProvider) String() string {
	switch r {
	case GITHUB:
		return "github"
	default:
		return "unknown"
	}
}

type RepositoryConfig struct {
	Provider RepositoryProvider
	Owner    string
	Name     string
	Token    string
	Endpoint string
	Artifact string
}

func (r RepositoryConfig) String() string {
	return path.Join(fmt.Sprintf("%s:%s", r.Provider.String(), r.Owner), r.Name)
}

type Config struct {
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
