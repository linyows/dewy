package client

import (
	"context"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v4"
	"github.com/google/go-github/v73/github"
)

const (
	// GitHub App JWT expires in 10 minutes (max allowed by GitHub)
	jwtExpiration = 10 * time.Minute
	// Refresh installation token 5 minutes before expiration
	tokenRefreshBuffer = 5 * time.Minute
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

// githubAppTransport is an http.RoundTripper that authenticates requests
// using GitHub App installation tokens.
type githubAppTransport struct {
	baseTransport  http.RoundTripper
	appID          int64
	installationID int64
	privateKey     *rsa.PrivateKey
	baseURL        string
	logger         *slog.Logger

	mu          sync.RWMutex
	token       string
	tokenExpiry time.Time
}

// parsePrivateKey parses a PEM encoded RSA private key.
func parsePrivateKey(pemBytes []byte) (*rsa.PrivateKey, error) {
	block, _ := pem.Decode(pemBytes)
	if block == nil {
		return nil, fmt.Errorf("failed to parse PEM block")
	}

	// Try PKCS#1 format first (BEGIN RSA PRIVATE KEY)
	if key, err := x509.ParsePKCS1PrivateKey(block.Bytes); err == nil {
		return key, nil
	}

	// Try PKCS#8 format (BEGIN PRIVATE KEY)
	if key, err := x509.ParsePKCS8PrivateKey(block.Bytes); err == nil {
		if rsaKey, ok := key.(*rsa.PrivateKey); ok {
			return rsaKey, nil
		}
		return nil, fmt.Errorf("not an RSA private key")
	}

	return nil, fmt.Errorf("failed to parse private key")
}

// NewGitHubAppTransport creates an HTTP transport for GitHub App authentication.
// The baseURL parameter should be empty for github.com or the API URL for GitHub Enterprise Server.
// The logger parameter is optional; if nil, logging is disabled.
func NewGitHubAppTransport(config *GitHubAppConfig, baseURL string, logger *slog.Logger) (http.RoundTripper, error) {
	if config == nil {
		return nil, fmt.Errorf("config is required")
	}

	privateKey, err := parsePrivateKey(config.PrivateKey)
	if err != nil {
		return nil, fmt.Errorf("failed to parse private key: %w", err)
	}

	return &githubAppTransport{
		baseTransport:  http.DefaultTransport,
		appID:          config.AppID,
		installationID: config.InstallationID,
		privateKey:     privateKey,
		baseURL:        baseURL,
		logger:         logger,
	}, nil
}

// RoundTrip implements http.RoundTripper.
func (t *githubAppTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	token, err := t.getToken(req.Context())
	if err != nil {
		return nil, fmt.Errorf("failed to get installation token: %w", err)
	}

	// Clone the request to avoid modifying the original
	req2 := req.Clone(req.Context())
	req2.Header.Set("Authorization", "Bearer "+token)

	return t.baseTransport.RoundTrip(req2)
}

// getToken returns a valid installation token, refreshing if necessary.
func (t *githubAppTransport) getToken(ctx context.Context) (string, error) {
	t.mu.RLock()
	if t.token != "" && time.Now().Add(tokenRefreshBuffer).Before(t.tokenExpiry) {
		token := t.token
		t.mu.RUnlock()
		return token, nil
	}
	isFirstToken := t.token == ""
	t.mu.RUnlock()

	t.mu.Lock()
	defer t.mu.Unlock()

	// Double-check after acquiring write lock
	if t.token != "" && time.Now().Add(tokenRefreshBuffer).Before(t.tokenExpiry) {
		return t.token, nil
	}

	// Generate new installation token
	token, expiry, err := t.createInstallationToken(ctx)
	if err != nil {
		return "", err
	}

	if t.logger != nil {
		if isFirstToken {
			t.logger.Info("GitHub App: installation token acquired",
				slog.Int64("app_id", t.appID),
				slog.Int64("installation_id", t.installationID),
				slog.Time("expires_at", expiry),
			)
		} else {
			t.logger.Info("GitHub App: installation token refreshed",
				slog.Int64("app_id", t.appID),
				slog.Int64("installation_id", t.installationID),
				slog.Time("expires_at", expiry),
			)
		}
	}

	t.token = token
	t.tokenExpiry = expiry

	return token, nil
}

// createJWT creates a JWT for GitHub App authentication.
func (t *githubAppTransport) createJWT() (string, error) {
	now := time.Now()
	claims := jwt.RegisteredClaims{
		IssuedAt:  jwt.NewNumericDate(now.Add(-60 * time.Second)), // Allow for clock drift
		ExpiresAt: jwt.NewNumericDate(now.Add(jwtExpiration)),
		Issuer:    strconv.FormatInt(t.appID, 10),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	return token.SignedString(t.privateKey)
}

// createInstallationToken exchanges a JWT for an installation access token.
func (t *githubAppTransport) createInstallationToken(ctx context.Context) (string, time.Time, error) {
	jwtToken, err := t.createJWT()
	if err != nil {
		return "", time.Time{}, fmt.Errorf("failed to create JWT: %w", err)
	}

	// Create a temporary client with JWT auth to get installation token
	httpClient := &http.Client{
		Transport: &jwtTransport{
			baseTransport: t.baseTransport,
			jwt:           jwtToken,
		},
	}

	client := github.NewClient(httpClient)
	if t.baseURL != "" {
		var err error
		client, err = client.WithEnterpriseURLs(t.baseURL, t.baseURL)
		if err != nil {
			return "", time.Time{}, fmt.Errorf("failed to set enterprise URL: %w", err)
		}
	}

	installationToken, _, err := client.Apps.CreateInstallationToken(ctx, t.installationID, nil)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("failed to create installation token: %w", err)
	}

	return installationToken.GetToken(), installationToken.GetExpiresAt().Time, nil
}

// jwtTransport is a simple http.RoundTripper that adds JWT auth header.
type jwtTransport struct {
	baseTransport http.RoundTripper
	jwt           string
}

func (t *jwtTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req2 := req.Clone(req.Context())
	req2.Header.Set("Authorization", "Bearer "+t.jwt)
	req2.Header.Set("Accept", "application/vnd.github+json")
	return t.baseTransport.RoundTrip(req2)
}
