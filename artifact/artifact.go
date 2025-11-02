package artifact

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"strings"
)

const (
	ghrScheme    = "ghr"
	s3Scheme     = "s3"
	gsScheme     = "gs"
	dockerScheme = "docker"
	ociScheme    = "oci"
)

// Fetcher is the interface that wraps the Fetch method.
type Artifact interface {
	// Fetch fetches the artifact from the storage.
	Download(ctx context.Context, w io.Writer) error
}

func New(ctx context.Context, url string, logger *slog.Logger) (Artifact, error) {
	splitted := strings.SplitN(url, "://", 2)

	switch splitted[0] {
	case ghrScheme:
		return NewGHR(ctx, url, logger)

	case s3Scheme:
		return NewS3(ctx, url, logger)

	case gsScheme:
		return NewGS(ctx, url, logger)

	case dockerScheme, ociScheme:
		return NewOCI(ctx, url, logger)
	}

	return nil, fmt.Errorf("unsupported scheme: %s", url)
}
