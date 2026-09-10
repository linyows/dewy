package container

import (
	"context"
	"fmt"
	"log/slog"
	"os/exec"
	"strings"

	"github.com/linyows/dewy/internal/ociauth"
)

// extractRegistry extracts the registry host from an image reference.
// For images without explicit registry (e.g., "nginx:latest"), returns "docker.io".
// For images with registry (e.g., "ghcr.io/owner/repo:tag"), returns the registry host.
func extractRegistry(imageRef string) string {
	// Strip any digest first (after `@`).
	ref := imageRef
	if idx := strings.Index(ref, "@"); idx != -1 {
		ref = ref[:idx]
	}

	// Strip the tag, but only when `:` is unambiguously a tag separator —
	// i.e. the colon appears after the last `/`. A colon before the last
	// `/` is part of a `host:port` registry (e.g. `localhost:5000/img`)
	// and must be preserved.
	lastSlash := strings.LastIndex(ref, "/")
	lastColon := strings.LastIndex(ref, ":")
	if lastColon > lastSlash {
		ref = ref[:lastColon]
	}

	// First component is the registry only if it contains a dot, a colon
	// (host:port), or is the literal "localhost". Otherwise it is the user
	// part of an implicit Docker Hub reference (e.g. "library/nginx").
	parts := strings.SplitN(ref, "/", 2)
	if len(parts) == 1 {
		return "docker.io"
	}
	firstPart := parts[0]
	if strings.Contains(firstPart, ".") || strings.Contains(firstPart, ":") || firstPart == "localhost" {
		return firstPart
	}
	return "docker.io"
}

// isAuthError checks if the error message indicates an authentication failure.
func isAuthError(output string) bool {
	lowerOutput := strings.ToLower(output)
	authIndicators := []string{
		"unauthorized",
		"authentication required",
		"denied",
		"access forbidden",
		"not authorized",
		"login required",
	}
	for _, indicator := range authIndicators {
		if strings.Contains(lowerOutput, indicator) {
			return true
		}
	}
	return false
}

// Login authenticates with the specified registry using credentials from environment variables.
func (r *Runtime) Login(ctx context.Context, registry string) error {
	username, password := ociauth.Credentials(registry)
	if username == "" || password == "" {
		r.logger.Debug("No credentials found for registry", slog.String("registry", registry))
		return nil
	}

	r.logger.Info("Logging in to registry", slog.String("registry", registry))

	// Use --password-stdin for security
	// #nosec G204 - args are constructed internally from validated inputs
	cmd := exec.CommandContext(ctx, r.cmd, "login", "-u", username, "--password-stdin", registry)
	cmd.Stdin = strings.NewReader(password)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s login failed for %s: %w: %s", r.cmd, registry, err, string(output))
	}

	r.loggedInRegistries[registry] = true
	r.logger.Info("Successfully logged in to registry", slog.String("registry", registry))

	return nil
}
