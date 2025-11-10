package registry

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/linyows/dewy/logging"
)

// OCI implements Registry interface for OCI/Docker registries.
type OCI struct {
	Registry     string `schema:"-"`
	Repository   string `schema:"-"`
	Tag          string `schema:"-"` // Optional: specific tag to track
	PreRelease   bool   `schema:"pre-release"`
	Constraint   string `schema:"constraint"` // Semver constraint (e.g., "~1.0", "^2.0")
	username     string
	password     string
	token        string // Bearer token for authentication
	client       *http.Client
	logger       *logging.Logger
}

// NewOCI creates a new OCI registry.
func NewOCI(ctx context.Context, u string, log *logging.Logger) (*OCI, error) {
	ur, err := url.Parse(u)
	if err != nil {
		return nil, err
	}

	oci := &OCI{
		Registry:   ur.Host,
		Repository: strings.TrimPrefix(ur.Path, "/"),
		client:     &http.Client{Timeout: 30 * time.Second},
		logger:     log,
	}

	// Parse query parameters
	if err := decoder.Decode(oci, ur.Query()); err != nil {
		return nil, err
	}

	// Get credentials from environment
	oci.loadCredentials()

	return oci, nil
}

// loadCredentials loads credentials from environment variables.
func (o *OCI) loadCredentials() {
	// Generic credentials
	if username := os.Getenv("DOCKER_USERNAME"); username != "" {
		o.username = username
		o.password = os.Getenv("DOCKER_PASSWORD")
		return
	}

	// GitHub Container Registry
	if strings.Contains(o.Registry, "ghcr.io") {
		if token := os.Getenv("GITHUB_TOKEN"); token != "" {
			o.username = "token"
			o.password = token
			return
		}
	}

	// AWS ECR - will use aws-cli credentials
	// Google Artifact Registry - will use gcloud credentials
	// TODO: Phase 2 implementation
}

// Current returns the current artifact from the OCI registry.
func (o *OCI) Current(ctx context.Context) (*CurrentResponse, error) {
	// Get list of tags
	tags, err := o.listTags(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list tags: %w", err)
	}

	if len(tags) == 0 {
		return nil, fmt.Errorf("no tags found in registry %s/%s", o.Registry, o.Repository)
	}

	// Filter and sort by semantic versioning
	latestTag, err := o.findLatestTag(tags)
	if err != nil {
		return nil, fmt.Errorf("failed to find latest tag: %w", err)
	}

	// Get image digest
	digest, createdAt, err := o.getImageDigest(ctx, latestTag)
	if err != nil {
		return nil, fmt.Errorf("failed to get image digest: %w", err)
	}

	imageRef := fmt.Sprintf("%s/%s:%s", o.Registry, o.Repository, latestTag)

	return &CurrentResponse{
		ID:          digest,
		Tag:         latestTag,
		ArtifactURL: fmt.Sprintf("docker://%s", imageRef),
		CreatedAt:   createdAt,
	}, nil
}

// Report reports the deployment result (no-op for OCI registries in Phase 1).
func (o *OCI) Report(ctx context.Context, req *ReportRequest) error {
	// OCI registries don't support reporting in Phase 1
	// This could be extended in Phase 2 to update labels or annotations
	return nil
}

// getScheme returns the appropriate URL scheme for the registry.
// For local development (localhost, 127.0.0.1), use http.
// For all other registries, use https.
func (o *OCI) getScheme() string {
	if strings.HasPrefix(o.Registry, "localhost") || strings.HasPrefix(o.Registry, "127.0.0.1") {
		return "http"
	}
	return "https"
}

// getBearerToken retrieves a bearer token from the registry's auth endpoint.
func (o *OCI) getBearerToken(ctx context.Context, authHeader string) error {
	// Parse WWW-Authenticate header
	// Format: Bearer realm="https://...",service="...",scope="..."
	if !strings.HasPrefix(authHeader, "Bearer ") {
		return fmt.Errorf("unsupported auth scheme: %s", authHeader)
	}

	params := make(map[string]string)
	parts := strings.Split(strings.TrimPrefix(authHeader, "Bearer "), ",")
	for _, part := range parts {
		kv := strings.SplitN(strings.TrimSpace(part), "=", 2)
		if len(kv) == 2 {
			key := kv[0]
			value := strings.Trim(kv[1], "\"")
			params[key] = value
		}
	}

	realm, ok := params["realm"]
	if !ok {
		return fmt.Errorf("no realm in auth header")
	}

	// Build token request URL
	tokenURL, err := url.Parse(realm)
	if err != nil {
		return fmt.Errorf("invalid realm URL: %w", err)
	}

	q := tokenURL.Query()
	if service, ok := params["service"]; ok {
		q.Set("service", service)
	}
	if scope, ok := params["scope"]; ok {
		q.Set("scope", scope)
	}
	tokenURL.RawQuery = q.Encode()

	// Request token
	req, err := http.NewRequestWithContext(ctx, "GET", tokenURL.String(), nil)
	if err != nil {
		return err
	}

	// Add basic auth if credentials are available
	if o.username != "" && o.password != "" {
		req.SetBasicAuth(o.username, o.password)
	}

	resp, err := o.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to get token: status %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		Token       string `json:"token"`
		AccessToken string `json:"access_token"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return err
	}

	// Use either token or access_token field
	if result.Token != "" {
		o.token = result.Token
	} else if result.AccessToken != "" {
		o.token = result.AccessToken
	} else {
		return fmt.Errorf("no token in response")
	}

	return nil
}

// listTags retrieves the list of tags from the registry.
func (o *OCI) listTags(ctx context.Context) ([]string, error) {
	// Docker Registry HTTP API V2: GET /v2/<name>/tags/list
	scheme := o.getScheme()
	apiURL := fmt.Sprintf("%s://%s/v2/%s/tags/list", scheme, o.Registry, o.Repository)

	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return nil, err
	}

	// Add authentication if available
	if o.token != "" {
		req.Header.Set("Authorization", "Bearer "+o.token)
	} else if o.username != "" && o.password != "" {
		auth := base64.StdEncoding.EncodeToString([]byte(o.username + ":" + o.password))
		req.Header.Set("Authorization", "Basic "+auth)
	}

	resp, err := o.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// Handle 401 Unauthorized - need to get bearer token
	if resp.StatusCode == http.StatusUnauthorized {
		authHeader := resp.Header.Get("WWW-Authenticate")
		if authHeader != "" {
			if err := o.getBearerToken(ctx, authHeader); err != nil {
				return nil, fmt.Errorf("failed to get bearer token: %w", err)
			}

			// Retry with bearer token
			req, err = http.NewRequestWithContext(ctx, "GET", apiURL, nil)
			if err != nil {
				return nil, err
			}
			req.Header.Set("Authorization", "Bearer "+o.token)

			resp, err = o.client.Do(req)
			if err != nil {
				return nil, err
			}
			defer resp.Body.Close()
		}
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to list tags: status %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		Name string   `json:"name"`
		Tags []string `json:"tags"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	o.logger.Debug("Retrieved tags from registry", "registry", o.Registry, "repository", o.Repository, "tags", result.Tags, "count", len(result.Tags))

	return result.Tags, nil
}

// findLatestTag finds the latest tag based on semantic versioning.
func (o *OCI) findLatestTag(tags []string) (string, error) {
	// TODO: Phase 2 - implement constraint support
	// For Phase 1, use the existing FindLatestSemVer function
	_, latestTag, err := FindLatestSemVer(tags, o.PreRelease)
	if err != nil {
		o.logger.Warn("Failed to find semantic versioned tag", "error", err, "tags", tags, "pre-release", o.PreRelease)
		return "", fmt.Errorf("%w (available tags: %v)", err, tags)
	}

	o.logger.Debug("Found latest tag", "tag", latestTag, "pre-release", o.PreRelease)
	return latestTag, nil
}

// getImageDigest retrieves the digest of an image.
func (o *OCI) getImageDigest(ctx context.Context, tag string) (string, *time.Time, error) {
	// Docker Registry HTTP API V2: GET /v2/<name>/manifests/<reference>
	scheme := o.getScheme()
	apiURL := fmt.Sprintf("%s://%s/v2/%s/manifests/%s", scheme, o.Registry, o.Repository, tag)

	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return "", nil, err
	}

	// Add authentication if available
	if o.token != "" {
		req.Header.Set("Authorization", "Bearer "+o.token)
	} else if o.username != "" && o.password != "" {
		auth := base64.StdEncoding.EncodeToString([]byte(o.username + ":" + o.password))
		req.Header.Set("Authorization", "Basic "+auth)
	}

	// Request Docker manifest schema v2 and OCI manifest/index
	// Support both single-platform and multi-platform images
	req.Header.Set("Accept", strings.Join([]string{
		"application/vnd.oci.image.index.v1+json",                  // OCI Index (multi-platform)
		"application/vnd.oci.image.manifest.v1+json",               // OCI Manifest
		"application/vnd.docker.distribution.manifest.list.v2+json", // Docker Manifest List (multi-platform)
		"application/vnd.docker.distribution.manifest.v2+json",      // Docker Manifest v2
	}, ", "))

	resp, err := o.client.Do(req)
	if err != nil {
		return "", nil, err
	}
	defer resp.Body.Close()

	// Handle 401 Unauthorized - need to get bearer token
	if resp.StatusCode == http.StatusUnauthorized {
		authHeader := resp.Header.Get("WWW-Authenticate")
		if authHeader != "" && o.token == "" {
			if err := o.getBearerToken(ctx, authHeader); err != nil {
				return "", nil, fmt.Errorf("failed to get bearer token: %w", err)
			}

			// Retry with bearer token
			req, err = http.NewRequestWithContext(ctx, "GET", apiURL, nil)
			if err != nil {
				return "", nil, err
			}
			req.Header.Set("Authorization", "Bearer "+o.token)
			req.Header.Set("Accept", strings.Join([]string{
				"application/vnd.oci.image.index.v1+json",
				"application/vnd.oci.image.manifest.v1+json",
				"application/vnd.docker.distribution.manifest.list.v2+json",
				"application/vnd.docker.distribution.manifest.v2+json",
			}, ", "))

			resp, err = o.client.Do(req)
			if err != nil {
				return "", nil, err
			}
			defer resp.Body.Close()
		}
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", nil, fmt.Errorf("failed to get manifest: status %d: %s", resp.StatusCode, string(body))
	}

	// Get digest from Docker-Content-Digest header
	digest := resp.Header.Get("Docker-Content-Digest")
	if digest == "" {
		return "", nil, fmt.Errorf("no digest found in response")
	}

	// Log the manifest type for debugging
	contentType := resp.Header.Get("Content-Type")
	o.logger.Debug("Retrieved manifest", "tag", tag, "digest", digest, "content-type", contentType)

	// Parse created time from manifest (optional)
	var manifest struct {
		Config struct {
			Digest string `json:"digest"`
		} `json:"config"`
	}

	body, err := io.ReadAll(resp.Body)
	if err == nil {
		// Ignore unmarshal error as manifest parsing is optional
		_ = json.Unmarshal(body, &manifest)
	}

	// For now, use current time as created time
	// TODO: Phase 2 - fetch actual created time from image config
	now := time.Now()

	return digest, &now, nil
}
