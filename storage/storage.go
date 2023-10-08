package storage

import (
	"fmt"
	"io"
	"strings"

	"github.com/linyows/dewy/storage/gcs"
	ghrelease "github.com/linyows/dewy/storage/github_release"
	"github.com/linyows/dewy/storage/s3"
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
	case s3.Scheme:
		r, err := s3.New()
		if err != nil {
			return err
		}
		return r.Fetch(urlstr, w)
	case gcs.Scheme, gcs.SchemeShort:
		r, err := gcs.New()
		if err != nil {
			return err
		}
		return r.Fetch(urlstr, w)
	}
	return fmt.Errorf("unsupported scheme: %s", urlstr)
}
