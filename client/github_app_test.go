package client

import (
	"os"
	"path/filepath"
	"testing"
)

// Test private key for testing purposes only (not a real key).
// This is a PKCS#1 formatted RSA private key required by ghinstallation.
const testPrivateKey = `-----BEGIN RSA PRIVATE KEY-----
MIIEowIBAAKCAQEAutT7C/747czD2B+69fSiIbzJ0Fg1k5kPU9QVhCHmZR8ufHjG
umf6gJgKSRixUk79eQX8HAJbUFYfPU9XqoGHenuZCJkorUWKXPacKBLsaiYfZjGm
momD/Vm6eGX+7Kg3sdRQQiJTa+pFEMLAu7u7z2tBK05KRWEBPkCxjM/eO8pI52KS
xDNJxKXJs0jfCqMxNdKzQGmXilSno3ZsDCfhMkYrjTtcnqZeqr1qTFAh0Yyckgda
HwoD4iU6TW5p7xM18ZGyAHUDe/pm+9bdc0gZmcA8s/fr8jb2DpglhLYfWI3XZYOX
c6Am2IUOYIAVSSVMP/irWGPlLJC2WToZNEHaYQIDAQABAoIBABUSU1QpiCrN1uLH
xV7bHfQfJkXUxQomD6f9OgYCiskp6KTKPGOmuYaKX1KaMdmeJhFhvurryx+27uQL
0E/fNwf1668gEwnj13SfrcIJTFe2gAEXJXq1esj2t0TAAC3x1QI992VWGMGJlQuM
Y49o34hHyPxY4qCLjcfXJQ9EHITynMgETeZ1mmVJd+bIjVjgBMqjtFV5QxOZw0VG
eCUlu2C0Ci6ej5+SKFPf4bY8FFksCorHiqeiOiKXQZmX09di213oALWLGWdU1b7X
m6tXF9ZGvr4TD1x99YocEw9C57sjQFfgUfTrcz/zdOb6T23Kk3LF78PbW6t3iMVw
0yvge58CgYEA4IOnSPa5ukDwtQ3W7/rk4ygYH97GhZsA7rOp4XqLvWaqkEII37VA
5o24IfJJaAGkwHDQ1czG1PEzwuD8ba8dum/FW4b/xnqY/XzH2qgBJIoeiKfVfah+
ghLvDr0y3OVfPL3ZzVeX2cfYvtnvU830DytNxG3yCK0coFnDrhzMwW8CgYEA1Qh7
/zDo95PIS9PSdI/uCujPBzoq9M/m42oaG5QxxtsyN9pZmVv5dyR6ItI/2jQxgKHF
2oC+8KPnHxx1rRpW0Ehp3kY0qhJKiI6mMIfRUMKVqs5n0rD0WiS3MlYwtXOvUs0v
Fe7jxfFJhWtEJdSv+YyGGgamc+QKvLqJbcBhmS8CgYAmuK4WWG8p319kapGibAwj
3Vtjy8FDc7tSb+whtkf5j4ZlQO5U3ublnJWgTTA53uayRgLOjPXR7hO2TaVbqXMg
H3zTT1I3whc2yNmTLZyc17FycjfQ50mCV4+hZCIslOa7DCdPUgcfiWcpa17qfj/U
iexsr2Wp92lTgofMNK1fwwKBgQC0GFQbTNHmWzz9PbmxaOwotOAwj/A4vnnGz6/6
mLHsFurBZQpSJ/shyeim/2+TnIQs5pZJPoYtEaMWHg0tphK2SkGV82waSxRPlajR
ZkCCMb4thAkpiQdKHbfyCgNror0ZFvUzaZ2NfYpWDHS0NrX+FdpYrj6RwruBCYGd
EwJvaQKBgDLtg98jkE7xRHbBvpIw7xwKe0kgRUulOgtb9n1W7UeSDBKkWquNIjDT
WbLO82+im0uvFeUAl8aceqcNCdK5JHcjm3gstiAGzLFWcXDgwfI3PfS0xCClNhwm
+C6f2NVf6F691t7faPpASzvO8apuISReljfsFVkpNw1WOikFCzWH
-----END RSA PRIVATE KEY-----`

func TestLoadGitHubAppConfig(t *testing.T) {
	tests := []struct {
		name           string
		envVars        map[string]string
		privateKeyFile bool
		wantNil        bool
		wantError      bool
	}{
		{
			name:      "no config - returns nil",
			envVars:   map[string]string{},
			wantNil:   true,
			wantError: false,
		},
		{
			name: "valid config with inline private key",
			envVars: map[string]string{
				"GITHUB_APP_ID":              "123456",
				"GITHUB_APP_INSTALLATION_ID": "789012",
				"GITHUB_APP_PRIVATE_KEY":     testPrivateKey,
			},
			wantNil:   false,
			wantError: false,
		},
		{
			name: "valid config with private key file",
			envVars: map[string]string{
				"GITHUB_APP_ID":              "123456",
				"GITHUB_APP_INSTALLATION_ID": "789012",
			},
			privateKeyFile: true,
			wantNil:        false,
			wantError:      false,
		},
		{
			name: "invalid app ID",
			envVars: map[string]string{
				"GITHUB_APP_ID":              "invalid",
				"GITHUB_APP_INSTALLATION_ID": "789012",
				"GITHUB_APP_PRIVATE_KEY":     testPrivateKey,
			},
			wantNil:   true,
			wantError: true,
		},
		{
			name: "missing installation ID",
			envVars: map[string]string{
				"GITHUB_APP_ID":          "123456",
				"GITHUB_APP_PRIVATE_KEY": testPrivateKey,
			},
			wantNil:   true,
			wantError: true,
		},
		{
			name: "invalid installation ID",
			envVars: map[string]string{
				"GITHUB_APP_ID":              "123456",
				"GITHUB_APP_INSTALLATION_ID": "invalid",
				"GITHUB_APP_PRIVATE_KEY":     testPrivateKey,
			},
			wantNil:   true,
			wantError: true,
		},
		{
			name: "missing private key",
			envVars: map[string]string{
				"GITHUB_APP_ID":              "123456",
				"GITHUB_APP_INSTALLATION_ID": "789012",
			},
			wantNil:   true,
			wantError: true,
		},
		{
			name: "private key file not found",
			envVars: map[string]string{
				"GITHUB_APP_ID":               "123456",
				"GITHUB_APP_INSTALLATION_ID":  "789012",
				"GITHUB_APP_PRIVATE_KEY_PATH": "/nonexistent/path/key.pem",
			},
			wantNil:   true,
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clear environment
			clearGitHubAppEnv()
			defer clearGitHubAppEnv()

			// Set up environment variables
			for k, v := range tt.envVars {
				os.Setenv(k, v)
			}

			// Create temporary private key file if needed
			if tt.privateKeyFile {
				tmpDir := t.TempDir()
				keyPath := filepath.Join(tmpDir, "private-key.pem")
				if err := os.WriteFile(keyPath, []byte(testPrivateKey), 0600); err != nil {
					t.Fatalf("Failed to create temp key file: %v", err)
				}
				os.Setenv("GITHUB_APP_PRIVATE_KEY_PATH", keyPath)
			}

			config, err := LoadGitHubAppConfig()

			if tt.wantError {
				if err == nil {
					t.Error("Expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			if tt.wantNil {
				if config != nil {
					t.Error("Expected nil config but got non-nil")
				}
				return
			}

			if config == nil {
				t.Error("Expected non-nil config but got nil")
				return
			}

			// Verify config values
			if config.AppID != 123456 {
				t.Errorf("Expected AppID 123456, got %d", config.AppID)
			}
			if config.InstallationID != 789012 {
				t.Errorf("Expected InstallationID 789012, got %d", config.InstallationID)
			}
			if len(config.PrivateKey) == 0 {
				t.Error("Expected non-empty PrivateKey")
			}
		})
	}
}

func TestLoadGitHubAppConfig_PrivateKeyPrecedence(t *testing.T) {
	clearGitHubAppEnv()
	defer clearGitHubAppEnv()

	// Create temporary private key file with different content
	tmpDir := t.TempDir()
	keyPath := filepath.Join(tmpDir, "private-key.pem")
	fileKey := "file-key-content"
	if err := os.WriteFile(keyPath, []byte(fileKey), 0600); err != nil {
		t.Fatalf("Failed to create temp key file: %v", err)
	}

	// Set both inline key and file path
	inlineKey := "inline-key-content"
	os.Setenv("GITHUB_APP_ID", "123456")
	os.Setenv("GITHUB_APP_INSTALLATION_ID", "789012")
	os.Setenv("GITHUB_APP_PRIVATE_KEY", inlineKey)
	os.Setenv("GITHUB_APP_PRIVATE_KEY_PATH", keyPath)

	config, err := LoadGitHubAppConfig()
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Inline key should take precedence
	if string(config.PrivateKey) != inlineKey {
		t.Errorf("Expected inline key to take precedence, got: %s", string(config.PrivateKey))
	}
}

func TestNewGitHubAppTransport(t *testing.T) {
	tests := []struct {
		name      string
		config    *GitHubAppConfig
		baseURL   string
		wantError bool
	}{
		{
			name:      "nil config",
			config:    nil,
			wantError: true,
		},
		{
			name: "valid config without base URL",
			config: &GitHubAppConfig{
				AppID:          123456,
				InstallationID: 789012,
				PrivateKey:     []byte(testPrivateKey),
			},
			wantError: false,
		},
		{
			name: "valid config with base URL",
			config: &GitHubAppConfig{
				AppID:          123456,
				InstallationID: 789012,
				PrivateKey:     []byte(testPrivateKey),
			},
			baseURL:   "https://api.github.enterprise.com/",
			wantError: false,
		},
		{
			name: "invalid private key",
			config: &GitHubAppConfig{
				AppID:          123456,
				InstallationID: 789012,
				PrivateKey:     []byte("invalid key"),
			},
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			transport, err := NewGitHubAppTransport(tt.config, tt.baseURL)

			if tt.wantError {
				if err == nil {
					t.Error("Expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			if transport == nil {
				t.Error("Expected non-nil transport")
			}
		})
	}
}

// clearGitHubAppEnv clears all GitHub App related environment variables.
func clearGitHubAppEnv() {
	os.Unsetenv("GITHUB_APP_ID")
	os.Unsetenv("GITHUB_APP_INSTALLATION_ID")
	os.Unsetenv("GITHUB_APP_PRIVATE_KEY")
	os.Unsetenv("GITHUB_APP_PRIVATE_KEY_PATH")
}
