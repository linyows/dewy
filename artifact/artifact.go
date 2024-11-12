package artifact

import (
	"fmt"
	"io"
	"strings"
)

const (
	ghrScheme = "ghr"
	s3Scheme  = "s3"
	gcsScheme = "gcs"
)

// Fetcher is the interface that wraps the Fetch method.
type Artifact interface {
	// Fetch fetches the artifact from the storage.
	Fetch(url string, w io.Writer) error
}

func Fetch(url string, w io.Writer) error {
	splitted := strings.SplitN(url, "://", 2)

	switch splitted[0] {
	case ghrScheme:
		g, err := NewGHR()
		if err != nil {
			return err
		}
		return g.Fetch(url, w)

	case s3Scheme:
		s, err := NewS3(splitted[1])
		if err != nil {
			return err
		}
		return s.Fetch(url, w)

	case gcsScheme:
		r, err := NewGCS()
		if err != nil {
			return err
		}
		return r.Fetch(url, w)
	}

	return fmt.Errorf("unsupported scheme: %s", url)
}
