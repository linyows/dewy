package repo

import (
	"path"

	"github.com/linyows/dewy/kvs"
)

// Repo interface for repository
type Repo interface {
	String() string
	Fetch() error
	GetDeploySourceKey() (string, error)
	RecordShipping() error
	ReleaseTag() string
	ReleaseURL() string
	OwnerURL() string
	OwnerIconURL() string
	URL() string
}

// Provider for repository
type Provider int

const (
	// GITHUB repository provider
	GITHUB Provider = iota
)

// String to string for Provider
func (r Provider) String() string {
	switch r {
	case GITHUB:
		return "github.com"
	default:
		return "unknown"
	}
}

// Config struct
type Config struct {
	Provider
	Owner                 string
	Name                  string
	Token                 string
	Endpoint              string
	Artifact              string
	PreRelease            bool
	DisableRecordShipping bool // FIXME: For testing. Remove this.
}

// String to string for Config
func (c Config) String() string {
	return path.Join(c.Provider.String(), c.Owner, c.Name)
}

// New returns repo
func New(c Config, d kvs.KVS) Repo {
	switch c.Provider {
	case GITHUB:
		return NewGithubRelease(c, d)
	default:
		panic("no repository provider")
	}
}
