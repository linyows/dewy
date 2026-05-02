package container

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os/exec"
	"sort"
	"strconv"
	"strings"
	"time"
)

// imageInspect represents the structure of image inspect JSON output.
type imageInspect struct {
	ID     string `json:"Id"`
	Config struct {
		ExposedPorts map[string]struct{} `json:"ExposedPorts"`
	} `json:"Config"`
}

// Pull pulls an image from the registry.
// If the image already exists locally, it will still attempt to pull to get the latest version.
// Automatically handles authentication: logs in on first pull and retries on auth errors.
func (r *Runtime) Pull(ctx context.Context, imageRef string) error {
	// Check if image exists locally first (using direct exec to avoid ERROR log for expected miss)
	// #nosec G204 - imageRef is validated by caller
	inspectCmd := exec.CommandContext(ctx, r.cmd, "image", "inspect", imageRef)
	r.logger.Debug("Executing command",
		slog.String("cmd", r.cmd),
		slog.Any("args", []string{"image", "inspect", imageRef}))
	localErr := inspectCmd.Run()
	if localErr == nil {
		r.logger.Info("Image already exists locally, pulling to check for updates",
			slog.String("image", imageRef))
	}

	// Extract registry and attempt login if not already logged in
	registry := extractRegistry(imageRef)
	if !r.loggedInRegistries[registry] {
		if err := r.Login(ctx, registry); err != nil {
			r.logger.Warn("Initial login failed, will attempt pull anyway",
				slog.String("registry", registry),
				slog.String("error", err.Error()))
		}
	}

	// Always try to pull to get the latest version
	r.logger.Info("Pulling image", slog.String("image", imageRef))
	output, pullErr := r.pullImage(ctx, imageRef)

	// If pull fails with auth error, retry login and pull again
	if pullErr != nil && isAuthError(output) {
		r.logger.Info("Authentication error detected, attempting re-login",
			slog.String("registry", registry))

		delete(r.loggedInRegistries, registry)
		if loginErr := r.Login(ctx, registry); loginErr != nil {
			r.logger.Error("Re-login failed",
				slog.String("registry", registry),
				slog.String("error", loginErr.Error()))
		} else {
			r.logger.Info("Retrying pull after re-login", slog.String("image", imageRef))
			_, pullErr = r.pullImage(ctx, imageRef)
		}
	}

	// If pull fails but image exists locally, we can use the local image
	if pullErr != nil && localErr == nil {
		r.logger.Warn("Failed to pull image, but local image exists - using local version",
			slog.String("image", imageRef),
			slog.String("pull_error", pullErr.Error()))
		return nil
	}

	return pullErr
}

// pullImage executes pull and returns output and error.
func (r *Runtime) pullImage(ctx context.Context, imageRef string) (string, error) {
	// #nosec G204 - args are constructed internally from validated inputs
	cmd := exec.CommandContext(ctx, r.cmd, "pull", imageRef)
	r.logger.Debug("Executing command",
		slog.String("cmd", r.cmd),
		slog.Any("args", []string{"pull", imageRef}))

	output, err := cmd.CombinedOutput()
	if err != nil {
		r.logger.Error("Pull failed",
			slog.String("image", imageRef),
			slog.String("output", string(output)),
			slog.String("error", err.Error()))
		return string(output), fmt.Errorf("%s pull %s failed: %w: %s", r.cmd, imageRef, err, string(output))
	}

	return string(output), nil
}

// imageRepositoryFromRef derives the repository part of an image reference.
// It removes any digest component (after '@') and then strips a tag (after ':')
// only if the colon appears after the last slash, to avoid confusing registry ports
// with tag separators.
func imageRepositoryFromRef(imageRef string) string {
	ref := imageRef

	// Strip digest if present (e.g., "repo@sha256:abcdef...")
	if at := strings.Index(ref, "@"); at != -1 {
		ref = ref[:at]
	}

	lastSlash := strings.LastIndex(ref, "/")
	lastColon := strings.LastIndex(ref, ":")

	if lastSlash == -1 {
		// No slash: any colon is a tag separator
		if lastColon != -1 {
			return ref[:lastColon]
		}
		return ref
	}

	// Only treat colon as tag separator if it appears after the last slash
	if lastColon > lastSlash {
		return ref[:lastColon]
	}

	return ref
}

// CleanupOldImages removes old container images, keeping only the most recent ones.
func (r *Runtime) CleanupOldImages(ctx context.Context, imageRef string, keepCount int) error {
	repository := imageRepositoryFromRef(imageRef)

	images, err := r.ListImages(ctx, repository)
	if err != nil {
		return fmt.Errorf("failed to list images: %w", err)
	}

	if len(images) <= keepCount {
		r.logger.Debug("No old images to clean up",
			slog.String("repository", repository),
			slog.Int("count", len(images)),
			slog.Int("keep", keepCount))
		return nil
	}

	// Sort images by creation time (newest first)
	sort.Slice(images, func(i, j int) bool {
		return images[i].Created.After(images[j].Created)
	})

	// Remove old images (keep only the most recent keepCount)
	for i, img := range images {
		if i < keepCount {
			r.logger.Debug("Keeping image",
				slog.String("id", img.ID),
				slog.String("tag", img.Tag),
				slog.Time("created", img.Created))
			continue
		}

		r.logger.Info("Removing old image",
			slog.String("id", img.ID),
			slog.String("tag", img.Tag),
			slog.Time("created", img.Created))

		if err := r.RemoveImage(ctx, img.ID); err != nil {
			r.logger.Warn("Failed to remove image",
				slog.String("id", img.ID),
				slog.String("error", err.Error()))
			continue
		}
	}

	return nil
}

// ListImages returns a list of images matching the given repository.
func (r *Runtime) ListImages(ctx context.Context, repository string) ([]ImageInfo, error) {
	format := "{{.ID}}|{{.Repository}}|{{.Tag}}|{{.CreatedAt}}|{{.Size}}"
	output, err := r.execCommandOutput(ctx, "images", "--format", format, repository)
	if err != nil {
		return nil, fmt.Errorf("failed to list images: %w", err)
	}

	if output == "" {
		return []ImageInfo{}, nil
	}

	lines := strings.Split(output, "\n")
	images := make([]ImageInfo, 0, len(lines))

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		parts := strings.Split(line, "|")
		if len(parts) != 5 {
			r.logger.Warn("Unexpected image format", slog.String("line", line))
			continue
		}

		var created time.Time
		createdStr := strings.TrimSpace(parts[3])

		timeFormats := []string{
			"2006-01-02 15:04:05 -0700 MST",
			"2006-01-02 15:04:05 -0700",
			time.RFC3339,
		}

		for _, format := range timeFormats {
			if t, err := time.Parse(format, createdStr); err == nil {
				created = t
				break
			}
		}

		images = append(images, ImageInfo{
			ID:         strings.TrimSpace(parts[0]),
			Repository: strings.TrimSpace(parts[1]),
			Tag:        strings.TrimSpace(parts[2]),
			Created:    created,
			Size:       0,
		})
	}

	r.logger.Debug("Listed images",
		slog.String("repository", repository),
		slog.Int("count", len(images)))

	return images, nil
}

// RemoveImage removes an image by ID.
func (r *Runtime) RemoveImage(ctx context.Context, imageID string) error {
	r.logger.Info("Removing image", slog.String("image", imageID))

	err := r.execCommand(ctx, "rmi", "--force", imageID)
	if err != nil {
		return fmt.Errorf("failed to remove image: %w", err)
	}

	return nil
}

// GetImageExposedPorts returns the list of exposed ports from an image.
// Returns port numbers (e.g., [80, 443]) sorted in ascending order.
func (r *Runtime) GetImageExposedPorts(ctx context.Context, imageRef string) ([]int, error) {
	output, err := r.execCommandOutput(ctx, "image", "inspect", imageRef)
	if err != nil {
		return nil, fmt.Errorf("failed to inspect image: %w", err)
	}

	var inspects []imageInspect
	if err := json.Unmarshal([]byte(output), &inspects); err != nil {
		return nil, fmt.Errorf("failed to parse image inspect output: %w", err)
	}

	if len(inspects) == 0 {
		return nil, fmt.Errorf("image not found: %s", imageRef)
	}

	inspect := inspects[0]
	if len(inspect.Config.ExposedPorts) == 0 {
		return []int{}, nil
	}

	ports := make([]int, 0, len(inspect.Config.ExposedPorts))
	for portSpec := range inspect.Config.ExposedPorts {
		if !strings.HasSuffix(portSpec, "/tcp") {
			continue
		}

		portStr := strings.TrimSuffix(portSpec, "/tcp")
		port, err := strconv.Atoi(portStr)
		if err != nil {
			r.logger.Warn("Failed to parse exposed port",
				slog.String("port_spec", portSpec),
				slog.String("error", err.Error()))
			continue
		}

		ports = append(ports, port)
	}

	sort.Ints(ports)

	r.logger.Debug("Detected exposed ports from image",
		slog.String("image", imageRef),
		slog.Any("ports", ports))

	return ports, nil
}
