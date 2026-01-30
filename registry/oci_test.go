package registry

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/linyows/dewy/logging"
)

// mockRegistryServer creates a mock Docker Registry HTTP API V2 server.
func mockRegistryServer(t *testing.T) *httptest.Server {
	t.Helper()

	mux := http.NewServeMux()

	// GET /v2/ - Registry API version check
	mux.HandleFunc("/v2/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	})

	// GET /v2/<name>/tags/list - List tags
	mux.HandleFunc("/v2/testapp/tags/list", func(w http.ResponseWriter, r *http.Request) {
		response := map[string]interface{}{
			"name": "testapp",
			"tags": []string{
				"v1.0.0",
				"v1.0.1",
				"v1.1.0",
				"v2.0.0",
				"v2.0.0-beta",
				"v2.1.0-rc1",
				"latest",
			},
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(response)
	})

	// GET /v2/<name>/manifests/<reference> - Get manifest
	mux.HandleFunc("/v2/testapp/manifests/", func(w http.ResponseWriter, r *http.Request) {
		// Extract tag from path
		path := strings.TrimPrefix(r.URL.Path, "/v2/testapp/manifests/")

		w.Header().Set("Content-Type", "application/vnd.docker.distribution.manifest.v2+json")
		w.Header().Set("Docker-Content-Digest", "sha256:abc123def456")

		manifest := map[string]interface{}{
			"schemaVersion": 2,
			"mediaType":     "application/vnd.docker.distribution.manifest.v2+json",
			"config": map[string]string{
				"digest": "sha256:config123" + path,
			},
		}
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(manifest)
	})

	return httptest.NewServer(mux)
}

func TestOCI_listTags(t *testing.T) {
	server := mockRegistryServer(t)
	defer server.Close()

	// Extract host from URL (http://127.0.0.1:12345 -> 127.0.0.1:12345)
	registryHost := strings.TrimPrefix(server.URL, "http://")

	logger := logging.SetupLogger("ERROR", "text", os.Stderr)
	oci := &OCI{
		Registry:   registryHost,
		Repository: "testapp",
		client:     &http.Client{Timeout: 5 * time.Second},
		logger:     logger,
	}

	ctx := context.Background()
	tags, err := oci.listTags(ctx)

	if err != nil {
		t.Fatalf("Failed to list tags: %v", err)
	}

	expectedTags := []string{"v1.0.0", "v1.0.1", "v1.1.0", "v2.0.0", "v2.0.0-beta", "v2.1.0-rc1", "latest"}
	if len(tags) != len(expectedTags) {
		t.Errorf("Expected %d tags, got %d", len(expectedTags), len(tags))
	}

	for i, tag := range tags {
		if tag != expectedTags[i] {
			t.Errorf("Expected tag %s at index %d, got %s", expectedTags[i], i, tag)
		}
	}
}

func TestOCI_findLatestTag(t *testing.T) {
	tests := []struct {
		name        string
		tags        []string
		preRelease  bool
		expectedTag string
		expectError bool
	}{
		{
			name:        "latest stable version",
			tags:        []string{"v1.0.0", "v1.0.1", "v2.0.0", "v2.0.0-beta"},
			preRelease:  false,
			expectedTag: "v2.0.0",
			expectError: false,
		},
		{
			name:        "with pre-release enabled",
			tags:        []string{"v1.0.0", "v2.0.0", "v2.1.0-rc1"},
			preRelease:  true,
			expectedTag: "v2.1.0-rc1",
			expectError: false,
		},
		{
			name:        "without pre-release",
			tags:        []string{"v1.0.0", "v2.0.0", "v2.1.0-rc1"},
			preRelease:  false,
			expectedTag: "v2.0.0",
			expectError: false,
		},
		{
			name:        "no valid semver tags",
			tags:        []string{"latest", "main", "dev"},
			preRelease:  false,
			expectedTag: "",
			expectError: true,
		},
		{
			name:        "mixed valid and invalid tags",
			tags:        []string{"latest", "v1.0.0", "main", "v1.0.1"},
			preRelease:  false,
			expectedTag: "v1.0.1",
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := logging.SetupLogger("INFO", "text", os.Stderr)
			oci := &OCI{
				PreRelease: tt.preRelease,
				logger:     logger,
			}

			latestTag, err := oci.findLatestTag(tt.tags)

			if tt.expectError {
				if err == nil {
					t.Fatal("Expected error, but got none")
				}
				return
			}

			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			if latestTag != tt.expectedTag {
				t.Errorf("Expected tag %s, got %s", tt.expectedTag, latestTag)
			}
		})
	}
}

func TestOCI_getImageDigest(t *testing.T) {
	server := mockRegistryServer(t)
	defer server.Close()

	registryHost := strings.TrimPrefix(server.URL, "http://")

	logger := logging.SetupLogger("ERROR", "text", os.Stderr)
	oci := &OCI{
		Registry:   registryHost,
		Repository: "testapp",
		client:     &http.Client{Timeout: 5 * time.Second},
		logger:     logger,
	}

	ctx := context.Background()
	digest, createdAt, err := oci.getImageDigest(ctx, "v1.0.0")

	if err != nil {
		t.Fatalf("Failed to get image digest: %v", err)
	}

	expectedDigest := "sha256:abc123def456"
	if digest != expectedDigest {
		t.Errorf("Expected digest %s, got %s", expectedDigest, digest)
	}

	if createdAt == nil {
		t.Error("Expected createdAt to be set, got nil")
	}
}

func TestOCI_Current(t *testing.T) {
	server := mockRegistryServer(t)
	defer server.Close()

	registryHost := strings.TrimPrefix(server.URL, "http://")

	logger := logging.SetupLogger("ERROR", "text", os.Stderr)

	tests := []struct {
		name        string
		preRelease  bool
		expectedTag string
	}{
		{
			name:        "stable release only",
			preRelease:  false,
			expectedTag: "v2.0.0",
		},
		{
			name:        "with pre-release",
			preRelease:  true,
			expectedTag: "v2.1.0-rc1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			oci := &OCI{
				Registry:   registryHost,
				Repository: "testapp",
				PreRelease: tt.preRelease,
				client:     &http.Client{Timeout: 5 * time.Second},
				logger:     logger,
			}

			ctx := context.Background()
			res, err := oci.Current(ctx)

			if err != nil {
				t.Fatalf("Failed to get current: %v", err)
			}

			if res.Tag != tt.expectedTag {
				t.Errorf("Expected tag %s, got %s", tt.expectedTag, res.Tag)
			}

			if !strings.HasPrefix(res.ArtifactURL, "img://") {
				t.Errorf("Expected ArtifactURL to start with 'img://', got %s", res.ArtifactURL)
			}

			expectedImageRef := registryHost + "/testapp:" + tt.expectedTag
			if !strings.Contains(res.ArtifactURL, expectedImageRef) {
				t.Errorf("Expected ArtifactURL to contain %s, got %s", expectedImageRef, res.ArtifactURL)
			}

			if res.ID == "" {
				t.Error("Expected ID (digest) to be set, got empty string")
			}
		})
	}
}

func TestOCI_loadCredentials(t *testing.T) {
	tests := []struct {
		name             string
		registry         string
		envUsername      string
		envPassword      string
		envGitHubToken   string
		expectedUsername string
		expectedPassword string
	}{
		{
			name:             "generic docker credentials",
			registry:         "registry.example.com",
			envUsername:      "testuser",
			envPassword:      "testpass",
			expectedUsername: "testuser",
			expectedPassword: "testpass",
		},
		{
			name:             "GitHub Container Registry with token",
			registry:         "ghcr.io",
			envGitHubToken:   "ghp_testtoken123",
			expectedUsername: "token",
			expectedPassword: "ghp_testtoken123",
		},
		{
			name:             "no credentials",
			registry:         "registry.example.com",
			expectedUsername: "",
			expectedPassword: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set environment variables
			if tt.envUsername != "" {
				os.Setenv("DOCKER_USERNAME", tt.envUsername)
				defer os.Unsetenv("DOCKER_USERNAME")
			}
			if tt.envPassword != "" {
				os.Setenv("DOCKER_PASSWORD", tt.envPassword)
				defer os.Unsetenv("DOCKER_PASSWORD")
			}
			if tt.envGitHubToken != "" {
				os.Setenv("GITHUB_TOKEN", tt.envGitHubToken)
				defer os.Unsetenv("GITHUB_TOKEN")
			}

			oci := &OCI{
				Registry: tt.registry,
			}

			oci.loadCredentials()

			if oci.username != tt.expectedUsername {
				t.Errorf("Expected username %s, got %s", tt.expectedUsername, oci.username)
			}

			if oci.password != tt.expectedPassword {
				t.Errorf("Expected password %s, got %s", tt.expectedPassword, oci.password)
			}
		})
	}
}

// mockRegistryServerWithAuth creates a mock Docker Registry HTTP API V2 server
// that requires authentication and simulates token expiry scenarios.
func mockRegistryServerWithAuth(t *testing.T, validToken string) *httptest.Server {
	t.Helper()

	mux := http.NewServeMux()

	// Token endpoint
	mux.HandleFunc("/token", func(w http.ResponseWriter, r *http.Request) {
		response := map[string]interface{}{
			"token":      validToken,
			"expires_in": 300,
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(response)
	})

	// GET /v2/ - Registry API version check
	mux.HandleFunc("/v2/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	})

	// GET /v2/<name>/tags/list - List tags (requires auth)
	mux.HandleFunc("/v2/testapp/tags/list", func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		expectedAuth := "Bearer " + validToken

		if authHeader != expectedAuth {
			w.Header().Set("WWW-Authenticate", `Bearer realm="http://`+r.Host+`/token",service="registry",scope="repository:testapp:pull"`)
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"errors":[{"code":"UNAUTHORIZED","message":"authentication required"}]}`))
			return
		}

		response := map[string]interface{}{
			"name": "testapp",
			"tags": []string{"v1.0.0", "v1.0.1", "v2.0.0"},
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(response)
	})

	// GET /v2/<name>/manifests/<reference> - Get manifest (requires auth)
	mux.HandleFunc("/v2/testapp/manifests/", func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		expectedAuth := "Bearer " + validToken

		if authHeader != expectedAuth {
			w.Header().Set("WWW-Authenticate", `Bearer realm="http://`+r.Host+`/token",service="registry",scope="repository:testapp:pull"`)
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"errors":[{"code":"UNAUTHORIZED","message":"authentication required"}]}`))
			return
		}

		w.Header().Set("Content-Type", "application/vnd.docker.distribution.manifest.v2+json")
		w.Header().Set("Docker-Content-Digest", "sha256:abc123def456")

		manifest := map[string]interface{}{
			"schemaVersion": 2,
			"mediaType":     "application/vnd.docker.distribution.manifest.v2+json",
			"config": map[string]string{
				"digest": "sha256:config123",
			},
		}
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(manifest)
	})

	return httptest.NewServer(mux)
}

func TestOCI_getImageDigest_WithExpiredToken(t *testing.T) {
	validToken := "new_valid_token_123"
	server := mockRegistryServerWithAuth(t, validToken)
	defer server.Close()

	registryHost := strings.TrimPrefix(server.URL, "http://")

	logger := logging.SetupLogger("ERROR", "text", os.Stderr)
	oci := &OCI{
		Registry:   registryHost,
		Repository: "testapp",
		client:     &http.Client{Timeout: 5 * time.Second},
		logger:     logger,
		username:   "testuser",
		password:   "testpass",
		token:      "expired_old_token", // Simulate expired token
	}

	ctx := context.Background()
	digest, _, err := oci.getImageDigest(ctx, "v1.0.0")

	if err != nil {
		t.Fatalf("Expected success after token refresh, got error: %v", err)
	}

	expectedDigest := "sha256:abc123def456"
	if digest != expectedDigest {
		t.Errorf("Expected digest %s, got %s", expectedDigest, digest)
	}

	// Verify that token was refreshed
	if oci.token != validToken {
		t.Errorf("Expected token to be refreshed to %s, got %s", validToken, oci.token)
	}
}

func TestOCI_listTags_WithExpiredToken(t *testing.T) {
	validToken := "new_valid_token_456"
	server := mockRegistryServerWithAuth(t, validToken)
	defer server.Close()

	registryHost := strings.TrimPrefix(server.URL, "http://")

	logger := logging.SetupLogger("ERROR", "text", os.Stderr)
	oci := &OCI{
		Registry:   registryHost,
		Repository: "testapp",
		client:     &http.Client{Timeout: 5 * time.Second},
		logger:     logger,
		username:   "testuser",
		password:   "testpass",
		token:      "expired_old_token", // Simulate expired token
	}

	ctx := context.Background()
	tags, err := oci.listTags(ctx)

	if err != nil {
		t.Fatalf("Expected success after token refresh, got error: %v", err)
	}

	if len(tags) != 3 {
		t.Errorf("Expected 3 tags, got %d", len(tags))
	}

	// Verify that token was refreshed
	if oci.token != validToken {
		t.Errorf("Expected token to be refreshed to %s, got %s", validToken, oci.token)
	}
}

func TestOCI_Report(t *testing.T) {
	logger := logging.SetupLogger("ERROR", "text", os.Stderr)
	oci := &OCI{
		Registry:   "registry.example.com",
		Repository: "testapp",
		logger:     logger,
	}

	ctx := context.Background()
	req := &ReportRequest{
		ID:      "sha256:abc123",
		Tag:     "v1.0.0",
		Command: "container",
		Err:     nil,
	}

	// Report should not error (it's a no-op in Phase 1)
	err := oci.Report(ctx, req)
	if err != nil {
		t.Errorf("Expected Report to succeed, got error: %v", err)
	}
}

func TestOCI_parseNextLink(t *testing.T) {
	oci := &OCI{
		Registry: "ghcr.io",
	}

	tests := []struct {
		name       string
		linkHeader string
		expected   string
	}{
		{
			name:       "valid next link with relative URL",
			linkHeader: `</v2/testapp/tags/list?n=100&last=v1.0.0>; rel="next"`,
			expected:   "https://ghcr.io/v2/testapp/tags/list?n=100&last=v1.0.0",
		},
		{
			name:       "valid next link with absolute URL",
			linkHeader: `<https://ghcr.io/v2/testapp/tags/list?n=100&last=v2.0.0>; rel="next"`,
			expected:   "https://ghcr.io/v2/testapp/tags/list?n=100&last=v2.0.0",
		},
		{
			name:       "empty link header",
			linkHeader: "",
			expected:   "",
		},
		{
			name:       "link without rel=next",
			linkHeader: `</v2/testapp/tags/list?n=100&last=v1.0.0>; rel="prev"`,
			expected:   "",
		},
		{
			name:       "malformed link header - no angle brackets",
			linkHeader: `/v2/testapp/tags/list?n=100&last=v1.0.0; rel="next"`,
			expected:   "",
		},
		{
			name:       "malformed link header - no semicolon",
			linkHeader: `</v2/testapp/tags/list?n=100&last=v1.0.0>`,
			expected:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := oci.parseNextLink(tt.linkHeader)
			if result != tt.expected {
				t.Errorf("parseNextLink(%q) = %q, want %q", tt.linkHeader, result, tt.expected)
			}
		})
	}
}

func mockRegistryServerWithPagination(t *testing.T) *httptest.Server {
	t.Helper()

	mux := http.NewServeMux()

	// GET /v2/ - Registry API version check
	mux.HandleFunc("/v2/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	})

	// GET /v2/<name>/tags/list - List tags with pagination
	mux.HandleFunc("/v2/testapp/tags/list", func(w http.ResponseWriter, r *http.Request) {
		last := r.URL.Query().Get("last")

		var response map[string]interface{}

		switch last {
		case "":
			// First page
			response = map[string]interface{}{
				"name": "testapp",
				"tags": []string{"v1.0.0", "v1.0.1", "v1.1.0"},
			}
			w.Header().Set("Link", `</v2/testapp/tags/list?n=3&last=v1.1.0>; rel="next"`)
		case "v1.1.0":
			// Second page
			response = map[string]interface{}{
				"name": "testapp",
				"tags": []string{"v2.0.0", "v2.0.1", "v2.1.0"},
			}
			w.Header().Set("Link", `</v2/testapp/tags/list?n=3&last=v2.1.0>; rel="next"`)
		case "v2.1.0":
			// Third page (last page, no Link header)
			response = map[string]interface{}{
				"name": "testapp",
				"tags": []string{"v3.0.0", "latest"},
			}
			// No Link header for last page
		default:
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(response)
	})

	return httptest.NewServer(mux)
}

func TestOCI_listTags_WithPagination(t *testing.T) {
	server := mockRegistryServerWithPagination(t)
	defer server.Close()

	registryHost := strings.TrimPrefix(server.URL, "http://")

	logger := logging.SetupLogger("ERROR", "text", os.Stderr)
	oci := &OCI{
		Registry:   registryHost,
		Repository: "testapp",
		client:     &http.Client{Timeout: 5 * time.Second},
		logger:     logger,
	}

	ctx := context.Background()
	tags, err := oci.listTags(ctx)

	if err != nil {
		t.Fatalf("Failed to list tags: %v", err)
	}

	expectedTags := []string{"v1.0.0", "v1.0.1", "v1.1.0", "v2.0.0", "v2.0.1", "v2.1.0", "v3.0.0", "latest"}
	if len(tags) != len(expectedTags) {
		t.Errorf("Expected %d tags, got %d: %v", len(expectedTags), len(tags), tags)
	}

	for i, tag := range tags {
		if i < len(expectedTags) && tag != expectedTags[i] {
			t.Errorf("Expected tag %s at index %d, got %s", expectedTags[i], i, tag)
		}
	}
}

