package artifact

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"testing"
)

type mockPuller struct {
	err error
}

func (m *mockPuller) Pull(ctx context.Context, imageRef string) error {
	return m.err
}

func TestNewOCI(t *testing.T) {
	tests := []struct {
		name             string
		url              string
		expectedImageRef string
		expectError      bool
	}{
		{
			name:             "container scheme with registry and tag",
			url:              "img://ghcr.io/linyows/myapp:v1.0.0",
			expectedImageRef: "ghcr.io/linyows/myapp:v1.0.0",
			expectError:      false,
		},
		{
			name:             "with port number",
			url:              "img://localhost:5000/testapp:v1",
			expectedImageRef: "localhost:5000/testapp:v1",
			expectError:      false,
		},
		{
			name:             "nested repository path",
			url:              "img://ghcr.io/org/team/project:tag",
			expectedImageRef: "ghcr.io/org/team/project:tag",
			expectError:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))

			ctx := context.Background()
			oci, err := NewOCI(ctx, tt.url, &mockPuller{}, logger)

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
			url:      "img://docker.io/library/nginx:latest",
			expected: "docker.io/library/nginx:latest",
		},
		{
			name:     "ghcr with org and repo",
			url:      "img://ghcr.io/linyows/dewy:v1.0.0",
			expected: "ghcr.io/linyows/dewy:v1.0.0",
		},
		{
			name:     "local registry",
			url:      "img://localhost:5555/testapp:v2",
			expected: "localhost:5555/testapp:v2",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
			ctx := context.Background()

			oci, err := NewOCI(ctx, tt.url, &mockPuller{}, logger)
			if err != nil {
				t.Fatalf("Failed to create OCI: %v", err)
			}

			if oci.ImageRef != tt.expected {
				t.Errorf("Expected ImageRef %s, got %s", tt.expected, oci.ImageRef)
			}
		})
	}
}

func TestOCI_Download_Success(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	ctx := context.Background()

	oci, err := NewOCI(ctx, "img://ghcr.io/test/app:v1", &mockPuller{}, logger)
	if err != nil {
		t.Fatalf("Failed to create OCI: %v", err)
	}

	var buf bytes.Buffer
	err = oci.Download(ctx, &buf)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if !strings.Contains(buf.String(), "Pulled image: ghcr.io/test/app:v1") {
		t.Errorf("Expected confirmation message, got: %s", buf.String())
	}
}

func TestOCI_Download_PullError(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	ctx := context.Background()

	puller := &mockPuller{err: fmt.Errorf("auth failed")}
	oci, err := NewOCI(ctx, "img://ghcr.io/test/app:v1", puller, logger)
	if err != nil {
		t.Fatalf("Failed to create OCI: %v", err)
	}

	err = oci.Download(ctx, nil)
	if err == nil {
		t.Fatal("Expected error, got nil")
	}
	if !strings.Contains(err.Error(), "pull failed") {
		t.Errorf("Expected 'pull failed' error, got: %v", err)
	}
	if !strings.Contains(err.Error(), "auth failed") {
		t.Errorf("Expected wrapped 'auth failed' error, got: %v", err)
	}
}
