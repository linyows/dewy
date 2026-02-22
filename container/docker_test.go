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
			name:     "-u alone without value",
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

func TestExtractRegistry(t *testing.T) {
	tests := []struct {
		name     string
		imageRef string
		expected string
	}{
		{
			name:     "Docker Hub official image",
			imageRef: "nginx",
			expected: "docker.io",
		},
		{
			name:     "Docker Hub official image with tag",
			imageRef: "nginx:latest",
			expected: "docker.io",
		},
		{
			name:     "Docker Hub user image",
			imageRef: "library/nginx:latest",
			expected: "docker.io",
		},
		{
			name:     "GitHub Container Registry",
			imageRef: "ghcr.io/owner/repo:v1.0.0",
			expected: "ghcr.io",
		},
		{
			name:     "GitHub Container Registry with digest",
			imageRef: "ghcr.io/owner/repo@sha256:abc123",
			expected: "ghcr.io",
		},
		{
			name:     "AWS ECR",
			imageRef: "123456789.dkr.ecr.us-east-1.amazonaws.com/myrepo:v1.0.0",
			expected: "123456789.dkr.ecr.us-east-1.amazonaws.com",
		},
		{
			name:     "Google Container Registry",
			imageRef: "gcr.io/my-project/myimage:latest",
			expected: "gcr.io",
		},
		{
			name:     "Google Artifact Registry",
			imageRef: "us-docker.pkg.dev/my-project/my-repo/myimage:latest",
			expected: "us-docker.pkg.dev",
		},
		{
			name:     "localhost registry",
			imageRef: "localhost:5000/myimage:latest",
			expected: "localhost:5000",
		},
		{
			name:     "localhost without port",
			imageRef: "localhost/myimage:latest",
			expected: "localhost",
		},
		{
			name:     "private registry with port",
			imageRef: "registry.example.com:5000/myimage:v1.0.0",
			expected: "registry.example.com:5000",
		},
		{
			name:     "private registry without port",
			imageRef: "registry.example.com/myimage:v1.0.0",
			expected: "registry.example.com",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractRegistry(tt.imageRef)
			if result != tt.expected {
				t.Errorf("extractRegistry(%q) = %q, expected %q", tt.imageRef, result, tt.expected)
			}
		})
	}
}

func TestGetCredentials(t *testing.T) {
	tests := []struct {
		name             string
		registry         string
		envVars          map[string]string
		expectedUsername string
		expectedPassword string
	}{
		{
			name:     "GitHub Container Registry with GITHUB_TOKEN",
			registry: "ghcr.io",
			envVars: map[string]string{
				"GITHUB_TOKEN": "ghp_test_token",
			},
			expectedUsername: "token",
			expectedPassword: "ghp_test_token",
		},
		{
			name:     "Generic registry with DOCKER_USERNAME/PASSWORD",
			registry: "docker.io",
			envVars: map[string]string{
				"DOCKER_USERNAME": "myuser",
				"DOCKER_PASSWORD": "mypassword",
			},
			expectedUsername: "myuser",
			expectedPassword: "mypassword",
		},
		{
			name:     "AWS ECR with AWS_ECR_PASSWORD",
			registry: "123456789.dkr.ecr.us-east-1.amazonaws.com",
			envVars: map[string]string{
				"AWS_ECR_PASSWORD": "ecr-token",
			},
			expectedUsername: "AWS",
			expectedPassword: "ecr-token",
		},
		{
			name:     "Google Container Registry with GCR_TOKEN",
			registry: "gcr.io",
			envVars: map[string]string{
				"GCR_TOKEN": "gcr-json-key",
			},
			expectedUsername: "_json_key",
			expectedPassword: "gcr-json-key",
		},
		{
			name:             "No credentials",
			registry:         "docker.io",
			envVars:          map[string]string{},
			expectedUsername: "",
			expectedPassword: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clear all relevant env vars first
			envVarsToClean := []string{"GITHUB_TOKEN", "DOCKER_USERNAME", "DOCKER_PASSWORD", "AWS_ECR_PASSWORD", "GCR_TOKEN"}
			for _, key := range envVarsToClean {
				os.Unsetenv(key)
			}

			// Set test env vars
			for key, value := range tt.envVars {
				os.Setenv(key, value)
			}

			// Run test
			username, password := getCredentials(tt.registry)

			if username != tt.expectedUsername {
				t.Errorf("getCredentials(%q) username = %q, expected %q", tt.registry, username, tt.expectedUsername)
			}
			if password != tt.expectedPassword {
				t.Errorf("getCredentials(%q) password = %q, expected %q", tt.registry, password, tt.expectedPassword)
			}

			// Cleanup
			for key := range tt.envVars {
				os.Unsetenv(key)
			}
		})
	}
}

func TestIsAuthError(t *testing.T) {
	tests := []struct {
		name     string
		output   string
		expected bool
	}{
		{
			name:     "unauthorized error",
			output:   "Error response from daemon: unauthorized: authentication required",
			expected: true,
		},
		{
			name:     "denied error",
			output:   "Error response from daemon: denied: requested access to the resource is denied",
			expected: true,
		},
		{
			name:     "access forbidden error",
			output:   "Error: access forbidden",
			expected: true,
		},
		{
			name:     "login required",
			output:   "Error: login required",
			expected: true,
		},
		{
			name:     "not authorized",
			output:   "Error: not authorized to access this resource",
			expected: true,
		},
		{
			name:     "network error (not auth)",
			output:   "Error: dial tcp: lookup registry.example.com: no such host",
			expected: false,
		},
		{
			name:     "image not found (not auth)",
			output:   "Error: manifest unknown: manifest unknown",
			expected: false,
		},
		{
			name:     "empty output",
			output:   "",
			expected: false,
		},
		{
			name:     "success message",
			output:   "Successfully pulled image nginx:latest",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isAuthError(tt.output)
			if result != tt.expected {
				t.Errorf("isAuthError(%q) = %v, expected %v", tt.output, result, tt.expected)
			}
		})
	}
}

// Note: Tests for actual Docker operations (Pull, Run, Stop, etc.) are not
// included in unit tests because they require a running Docker daemon.
// These will be tested in integration tests instead.
