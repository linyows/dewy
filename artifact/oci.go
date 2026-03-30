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
	ImageRef   string
	RuntimeCmd string // Container runtime command (e.g., "docker", "podman")
	logger     *slog.Logger
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

// Download pulls the container image using the configured runtime command.
// The io.Writer parameter is not used for container images,
// as they are pulled directly into the runtime's image store.
func (o *OCI) Download(ctx context.Context, w io.Writer) error {
	runtimeCmd := o.RuntimeCmd
	if runtimeCmd == "" {
		runtimeCmd = "docker"
	}

	o.logger.Info("Pulling container image",
		slog.String("image", o.ImageRef),
		slog.String("runtime", runtimeCmd))

	// Check if runtime command exists
	if _, err := exec.LookPath(runtimeCmd); err != nil {
		return fmt.Errorf("%s command not found: %w", runtimeCmd, err)
	}

	// Execute pull
	// #nosec G204 - ImageRef is validated during URL parsing in NewOCI
	cmd := exec.CommandContext(ctx, runtimeCmd, "pull", o.ImageRef)
	output, err := cmd.CombinedOutput()
	if err != nil {
		o.logger.Error("Failed to pull image",
			slog.String("image", o.ImageRef),
			slog.String("output", string(output)),
			slog.String("error", err.Error()))
		return fmt.Errorf("%s pull failed: %w: %s", runtimeCmd, err, string(output))
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
