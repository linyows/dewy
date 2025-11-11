package container

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"
)

// ProxyOptions contains options for starting a reverse proxy container.
type ProxyOptions struct {
	Image        string // Proxy container image (e.g., "caddy:2-alpine")
	Name         string // Proxy container name (e.g., "<app_name>-proxy")
	Network      string // Docker network
	ProxyPort    int    // External port to expose
	UpstreamHost string // Upstream host (e.g., "dewy-current")
	UpstreamPort int    // Upstream port (e.g., 8080)
}

// StartProxy starts a Caddy reverse proxy container.
func (d *Docker) StartProxy(ctx context.Context, opts ProxyOptions) (string, error) {
	d.logger.Info("Starting reverse proxy container",
		slog.String("name", opts.Name),
		slog.String("image", opts.Image),
		slog.Int("proxy_port", opts.ProxyPort),
		slog.String("upstream", fmt.Sprintf("%s:%d", opts.UpstreamHost, opts.UpstreamPort)))

	// Build docker run command with Caddy reverse-proxy arguments
	// Format: docker run -d --name <name> --network <network> -p <port>:80 \
	//         --label dewy.role=proxy --label dewy.managed=true \
	//         caddy:2-alpine caddy reverse-proxy --from :80 --to http://dewy-current:8080
	args := []string{"run", "-d"}
	args = append(args, "--name", opts.Name)
	args = append(args, "--network", opts.Network)
	args = append(args, "-p", fmt.Sprintf("%d:80", opts.ProxyPort))
	args = append(args, "--label", "dewy.role=proxy")
	args = append(args, "--label", "dewy.managed=true")
	args = append(args, opts.Image)
	// Caddy command arguments
	args = append(args, "caddy", "reverse-proxy")
	args = append(args, "--from", ":80")
	args = append(args, "--to", fmt.Sprintf("http://%s:%d", opts.UpstreamHost, opts.UpstreamPort))

	proxyID, err := d.execCommandOutput(ctx, args...)
	if err != nil {
		return "", fmt.Errorf("failed to start proxy container: %w", err)
	}

	d.logger.Info("Reverse proxy container started successfully",
		slog.String("container", proxyID),
		slog.String("name", opts.Name))

	return proxyID, nil
}

// FindProxyContainer finds an existing proxy container.
func (d *Docker) FindProxyContainer(ctx context.Context) (string, error) {
	containerID, err := d.FindContainerByLabel(ctx, map[string]string{
		"dewy.role": "proxy",
	})
	if err != nil {
		if errors.Is(err, ErrContainerNotFound) {
			return "", nil // No proxy container found (not an error)
		}
		return "", err
	}
	return containerID, nil
}

// CleanupManagedContainers stops and removes all containers with dewy.managed=true label.
func (d *Docker) CleanupManagedContainers(ctx context.Context) error {
	d.logger.Info("Cleaning up managed containers")

	// Find all managed containers
	args := []string{"ps", "-aq", "--filter", "label=dewy.managed=true"}
	output, err := d.execCommandOutput(ctx, args...)
	if err != nil {
		return fmt.Errorf("failed to list managed containers: %w", err)
	}

	if output == "" {
		d.logger.Info("No managed containers to clean up")
		return nil
	}

	// Split container IDs
	containerIDs := strings.Split(strings.TrimSpace(output), "\n")

	// Stop and remove each container
	for _, containerID := range containerIDs {
		containerID = strings.TrimSpace(containerID)
		if containerID == "" {
			continue
		}

		d.logger.Info("Stopping managed container", slog.String("container", containerID))
		if err := d.Stop(ctx, containerID, 10*time.Second); err != nil {
			d.logger.Warn("Failed to stop container",
				slog.String("container", containerID),
				slog.String("error", err.Error()))
		}

		d.logger.Info("Removing managed container", slog.String("container", containerID))
		if err := d.Remove(ctx, containerID); err != nil {
			d.logger.Warn("Failed to remove container",
				slog.String("container", containerID),
				slog.String("error", err.Error()))
		}
	}

	d.logger.Info("Cleanup completed", slog.Int("containers_cleaned", len(containerIDs)))
	return nil
}
