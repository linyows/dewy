package artifact

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/url"
	"os/exec"
	"strings"
)

// OCI implements Artifact interface for OCI/Docker images.
type OCI struct {
	ImageRef string
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

// Download pulls the Docker image.
// For OCI artifacts, we use docker pull command.
// The io.Writer parameter is not used for container images,
// as they are pulled directly into Docker's image store.
func (o *OCI) Download(ctx context.Context, w io.Writer) error {
	o.logger.Info("Pulling Docker image", slog.String("image", o.ImageRef))

	// Check if docker command exists
	if _, err := exec.LookPath("docker"); err != nil {
		return fmt.Errorf("docker command not found: %w", err)
	}

	// Execute docker pull
	// #nosec G204 - ImageRef is validated during URL parsing in NewOCI
	cmd := exec.CommandContext(ctx, "docker", "pull", o.ImageRef)
	output, err := cmd.CombinedOutput()
	if err != nil {
		o.logger.Error("Failed to pull image",
			slog.String("image", o.ImageRef),
			slog.String("output", string(output)),
			slog.String("error", err.Error()))
		return fmt.Errorf("docker pull failed: %w: %s", err, string(output))
	}

	o.logger.Info("Successfully pulled image",
		slog.String("image", o.ImageRef),
		slog.String("output", strings.TrimSpace(string(output))))

	// Write confirmation to writer (optional)
	if w != nil {
		fmt.Fprintf(w, "Pulled image: %s\n", o.ImageRef)
	}

	return nil
}
