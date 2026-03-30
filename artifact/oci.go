package artifact

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/url"

	"github.com/linyows/dewy/container"
)

// OCI implements Artifact interface for OCI/Docker images.
type OCI struct {
	ImageRef string
	runtime  *container.Runtime
	logger   *slog.Logger
}

// NewOCI creates a new OCI artifact.
func NewOCI(ctx context.Context, u string, logger *slog.Logger) (*OCI, error) {
	// Parse URL: container://registry/repo:tag
	ur, err := url.Parse(u)
	if err != nil {
		return nil, err
	}

	// Construct image reference: registry/repo:tag
	imageRef := ur.Host + ur.Path

	return &OCI{
		ImageRef: imageRef,
		logger:   logger,
	}, nil
}

// SetRuntime sets the container runtime for image pulling.
func (o *OCI) SetRuntime(rt *container.Runtime) {
	o.runtime = rt
}

// Download pulls the container image using the container runtime.
// The io.Writer parameter receives a confirmation message after the pull.
// The actual image data is stored in the runtime's image store.
func (o *OCI) Download(ctx context.Context, w io.Writer) error {
	if o.runtime == nil {
		return fmt.Errorf("container runtime is not set: call SetRuntime before Download")
	}

	if err := o.runtime.Pull(ctx, o.ImageRef); err != nil {
		return fmt.Errorf("pull failed: %w", err)
	}

	if w != nil {
		fmt.Fprintf(w, "Pulled image: %s\n", o.ImageRef)
	}

	return nil
}
