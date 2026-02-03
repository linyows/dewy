package client

import (
	"fmt"
	"net/http"
	"os"
	"strconv"

	"github.com/bradleyfalzon/ghinstallation/v2"
)

// GitHubAppConfig holds the configuration for GitHub App authentication.
type GitHubAppConfig struct {
	AppID          int64
	InstallationID int64
	PrivateKey     []byte
}

// LoadGitHubAppConfig loads GitHub App configuration from environment variables.
// Returns nil if GitHub App configuration is not set.
// Required environment variables:
//   - GITHUB_APP_ID: The GitHub App ID
//   - GITHUB_APP_INSTALLATION_ID: The installation ID
//   - GITHUB_APP_PRIVATE_KEY or GITHUB_APP_PRIVATE_KEY_PATH: The private key (PEM format)
func LoadGitHubAppConfig() (*GitHubAppConfig, error) {
	appIDStr := os.Getenv("GITHUB_APP_ID")
	if appIDStr == "" {
		return nil, nil
	}

	appID, err := strconv.ParseInt(appIDStr, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid GITHUB_APP_ID: %w", err)
	}

	installationIDStr := os.Getenv("GITHUB_APP_INSTALLATION_ID")
	if installationIDStr == "" {
		return nil, fmt.Errorf("GITHUB_APP_INSTALLATION_ID is required when GITHUB_APP_ID is set")
	}

	installationID, err := strconv.ParseInt(installationIDStr, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid GITHUB_APP_INSTALLATION_ID: %w", err)
	}

	var privateKey []byte

	// Try GITHUB_APP_PRIVATE_KEY first (direct PEM content)
	privateKeyStr := os.Getenv("GITHUB_APP_PRIVATE_KEY")
	if privateKeyStr != "" {
		privateKey = []byte(privateKeyStr)
	} else {
		// Fall back to GITHUB_APP_PRIVATE_KEY_PATH (file path)
		privateKeyPath := os.Getenv("GITHUB_APP_PRIVATE_KEY_PATH")
		if privateKeyPath == "" {
			return nil, fmt.Errorf("either GITHUB_APP_PRIVATE_KEY or GITHUB_APP_PRIVATE_KEY_PATH is required")
		}

		privateKey, err = os.ReadFile(privateKeyPath)
		if err != nil {
			return nil, fmt.Errorf("failed to read private key file: %w", err)
		}
	}

	return &GitHubAppConfig{
		AppID:          appID,
		InstallationID: installationID,
		PrivateKey:     privateKey,
	}, nil
}

// NewGitHubAppTransport creates an HTTP transport for GitHub App authentication.
// The baseURL parameter should be empty for github.com or the API URL for GitHub Enterprise Server.
func NewGitHubAppTransport(config *GitHubAppConfig, baseURL string) (http.RoundTripper, error) {
	if config == nil {
		return nil, fmt.Errorf("config is required")
	}

	var transport *ghinstallation.Transport
	var err error

	if baseURL != "" {
		transport, err = ghinstallation.New(
			http.DefaultTransport,
			config.AppID,
			config.InstallationID,
			config.PrivateKey,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to create GitHub App transport: %w", err)
		}
		transport.BaseURL = baseURL
	} else {
		transport, err = ghinstallation.New(
			http.DefaultTransport,
			config.AppID,
			config.InstallationID,
			config.PrivateKey,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to create GitHub App transport: %w", err)
		}
	}

	return transport, nil
}
