package container

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"strings"
)

// extractRegistry extracts the registry host from an image reference.
// For images without explicit registry (e.g., "nginx:latest"), returns "docker.io".
// For images with registry (e.g., "ghcr.io/owner/repo:tag"), returns the registry host.
func extractRegistry(imageRef string) string {
	// Remove tag or digest
	ref := imageRef
	if idx := strings.LastIndex(ref, "@"); idx != -1 {
		ref = ref[:idx]
	}
	if idx := strings.LastIndex(ref, ":"); idx != -1 {
		// Check if this is a port number (e.g., localhost:5000/image)
		slashIdx := strings.LastIndex(ref[:idx], "/")
		if slashIdx == -1 || !strings.Contains(ref[slashIdx:idx], ".") {
			ref = ref[:idx]
		}
	}

	// Check if the first part contains a dot or colon (indicating a registry)
	parts := strings.SplitN(ref, "/", 2)
	if len(parts) == 1 {
		// No slash, it's a Docker Hub official image (e.g., "nginx")
		return "docker.io"
	}

	firstPart := parts[0]
	if strings.Contains(firstPart, ".") || strings.Contains(firstPart, ":") || firstPart == "localhost" {
		return firstPart
	}

	// No registry specified (e.g., "library/nginx"), use Docker Hub
	return "docker.io"
}

// getCredentials returns username and password for the given registry from environment variables.
func getCredentials(registry string) (username, password string) {
	// GitHub Container Registry
	if strings.Contains(registry, "ghcr.io") {
		if token := os.Getenv("GITHUB_TOKEN"); token != "" {
			return "token", token
		}
	}

	// AWS ECR - check for ECR-specific credentials first
	if strings.Contains(registry, ".ecr.") && strings.Contains(registry, ".amazonaws.com") {
		if token := os.Getenv("AWS_ECR_PASSWORD"); token != "" {
			return "AWS", token
		}
	}

	// Google Artifact Registry / Container Registry
	if strings.Contains(registry, "gcr.io") || strings.Contains(registry, "-docker.pkg.dev") {
		if token := os.Getenv("GCR_TOKEN"); token != "" {
			return "_json_key", token
		}
	}

	// Generic credentials (fallback)
	username = os.Getenv("DOCKER_USERNAME")
	password = os.Getenv("DOCKER_PASSWORD")

	return username, password
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
	username, password := getCredentials(registry)
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
