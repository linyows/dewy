package artifact

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"strings"
)

const (
	ghrScheme = "ghr"
	s3Scheme  = "s3"
	gsScheme  = "gs"
	imgScheme = "img"
)

// Fetcher is the interface that wraps the Fetch method.
type Artifact interface {
	// Fetch fetches the artifact from the storage.
	Download(ctx context.Context, w io.Writer) error
}

// Option configures artifact creation.
type Option func(*options)

type options struct {
	puller Puller
}

// WithPuller sets the container image puller for OCI artifacts.
func WithPuller(p Puller) Option {
	return func(o *options) {
		o.puller = p
	}
}

func New(ctx context.Context, url string, logger *slog.Logger, opts ...Option) (Artifact, error) {
	var o options
	for _, opt := range opts {
		opt(&o)
	}

	splitted := strings.SplitN(url, "://", 2)

	switch splitted[0] {
	case ghrScheme:
		return NewGHR(ctx, url, logger)

	case s3Scheme:
		return NewS3(ctx, url, logger)

	case gsScheme:
		return NewGS(ctx, url, logger)

	case imgScheme:
		return NewOCI(ctx, url, o.puller, logger)
	}

	return nil, fmt.Errorf("unsupported scheme: %s", url)
}
