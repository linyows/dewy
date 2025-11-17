package dewy

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/linyows/dewy/container"
)

func TestFindDewySocketFiles(t *testing.T) {
	// Create a temporary directory for testing
	tmpDir := t.TempDir()

	// Change to temp directory for test
	originalDir, _ := os.Getwd()
	defer func() {
		_ = os.Chdir(originalDir)
	}()

	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("Failed to change to temp directory: %v", err)
	}

	tests := []struct {
		name      string
		setupFunc func() error
		wantFound bool
	}{
		{
			name: "socket exists",
			setupFunc: func() error {
				dewyDir := filepath.Join(tmpDir, ".dewy")
				if err := os.MkdirAll(dewyDir, 0755); err != nil {
					return err
				}
				socketPath := filepath.Join(dewyDir, "api.sock")
				return os.WriteFile(socketPath, []byte{}, 0600)
			},
			wantFound: true,
		},
		{
			name: "socket does not exist",
			setupFunc: func() error {
				// Clean up .dewy directory
				os.RemoveAll(filepath.Join(tmpDir, ".dewy"))
				return nil
			},
			wantFound: false,
		},
		{
			name: ".dewy directory exists but no socket",
			setupFunc: func() error {
				dewyDir := filepath.Join(tmpDir, ".dewy")
				if err := os.MkdirAll(dewyDir, 0755); err != nil {
					return err
				}
				// Remove socket if exists
				os.Remove(filepath.Join(dewyDir, "api.sock"))
				return nil
			},
			wantFound: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.setupFunc(); err != nil {
				t.Fatalf("Setup failed: %v", err)
			}

			c := &cli{}
			got, err := c.findDewySocketFiles()

			if err != nil {
				t.Errorf("findDewySocketFiles() unexpected error: %v", err)
				return
			}

			found := len(got) > 0
			if found != tt.wantFound {
				t.Errorf("findDewySocketFiles() found = %v, want %v", found, tt.wantFound)
			}
		})
	}
}

func TestDisplayContainerList(t *testing.T) {
	tests := []struct {
		name       string
		containers []*container.Info
		wantOutput string
	}{
		{
			name:       "empty list",
			containers: []*container.Info{},
			wantOutput: "No containers found.\n",
		},
		{
			name: "single container",
			containers: []*container.Info{
				{
					ID:         "abc123",
					Name:       "myapp-0",
					Image:      "nginx:latest",
					Status:     "running",
					IPPort:     "127.0.0.1:8080",
					StartedAt:  time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC),
					DeployedAt: time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC),
				},
			},
			wantOutput: "UPSTREAM",
		},
		{
			name: "multiple containers sorted by name",
			containers: []*container.Info{
				{
					ID:         "def456",
					Name:       "myapp-2",
					Image:      "nginx:latest",
					Status:     "running",
					IPPort:     "127.0.0.1:8082",
					StartedAt:  time.Date(2025, 1, 15, 10, 2, 0, 0, time.UTC),
					DeployedAt: time.Date(2025, 1, 15, 10, 2, 0, 0, time.UTC),
				},
				{
					ID:         "abc123",
					Name:       "myapp-0",
					Image:      "nginx:latest",
					Status:     "running",
					IPPort:     "127.0.0.1:8080",
					StartedAt:  time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC),
					DeployedAt: time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC),
				},
				{
					ID:         "ghi789",
					Name:       "myapp-1",
					Image:      "nginx:latest",
					Status:     "running",
					IPPort:     "127.0.0.1:8081",
					StartedAt:  time.Date(2025, 1, 15, 10, 1, 0, 0, time.UTC),
					DeployedAt: time.Date(2025, 1, 15, 10, 1, 0, 0, time.UTC),
				},
			},
			wantOutput: "myapp-0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			c := &cli{
				env: Env{
					Out: &buf,
					Err: &buf,
				},
			}

			c.displayContainerList(tt.containers)

			got := buf.String()
			if !strings.Contains(got, tt.wantOutput) {
				t.Errorf("displayContainerList() output does not contain %q\nGot:\n%s", tt.wantOutput, got)
			}

			// For multiple containers, verify they are sorted
			if tt.name == "multiple containers sorted by name" {
				lines := strings.Split(strings.TrimSpace(got), "\n")
				if len(lines) < 4 {
					t.Errorf("Expected at least 4 lines (header + 3 containers), got %d", len(lines))
					return
				}

				// Check that myapp-0 comes before myapp-1, which comes before myapp-2
				output := strings.Join(lines, "\n")
				idx0 := strings.Index(output, "myapp-0")
				idx1 := strings.Index(output, "myapp-1")
				idx2 := strings.Index(output, "myapp-2")

				if idx0 == -1 || idx1 == -1 || idx2 == -1 {
					t.Errorf("Not all container names found in output")
					return
				}

				if idx0 >= idx1 || idx1 >= idx2 {
					t.Errorf("Containers are not sorted correctly: myapp-0 at %d, myapp-1 at %d, myapp-2 at %d",
						idx0, idx1, idx2)
				}
			}
		})
	}
}

func TestGetAdminSocketPath(t *testing.T) {
	c := &cli{}
	got := c.getAdminSocketPath()

	// Should end with .dewy/api.sock
	if !strings.HasSuffix(got, filepath.Join(".dewy", "api.sock")) {
		t.Errorf("getAdminSocketPath() = %q, want suffix .dewy/api.sock", got)
	}

	// Should be an absolute path or contain current directory reference
	if !filepath.IsAbs(got) && !strings.Contains(got, ".dewy") {
		t.Errorf("getAdminSocketPath() = %q, expected to contain .dewy", got)
	}
}
