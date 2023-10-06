package storage

import (
	"fmt"
	"io"
	"strings"

	ghrelease "github.com/linyows/dewy/storage/github_release"
)

// Fetcher is the interface that wraps the Fetch method.
type Fetcher interface {
	// Fetch fetches the artifact from the storage.
	Fetch(urlstr string, w io.Writer) error
}

var _ Fetcher = (*ghrelease.GithubRelease)(nil)

func Fetch(urlstr string, w io.Writer) error {
	pair := strings.SplitN(urlstr, "://", 2)
	scheme := pair[0]
	switch scheme {
	case ghrelease.Scheme:
		r, err := ghrelease.New()
		if err != nil {
			return err
		}
		return r.Fetch(urlstr, w)
	}
	return fmt.Errorf("unsupported scheme: %s", urlstr)
}
