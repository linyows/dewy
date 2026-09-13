package dewy

import "testing"

// appName must prefer Config.Container.Name when set. The label filter that
// admin API and stopManagedContainers use must agree with the deploy path.
func TestAppName_ExplicitName(t *testing.T) {
	d := &Dewy{config: Config{
		Container: &ContainerConfig{Name: "myapp"},
		Registry:  "img://ghcr.io/owner/derived-name",
	}}
	if got := d.appName(); got != "myapp" {
		t.Errorf("appName() = %q, want myapp", got)
	}
}

// When Container.Name is empty, fall back to the registry repository segment.
// This is what the deploy path was already doing (via imageRef parsing); the
// admin API used to read Config.Container.Name directly and report empty.
func TestAppName_DerivedFromRegistry(t *testing.T) {
	tests := []struct {
		name     string
		registry string
		want     string
	}{
		{"OCI ghcr", "img://ghcr.io/owner/myrepo", "myrepo"},
		{"OCI with query", "img://ghcr.io/owner/myrepo?pre-release=true", "myrepo"},
		{"OCI with tag", "img://ghcr.io/owner/myrepo:latest", "myrepo"},
		{"GHR", "ghr://owner/myrepo", "myrepo"},
		{"GHR with query", "ghr://owner/myrepo?pre-release=true", "myrepo"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := &Dewy{config: Config{
				Container: &ContainerConfig{Name: ""},
				Registry:  tt.registry,
			}}
			if got := d.appName(); got != tt.want {
				t.Errorf("appName() = %q, want %q", got, tt.want)
			}
		})
	}
}

// deriveAppNameFromRegistry is the single implementation behind both the
// Dewy.appName fallback and the CLI's --name default. This table came from
// TestExtractAppNameFromRegistry in cli_test.go, which covered the CLI's own
// per-scheme copy of this logic; the cases marked below are the ones that
// copy got wrong.
func TestDeriveAppNameFromRegistry(t *testing.T) {
	tests := []struct {
		name     string
		registry string
		want     string
	}{
		// img:// (OCI registry) - container command
		{"img - ghcr.io with tag", "img://ghcr.io/owner/myapp:latest", "myapp"},
		{"img - ghcr.io without tag", "img://ghcr.io/owner/myapp", "myapp"},
		{"img - docker.io library", "img://docker.io/library/nginx:1.21", "nginx"},
		{"img - gcr.io", "img://gcr.io/my-project/myapp", "myapp"},
		{"img - simple image name", "img://myapp:latest", "myapp"},
		{"img - with query parameters", "img://ghcr.io/owner/myapp?pre-release=true", "myapp"},
		{"img - with tag and query parameters", "img://ghcr.io/owner/myapp:v1.0.0?pre-release=true", "myapp"},

		// ghr:// (GitHub Releases). The CLI copy split the path before
		// stripping the query, so it returned "myrepo?pre-release=true".
		{"ghr - owner/repo", "ghr://owner/myrepo", "myrepo"},
		{"ghr - with query parameters", "ghr://owner/myrepo?pre-release=true", "myrepo"},

		// s3://. Same query bug as ghr, and the CLI copy handled a trailing
		// slash while this one did not.
		{"s3 - simple path", "s3://us-east-1/bucket/myapp", "myapp"},
		{"s3 - nested path", "s3://us-east-1/bucket/path/to/myapp", "myapp"},
		{"s3 - with query parameters", "s3://us-east-1/bucket/myapp?pre-release=true", "myapp"},
		{"s3 - trailing slash", "s3://us-east-1/bucket/myapp/", "myapp"},

		// gs:// was not handled by the CLI copy at all, which fell through
		// to the literal "app".
		{"gs - simple path", "gs://bucket/myapp", "myapp"},

		// Invalid cases
		{"invalid format - no scheme", "invalid-url", ""},
		{"empty string", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := deriveAppNameFromRegistry(tt.registry); got != tt.want {
				t.Errorf("deriveAppNameFromRegistry(%q) = %q, want %q", tt.registry, got, tt.want)
			}
		})
	}
}

// nil Container config (server/assets paths can have it) must not panic; the
// fallback is the registry-derived name.
func TestAppName_NilContainerConfig(t *testing.T) {
	d := &Dewy{config: Config{
		Container: nil,
		Registry:  "ghr://owner/myrepo",
	}}
	if got := d.appName(); got != "myrepo" {
		t.Errorf("appName() = %q, want myrepo", got)
	}
}
