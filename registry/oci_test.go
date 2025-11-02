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
			oci := &OCI{
				PreRelease: tt.preRelease,
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

			if !strings.HasPrefix(res.ArtifactURL, "docker://") {
				t.Errorf("Expected ArtifactURL to start with 'docker://', got %s", res.ArtifactURL)
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
		name               string
		registry           string
		envUsername        string
		envPassword        string
		envGitHubToken     string
		expectedUsername   string
		expectedPassword   string
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
