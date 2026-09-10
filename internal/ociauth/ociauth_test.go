package ociauth_test

import (
	"testing"

	"github.com/linyows/dewy/internal/ociauth"
)

// envKeys is every variable Credentials consults. Each case starts from a
// cleared environment so an inherited value cannot make a case pass.
var envKeys = []string{"DOCKER_USERNAME", "DOCKER_PASSWORD", "GITHUB_TOKEN", "AWS_ECR_PASSWORD", "GCR_TOKEN"}

func TestCredentials(t *testing.T) {
	tests := []struct {
		name     string
		registry string
		env      map[string]string
		wantUser string
		wantPass string
	}{
		{
			name:     "GitHub Container Registry with GITHUB_TOKEN",
			registry: "ghcr.io",
			env:      map[string]string{"GITHUB_TOKEN": "ghp_test_token"},
			wantUser: "token",
			wantPass: "ghp_test_token",
		},
		{
			name:     "Generic registry with DOCKER_USERNAME/PASSWORD",
			registry: "docker.io",
			env:      map[string]string{"DOCKER_USERNAME": "myuser", "DOCKER_PASSWORD": "mypassword"},
			wantUser: "myuser",
			wantPass: "mypassword",
		},
		{
			name:     "AWS ECR with AWS_ECR_PASSWORD",
			registry: "123456789.dkr.ecr.us-east-1.amazonaws.com",
			env:      map[string]string{"AWS_ECR_PASSWORD": "ecr-token"},
			wantUser: "AWS",
			wantPass: "ecr-token",
		},
		{
			name:     "Google Container Registry with GCR_TOKEN",
			registry: "gcr.io",
			env:      map[string]string{"GCR_TOKEN": "gcr-json-key"},
			wantUser: "_json_key",
			wantPass: "gcr-json-key",
		},
		{
			name:     "Google Artifact Registry with GCR_TOKEN",
			registry: "asia-northeast1-docker.pkg.dev",
			env:      map[string]string{"GCR_TOKEN": "gcr-json-key"},
			wantUser: "_json_key",
			wantPass: "gcr-json-key",
		},
		{
			name:     "No credentials",
			registry: "docker.io",
			env:      map[string]string{},
			wantUser: "",
			wantPass: "",
		},
		{
			// An explicit DOCKER_USERNAME wins over a host-specific token.
			// The container package used to prefer the host-specific one,
			// which meant the pull and the tag listing could authenticate as
			// two different identities.
			name:     "DOCKER_USERNAME takes precedence over GITHUB_TOKEN",
			registry: "ghcr.io",
			env: map[string]string{
				"DOCKER_USERNAME": "myuser",
				"DOCKER_PASSWORD": "mypassword",
				"GITHUB_TOKEN":    "ghp_test_token",
			},
			wantUser: "myuser",
			wantPass: "mypassword",
		},
		{
			// A host-specific token still applies when only a password is
			// set: DOCKER_USERNAME is what marks the explicit choice.
			name:     "DOCKER_PASSWORD alone does not shadow GITHUB_TOKEN",
			registry: "ghcr.io",
			env: map[string]string{
				"DOCKER_PASSWORD": "mypassword",
				"GITHUB_TOKEN":    "ghp_test_token",
			},
			wantUser: "token",
			wantPass: "ghp_test_token",
		},
		{
			// The host-specific branch is skipped when its own variable is
			// absent, leaving the DOCKER_PASSWORD fallback with no username.
			// Callers require both parts, so this is "no credentials".
			name:     "ghcr.io without GITHUB_TOKEN falls through",
			registry: "ghcr.io",
			env:      map[string]string{"DOCKER_PASSWORD": "mypassword"},
			wantUser: "",
			wantPass: "mypassword",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for _, key := range envKeys {
				t.Setenv(key, "")
			}
			for key, value := range tt.env {
				t.Setenv(key, value)
			}

			user, pass := ociauth.Credentials(tt.registry)
			if user != tt.wantUser {
				t.Errorf("Credentials(%q) username = %q, want %q", tt.registry, user, tt.wantUser)
			}
			if pass != tt.wantPass {
				t.Errorf("Credentials(%q) password = %q, want %q", tt.registry, pass, tt.wantPass)
			}
		})
	}
}
