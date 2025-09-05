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
func NewGitHub() (*github.Client, error) {
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

	// Handle custom API URL - support both GITHUB_ENDPOINT and GITHUB_API_URL
	apiURL := os.Getenv("GITHUB_API_URL")
	if apiURL == "" {
		apiURL = os.Getenv("GITHUB_ENDPOINT")
	}
	if apiURL != "" {
		baseURL, err := url.Parse(apiURL)
		if err != nil {
			return nil, fmt.Errorf("invalid API URL: %w", err)
		}
		// Ensure the URL has a trailing slash as required by go-github
		if baseURL.Path == "" {
			baseURL.Path = "/"
		} else if baseURL.Path[len(baseURL.Path)-1] != '/' {
			baseURL.Path += "/"
		}
		client.BaseURL = baseURL
	}

	return client, nil
}

// NewMockGitHub creates a GitHub client with a mock HTTP client for testing.
func NewMockGitHub(httpClient *http.Client) *github.Client {
	return github.NewClient(httpClient)
}
