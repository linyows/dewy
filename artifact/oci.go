package artifact

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/url"
)

// Puller is the interface for pulling container images.
type Puller interface {
	Pull(ctx context.Context, imageRef string) error
}

// OCI implements Artifact interface for OCI/Docker images.
type OCI struct {
	ImageRef string
	puller   Puller
	logger   *slog.Logger
}

// NewOCI creates a new OCI artifact.
func NewOCI(ctx context.Context, u string, puller Puller, logger *slog.Logger) (*OCI, error) {
	// Parse URL: img://registry/repo:tag
	ur, err := url.Parse(u)
	if err != nil {
		return nil, err
	}

	// Construct image reference: registry/repo:tag
	imageRef := ur.Host + ur.Path

	return &OCI{
		ImageRef: imageRef,
		puller:   puller,
		logger:   logger,
	}, nil
}

// Download pulls the container image using the container runtime.
// The io.Writer parameter receives a confirmation message after the pull.
// The actual image data is stored in the runtime's image store.
func (o *OCI) Download(ctx context.Context, w io.Writer) error {
	if err := o.puller.Pull(ctx, o.ImageRef); err != nil {
		return fmt.Errorf("pull failed: %w", err)
	}

	if w != nil {
		fmt.Fprintf(w, "Pulled image: %s\n", o.ImageRef)
	}

	return nil
}
