package repo

import (
	"errors"
	"io"
	"path"
	"time"
)

// Repo interface for repository
type Repo interface {
	String() string
	Fetch() error
	RecordShipping() error
	ReleaseTag() string
	ReleaseURL() string
	OwnerURL() string
	OwnerIconURL() string
	LatestKey() (string, time.Time)
	Download(w io.Writer) error
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
	Artifact              string
	PreRelease            bool
	DisableRecordShipping bool // FIXME: For testing. Remove this.
}

// String to string for Config
func (c Config) String() string {
	return path.Join(c.Provider.String(), c.Owner, c.Name)
}

// New returns repo
func New(c Config) (Repo, error) {
	switch c.Provider {
	case GITHUB:
		return NewGithubRelease(c)
	default:
		return nil, errors.New("no repository provider")
	}
}
