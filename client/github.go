package client

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"os"

	"github.com/google/go-github/v73/github"
	"golang.org/x/oauth2"
)

// NewGitHub creates a new GitHub client with authentication.
// Authentication priority:
//  1. GitHub App (if GITHUB_APP_ID is set)
//  2. PAT (GH_TOKEN > GITHUB_TOKEN)
func NewGitHub() (*github.Client, error) {
	// Get API URL for GitHub Enterprise Server support
	apiURL := getAPIURL()

	// Try GitHub App authentication first
	appConfig, err := LoadGitHubAppConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to load GitHub App config: %w", err)
	}

	if appConfig != nil {
		return newGitHubWithApp(appConfig, apiURL)
	}

	// Fall back to PAT authentication
	return newGitHubWithPAT(apiURL)
}

// newGitHubWithApp creates a GitHub client using GitHub App authentication.
func newGitHubWithApp(config *GitHubAppConfig, apiURL string) (*github.Client, error) {
	// For GitHub Enterprise Server, strip trailing slash for ghinstallation
	baseURLForTransport := ""
	if apiURL != "" {
		baseURLForTransport = apiURL
	}

	transport, err := NewGitHubAppTransport(config, baseURLForTransport)
	if err != nil {
		return nil, err
	}

	httpClient := &http.Client{Transport: transport}
	client := github.NewClient(httpClient)

	if apiURL != "" {
		if err := setClientBaseURL(client, apiURL); err != nil {
			return nil, err
		}
	}

	return client, nil
}

// newGitHubWithPAT creates a GitHub client using Personal Access Token authentication.
func newGitHubWithPAT(apiURL string) (*github.Client, error) {
	// Check GH_TOKEN first to avoid GitHub Actions auto-override of GITHUB_TOKEN
	token := os.Getenv("GH_TOKEN")
	if token == "" {
		token = os.Getenv("GITHUB_TOKEN")
	}

	if token == "" {
		return nil, fmt.Errorf("no GitHub token found in GITHUB_TOKEN or GH_TOKEN environment variables")
	}

	ctx := context.Background()
	ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: token})
	tc := oauth2.NewClient(ctx, ts)

	client := github.NewClient(tc)

	if apiURL != "" {
		if err := setClientBaseURL(client, apiURL); err != nil {
			return nil, err
		}
	}

	return client, nil
}

// getAPIURL returns the GitHub API URL from environment variables.
func getAPIURL() string {
	apiURL := os.Getenv("GITHUB_API_URL")
	if apiURL == "" {
		apiURL = os.Getenv("GITHUB_ENDPOINT")
	}
	return apiURL
}

// setClientBaseURL sets the base URL for the GitHub client.
func setClientBaseURL(client *github.Client, apiURL string) error {
	baseURL, err := url.Parse(apiURL)
	if err != nil {
		return fmt.Errorf("invalid API URL: %w", err)
	}
	// Ensure the URL has a trailing slash as required by go-github
	if baseURL.Path == "" {
		baseURL.Path = "/"
	} else if baseURL.Path[len(baseURL.Path)-1] != '/' {
		baseURL.Path += "/"
	}
	client.BaseURL = baseURL
	return nil
}

// NewMockGitHub creates a GitHub client with a mock HTTP client for testing.
func NewMockGitHub(httpClient *http.Client) *github.Client {
	return github.NewClient(httpClient)
}
