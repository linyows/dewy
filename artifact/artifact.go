package artifact

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"strings"

	"github.com/linyows/dewy/internal/scheme"
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

// factoryFn constructs an Artifact from a parsed URL plus the resolved
// per-call options (puller, etc.).
type factoryFn func(ctx context.Context, url string, logger *slog.Logger, o *options) (Artifact, error)

var factories = map[string]factoryFn{
	scheme.GHR: func(ctx context.Context, url string, logger *slog.Logger, _ *options) (Artifact, error) {
		return NewGHR(ctx, url, logger)
	},
	scheme.S3: func(ctx context.Context, url string, logger *slog.Logger, _ *options) (Artifact, error) {
		return NewS3(ctx, url, logger)
	},
	scheme.GS: func(ctx context.Context, url string, logger *slog.Logger, _ *options) (Artifact, error) {
		return NewGS(ctx, url, logger)
	},
	scheme.OCI: func(ctx context.Context, url string, logger *slog.Logger, o *options) (Artifact, error) {
		return NewOCI(ctx, url, o.puller, logger)
	},
}

func New(ctx context.Context, url string, logger *slog.Logger, opts ...Option) (Artifact, error) {
	var o options
	for _, opt := range opts {
		opt(&o)
	}
	splitted := strings.SplitN(url, "://", 2)
	if f, ok := factories[splitted[0]]; ok {
		return f(ctx, url, logger, &o)
	}
	return nil, fmt.Errorf("unsupported scheme: %s", url)
}
