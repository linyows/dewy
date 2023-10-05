package repo

import (
	"errors"
	"io"
	"path"
)

// CurrentRequest is the request to get the current artifact.
type CurrentRequest struct {
	// ArtifactName is the name of the artifact to fetch.
	// FIXME: If possible, ArtifactName should be optional.
	ArtifactName string
}

// CurrentResponse is the response to get the current artifact.
type CurrentResponse struct {
	// Tag uniquely identifies the artifact concerned.
	Tag string
	// ArtifactURL is the URL to download the artifact.
	// The URL is not only "https://"
	ArtifactURL string
}

// Repo interface for repository
type Repo interface {
	String() string
	RecordShipping() error
	ReleaseTag() string
	ReleaseURL() string
	OwnerURL() string
	OwnerIconURL() string
	Current(req *CurrentRequest) (*CurrentResponse, error)
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
