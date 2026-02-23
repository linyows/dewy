package container

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"testing"
	"time"
)

func TestNewPodman(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	drainTime := 30 * time.Second

	podman, err := NewPodman(logger, drainTime)

	// This test may fail if podman is not installed
	// In CI environments without podman, this is expected
	if err != nil {
		if errors.Is(err, ErrRuntimeNotFound) {
			t.Skip("Podman not found, skipping test")
		}
		t.Fatalf("Failed to create Podman runtime: %v", err)
	}

	if podman.cmd != "podman" {
		t.Errorf("Expected cmd to be 'podman', got %s", podman.cmd)
	}

	if podman.drainTime != drainTime {
		t.Errorf("Expected drainTime to be %v, got %v", drainTime, podman.drainTime)
	}

	if podman.logger == nil {
		t.Error("Expected logger to be set")
	}
}

func TestPodmanRunOptions(t *testing.T) {
	opts := RunOptions{
		Image:        "nginx:latest",
		AppName:      "test-app",
		ReplicaIndex: 0,
		Command:      []string{"nginx", "-g", "daemon off;"},
		ExtraArgs:    []string{"-e", "FOO=bar", "-v", "/host:/container"},
		Labels: map[string]string{
			"app":  "test",
			"role": "blue",
		},
		Detach: true,
	}

	if opts.Image != "nginx:latest" {
		t.Errorf("Expected image nginx:latest, got %s", opts.Image)
	}

	if opts.AppName != "test-app" {
		t.Errorf("Expected appName test-app, got %s", opts.AppName)
	}

	if len(opts.Command) != 3 {
		t.Errorf("Expected 3 command arguments, got %d", len(opts.Command))
	}

	if len(opts.ExtraArgs) != 4 {
		t.Errorf("Expected 4 extra arguments, got %d", len(opts.ExtraArgs))
	}

	if len(opts.Labels) != 2 {
		t.Errorf("Expected 2 labels, got %d", len(opts.Labels))
	}
}

func TestPodmanDeployOptions(t *testing.T) {
	healthCheck := func(ctx context.Context, containerID string) error {
		return nil
	}

	opts := DeployOptions{
		ImageRef:      "ghcr.io/linyows/myapp:v1.0.0",
		AppName:       "myapp",
		ContainerPort: 8080,
		Command:       []string{"node", "server.js"},
		ExtraArgs:     []string{"-e", "APP_ENV=production", "-v", "/data:/app/data"},
		HealthCheck:   healthCheck,
	}

	if opts.ImageRef != "ghcr.io/linyows/myapp:v1.0.0" {
		t.Errorf("Expected imageRef ghcr.io/linyows/myapp:v1.0.0, got %s", opts.ImageRef)
	}

	if opts.AppName != "myapp" {
		t.Errorf("Expected appName myapp, got %s", opts.AppName)
	}

	if opts.ContainerPort != 8080 {
		t.Errorf("Expected containerPort 8080, got %d", opts.ContainerPort)
	}

	if len(opts.Command) != 2 {
		t.Errorf("Expected 2 command arguments, got %d", len(opts.Command))
	}

	if len(opts.ExtraArgs) != 4 {
		t.Errorf("Expected 4 extra arguments, got %d", len(opts.ExtraArgs))
	}

	if opts.HealthCheck == nil {
		t.Error("Expected healthCheck to be set")
	}
}

func TestPodmanHasUserOption(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		expected bool
	}{
		{
			name:     "empty args",
			args:     []string{},
			expected: false,
		},
		{
			name:     "no user option",
			args:     []string{"-e", "FOO=bar", "-v", "/host:/container"},
			expected: false,
		},
		{
			name:     "--user with space",
			args:     []string{"--user", "1000:1000"},
			expected: true,
		},
		{
			name:     "--user=xxx",
			args:     []string{"--user=1000:1000"},
			expected: true,
		},
		{
			name:     "-u with space",
			args:     []string{"-u", "1000:1000"},
			expected: true,
		},
		{
			name:     "-u=xxx",
			args:     []string{"-u=1000:1000"},
			expected: true,
		},
		{
			name:     "-u1000 combined",
			args:     []string{"-u1000:1000"},
			expected: true,
		},
		{
			name:     "--user with other args",
			args:     []string{"-e", "FOO=bar", "--user", "1000:1000", "-v", "/host:/container"},
			expected: true,
		},
		{
			name:     "-u with other args",
			args:     []string{"-e", "FOO=bar", "-u", "root", "-v", "/host:/container"},
			expected: true,
		},
		{
			name:     "--user=0:0 root",
			args:     []string{"--user=0:0"},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := hasUserOption(tt.args)
			if result != tt.expected {
				t.Errorf("hasUserOption(%v) = %v, expected %v", tt.args, result, tt.expected)
			}
		})
	}
}

func TestPodmanValidateExtraArgs(t *testing.T) {
	tests := []struct {
		name        string
		args        []string
		expectError bool
	}{
		{
			name:        "valid args",
			args:        []string{"-e", "FOO=bar", "-v", "/host:/container"},
			expectError: false,
		},
		{
			name:        "forbidden --privileged",
			args:        []string{"--privileged", "-e", "FOO=bar"},
			expectError: true,
		},
		{
			name:        "forbidden --pid",
			args:        []string{"--pid=host"},
			expectError: true,
		},
		{
			name:        "forbidden --cap-add",
			args:        []string{"--cap-add=SYS_ADMIN"},
			expectError: true,
		},
		{
			name:        "forbidden --security-opt",
			args:        []string{"--security-opt=seccomp=unconfined"},
			expectError: true,
		},
		{
			name:        "forbidden --device",
			args:        []string{"--device=/dev/sda"},
			expectError: true,
		},
		{
			name:        "forbidden --userns",
			args:        []string{"--userns=host"},
			expectError: true,
		},
		{
			name:        "forbidden --cgroupns",
			args:        []string{"--cgroupns=host"},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateExtraArgs(tt.args)
			if tt.expectError && err == nil {
				t.Errorf("validateExtraArgs(%v) expected error, got nil", tt.args)
			}
			if !tt.expectError && err != nil {
				t.Errorf("validateExtraArgs(%v) expected no error, got %v", tt.args, err)
			}
		})
	}
}

// Note: Tests for actual Podman operations (Pull, Run, Stop, etc.) are not
// included in unit tests because they require a running Podman daemon.
// These will be tested in integration tests instead.
