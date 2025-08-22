package client

import (
	"net/http"
	"net/url"
	"os"
	"testing"
)

func TestNewGitHub(t *testing.T) {
	tests := []struct {
		name            string
		githubToken     string
		ghToken         string
		githubAPIURL    string
		githubEndpoint  string
		expectedError   bool
		expectedBaseURL string
		cleanupEnvVars  []string
	}{
		{
			name:           "valid GITHUB_TOKEN",
			githubToken:    "ghp_test_token",
			expectedError:  false,
			cleanupEnvVars: []string{"GITHUB_TOKEN"},
		},
		{
			name:           "valid GH_TOKEN",
			ghToken:        "ghp_test_token",
			expectedError:  false,
			cleanupEnvVars: []string{"GH_TOKEN"},
		},
		{
			name:           "GITHUB_TOKEN takes precedence over GH_TOKEN",
			githubToken:    "ghp_github_token",
			ghToken:        "ghp_gh_token",
			expectedError:  false,
			cleanupEnvVars: []string{"GITHUB_TOKEN", "GH_TOKEN"},
		},
		{
			name:          "no token provided",
			expectedError: true,
		},
		{
			name:            "with GITHUB_API_URL",
			githubToken:     "ghp_test_token",
			githubAPIURL:    "https://api.github.enterprise.com/",
			expectedError:   false,
			expectedBaseURL: "https://api.github.enterprise.com/",
			cleanupEnvVars:  []string{"GITHUB_TOKEN", "GITHUB_API_URL"},
		},
		{
			name:            "with GITHUB_ENDPOINT",
			githubToken:     "ghp_test_token",
			githubEndpoint:  "https://api.github.enterprise.com/",
			expectedError:   false,
			expectedBaseURL: "https://api.github.enterprise.com/",
			cleanupEnvVars:  []string{"GITHUB_TOKEN", "GITHUB_ENDPOINT"},
		},
		{
			name:            "GITHUB_API_URL takes precedence over GITHUB_ENDPOINT",
			githubToken:     "ghp_test_token",
			githubAPIURL:    "https://api1.github.enterprise.com/",
			githubEndpoint:  "https://api2.github.enterprise.com/",
			expectedError:   false,
			expectedBaseURL: "https://api1.github.enterprise.com/",
			cleanupEnvVars:  []string{"GITHUB_TOKEN", "GITHUB_API_URL", "GITHUB_ENDPOINT"},
		},
		{
			name:           "invalid API URL",
			githubToken:    "ghp_test_token",
			githubAPIURL:   "://invalid-url",
			expectedError:  true,
			cleanupEnvVars: []string{"GITHUB_TOKEN", "GITHUB_API_URL"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clean up environment variables before each test
			for _, envVar := range tt.cleanupEnvVars {
				defer func(key string) {
					os.Unsetenv(key)
				}(envVar)
			}

			// Set up environment variables
			if tt.githubToken != "" {
				os.Setenv("GITHUB_TOKEN", tt.githubToken)
			}
			if tt.ghToken != "" {
				os.Setenv("GH_TOKEN", tt.ghToken)
			}
			if tt.githubAPIURL != "" {
				os.Setenv("GITHUB_API_URL", tt.githubAPIURL)
			}
			if tt.githubEndpoint != "" {
				os.Setenv("GITHUB_ENDPOINT", tt.githubEndpoint)
			}

			client, err := NewGitHub()

			if tt.expectedError {
				if err == nil {
					t.Errorf("Expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			if client == nil {
				t.Error("Expected client but got nil")
				return
			}

			// Check base URL if specified
			if tt.expectedBaseURL != "" {
				expectedURL, _ := url.Parse(tt.expectedBaseURL)
				if client.BaseURL.String() != expectedURL.String() {
					t.Errorf("Expected BaseURL %s, got %s", expectedURL.String(), client.BaseURL.String())
				}
			}
		})
	}
}

func TestNewGitHub_TokenPrecedence(t *testing.T) {
	// Clean up after test
	defer func() {
		os.Unsetenv("GITHUB_TOKEN")
		os.Unsetenv("GH_TOKEN")
	}()

	// Set both tokens
	os.Setenv("GITHUB_TOKEN", "github_token")
	os.Setenv("GH_TOKEN", "gh_token")

	client, err := NewGitHub()
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if client == nil {
		t.Fatal("Expected client but got nil")
	}

	// We can't directly check which token was used, but we can verify a client was created
	// In a real test, you might check the Authorization header in the underlying HTTP client
}

func TestNewGitHub_EnvironmentIsolation(t *testing.T) {
	// Save original environment
	originalGitHubToken := os.Getenv("GITHUB_TOKEN")
	originalGHToken := os.Getenv("GH_TOKEN")
	originalAPIURL := os.Getenv("GITHUB_API_URL")
	originalEndpoint := os.Getenv("GITHUB_ENDPOINT")

	// Clean up environment
	os.Unsetenv("GITHUB_TOKEN")
	os.Unsetenv("GH_TOKEN")
	os.Unsetenv("GITHUB_API_URL")
	os.Unsetenv("GITHUB_ENDPOINT")

	// Restore original environment after test
	defer func() {
		if originalGitHubToken != "" {
			os.Setenv("GITHUB_TOKEN", originalGitHubToken)
		}
		if originalGHToken != "" {
			os.Setenv("GH_TOKEN", originalGHToken)
		}
		if originalAPIURL != "" {
			os.Setenv("GITHUB_API_URL", originalAPIURL)
		}
		if originalEndpoint != "" {
			os.Setenv("GITHUB_ENDPOINT", originalEndpoint)
		}
	}()

	// Test with no tokens
	_, err := NewGitHub()
	if err == nil {
		t.Error("Expected error when no tokens are provided")
	}
}

func TestNewMockGitHub(t *testing.T) {
	tests := []struct {
		name       string
		httpClient *http.Client
	}{
		{
			name:       "with custom HTTP client",
			httpClient: &http.Client{},
		},
		{
			name:       "with nil HTTP client",
			httpClient: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := NewMockGitHub(tt.httpClient)

			if client == nil {
				t.Error("Expected client but got nil")
				return
			}

			// Verify it's a valid GitHub client
			if client.BaseURL == nil {
				t.Error("Expected BaseURL to be set")
			}
		})
	}
}

func TestNewMockGitHub_ClientCreation(t *testing.T) {
	httpClient := &http.Client{}
	githubClient := NewMockGitHub(httpClient)

	if githubClient == nil {
		t.Fatal("Expected GitHub client but got nil")
	}

	// Verify default GitHub API URL is set
	expectedURL := "https://api.github.com/"
	if githubClient.BaseURL.String() != expectedURL {
		t.Errorf("Expected BaseURL %s, got %s", expectedURL, githubClient.BaseURL.String())
	}
}

// Test for edge cases and error conditions.
func TestNewGitHub_EdgeCases(t *testing.T) {
	tests := []struct {
		name      string
		setupFunc func()
		cleanup   func()
		wantError bool
	}{
		{
			name: "empty GITHUB_TOKEN",
			setupFunc: func() {
				os.Setenv("GITHUB_TOKEN", "")
				os.Setenv("GH_TOKEN", "valid_token")
			},
			cleanup: func() {
				os.Unsetenv("GITHUB_TOKEN")
				os.Unsetenv("GH_TOKEN")
			},
			wantError: false, // Should use GH_TOKEN
		},
		{
			name: "whitespace only token",
			setupFunc: func() {
				os.Setenv("GITHUB_TOKEN", "   ")
			},
			cleanup: func() {
				os.Unsetenv("GITHUB_TOKEN")
			},
			wantError: false, // Whitespace is considered a valid token
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.setupFunc != nil {
				tt.setupFunc()
			}
			if tt.cleanup != nil {
				defer tt.cleanup()
			}

			client, err := NewGitHub()

			if tt.wantError && err == nil {
				t.Error("Expected error but got none")
			}
			if !tt.wantError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
			if !tt.wantError && client == nil {
				t.Error("Expected client but got nil")
			}
		})
	}
}
