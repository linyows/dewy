package container

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"testing"
	"time"
)

func TestNewDocker(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	drainTime := 30 * time.Second

	docker, err := NewDocker(logger, drainTime)

	// This test may fail if docker is not installed
	// In CI environments without docker, this is expected
	if err != nil {
		if errors.Is(err, ErrRuntimeNotFound) {
			t.Skip("Docker not found, skipping test")
		}
		t.Fatalf("Failed to create Docker runtime: %v", err)
	}

	if docker.cmd != "docker" {
		t.Errorf("Expected cmd to be 'docker', got %s", docker.cmd)
	}

	if docker.drainTime != drainTime {
		t.Errorf("Expected drainTime to be %v, got %v", drainTime, docker.drainTime)
	}

	if docker.logger == nil {
		t.Error("Expected logger to be set")
	}
}

func TestRunOptions(t *testing.T) {
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

func TestDeployOptions(t *testing.T) {
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

func TestContainer(t *testing.T) {
	c := Container{
		ID:     "abc123",
		Name:   "test-container",
		Image:  "nginx:latest",
		Status: "running",
		Labels: map[string]string{
			"app": "test",
		},
		Created: time.Now(),
	}

	if c.ID != "abc123" {
		t.Errorf("Expected ID abc123, got %s", c.ID)
	}

	if c.Status != "running" {
		t.Errorf("Expected status running, got %s", c.Status)
	}

	if c.Labels["app"] != "test" {
		t.Errorf("Expected label app=test, got %s", c.Labels["app"])
	}
}

func TestHasUserOption(t *testing.T) {
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
		{
			name:     "-u alone (followed by value)",
			args:     []string{"-u"},
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

func TestExtractNameOption(t *testing.T) {
	tests := []struct {
		name         string
		args         []string
		expectedName string
		expectedArgs []string
	}{
		{
			name:         "no name option",
			args:         []string{"-e", "FOO=bar"},
			expectedName: "",
			expectedArgs: []string{"-e", "FOO=bar"},
		},
		{
			name:         "--name with space",
			args:         []string{"--name", "mycontainer", "-e", "FOO=bar"},
			expectedName: "mycontainer",
			expectedArgs: []string{"-e", "FOO=bar"},
		},
		{
			name:         "--name=xxx",
			args:         []string{"--name=mycontainer", "-e", "FOO=bar"},
			expectedName: "mycontainer",
			expectedArgs: []string{"-e", "FOO=bar"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			name, filtered := extractNameOption(tt.args)
			if name != tt.expectedName {
				t.Errorf("extractNameOption(%v) name = %v, expected %v", tt.args, name, tt.expectedName)
			}
			if len(filtered) != len(tt.expectedArgs) {
				t.Errorf("extractNameOption(%v) filtered length = %v, expected %v", tt.args, len(filtered), len(tt.expectedArgs))
			}
			for i := range filtered {
				if filtered[i] != tt.expectedArgs[i] {
					t.Errorf("extractNameOption(%v) filtered[%d] = %v, expected %v", tt.args, i, filtered[i], tt.expectedArgs[i])
				}
			}
		})
	}
}

func TestValidateExtraArgs(t *testing.T) {
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
			name:        "forbidden -d",
			args:        []string{"-d", "-e", "FOO=bar"},
			expectError: true,
		},
		{
			name:        "forbidden --detach",
			args:        []string{"--detach", "-e", "FOO=bar"},
			expectError: true,
		},
		{
			name:        "forbidden -it",
			args:        []string{"-it", "-e", "FOO=bar"},
			expectError: true,
		},
		{
			name:        "forbidden -p",
			args:        []string{"-p", "8080:80"},
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

// Note: Tests for actual Docker operations (Pull, Run, Stop, etc.) are not
// included in unit tests because they require a running Docker daemon.
// These will be tested in integration tests instead.
