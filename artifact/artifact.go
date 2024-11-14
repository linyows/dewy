package artifact

import (
	"context"
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
	Download(ctx context.Context, w io.Writer) error
}

func New(ctx context.Context, url string) (Artifact, error) {
	splitted := strings.SplitN(url, "://", 2)

	switch splitted[0] {
	case ghrScheme:
		return NewGHR(ctx, url)

	case s3Scheme:
		return NewS3(ctx, url)

	case gcsScheme:
		return NewGCS(ctx, url)
	}

	return nil, fmt.Errorf("unsupported scheme: %s", url)
}
