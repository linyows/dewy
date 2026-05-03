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
