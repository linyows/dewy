package artifact

import (
	"context"
	"log/slog"
	"os"
	"testing"
)

func TestNewOCI(t *testing.T) {
	tests := []struct {
		name             string
		url              string
		expectedImageRef string
		expectError      bool
	}{
		{
			name:             "container scheme with registry and tag",
			url:              "container://ghcr.io/linyows/myapp:v1.0.0",
			expectedImageRef: "ghcr.io/linyows/myapp:v1.0.0",
			expectError:      false,
		},
		{
			name:             "with port number",
			url:              "container://localhost:5000/testapp:v1",
			expectedImageRef: "localhost:5000/testapp:v1",
			expectError:      false,
		},
		{
			name:             "nested repository path",
			url:              "container://ghcr.io/org/team/project:tag",
			expectedImageRef: "ghcr.io/org/team/project:tag",
			expectError:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))

			ctx := context.Background()
			oci, err := NewOCI(ctx, tt.url, logger)

			if tt.expectError {
				if err == nil {
					t.Fatal("Expected error, but got none")
				}
				return
			}

			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			if oci.ImageRef != tt.expectedImageRef {
				t.Errorf("Expected imageRef %s, got %s", tt.expectedImageRef, oci.ImageRef)
			}

			if oci.logger == nil {
				t.Error("Expected logger to be set")
			}
		})
	}
}

func TestOCI_ImageRefParsing(t *testing.T) {
	tests := []struct {
		name     string
		url      string
		expected string
	}{
		{
			name:     "container hub official image",
			url:      "container://docker.io/library/nginx:latest",
			expected: "docker.io/library/nginx:latest",
		},
		{
			name:     "ghcr with org and repo",
			url:      "container://ghcr.io/linyows/dewy:v1.0.0",
			expected: "ghcr.io/linyows/dewy:v1.0.0",
		},
		{
			name:     "local registry",
			url:      "container://localhost:5555/testapp:v2",
			expected: "localhost:5555/testapp:v2",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
			ctx := context.Background()

			oci, err := NewOCI(ctx, tt.url, logger)
			if err != nil {
				t.Fatalf("Failed to create OCI: %v", err)
			}

			if oci.ImageRef != tt.expected {
				t.Errorf("Expected ImageRef %s, got %s", tt.expected, oci.ImageRef)
			}
		})
	}
}

// Note: TestOCI_Download is not included in unit tests because it requires
// actual docker daemon and would execute 'docker pull' command.
// This will be tested in integration tests instead.
