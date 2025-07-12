package dewy

import (
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
	Notify           string
	Cache            CacheConfig
	Starter          starter.Config
	BeforeDeployHook string
	AfterDeployHook  string
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
