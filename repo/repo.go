package repo

import (
	"errors"
	"path"

	"github.com/linyows/dewy/registory"
	"github.com/linyows/dewy/storage"
)

// Repo interface for repository.
type Repo interface {
	registory.Registory
	storage.Fetcher
	String() string
	OwnerURL() string
	OwnerIconURL() string
	URL() string
}

// Provider for repository.
type Provider int

const (
	// GITHUB repository provider.
	GITHUB Provider = iota
)

// String to string for Provider.
func (r Provider) String() string {
	switch r {
	case GITHUB:
		return "github.com"
	default:
		return "unknown"
	}
}

// Config struct.
type Config struct {
	Provider
	Owner                 string
	Repo                  string
	Artifact              string
	PreRelease            bool
	DisableRecordShipping bool // FIXME: For testing. Remove this.
}

// String to string for Config.
func (c Config) String() string {
	return path.Join(c.Provider.String(), c.Owner, c.Repo)
}

// New returns repo.
func New(c Config) (Repo, error) {
	switch c.Provider {
	case GITHUB:
		return NewGithubRelease(c)
	default:
		return nil, errors.New("no repository provider")
	}
}
