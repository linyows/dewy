package dewy

import (
	"net/url"
	"strings"
	"testing"
)

func TestNewPassesChannelToTheRegistry(t *testing.T) {
	tests := []struct {
		name     string
		registry string
		channel  string
		want     string
	}{
		{
			name:     "channel is added to a registry URL without options",
			registry: "ghr://linyows/dewy",
			channel:  "canary",
			want:     "canary",
		},
		{
			name:     "channel is added next to existing options",
			registry: "ghr://linyows/dewy?artifact=app.tar.gz",
			channel:  "canary",
			want:     "canary",
		},
		{
			name:     "no channel leaves the URL alone",
			registry: "ghr://linyows/dewy",
			channel:  "",
			want:     "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := DefaultConfig()
			c.Registry = tt.registry
			c.Channel = tt.channel

			d, err := New(c, testLogger())
			if err != nil {
				t.Fatalf("New: %v", err)
			}

			_, after, _ := strings.Cut(d.config.Registry, "://")
			u, err := url.Parse(after)
			if err != nil {
				t.Fatalf("url.Parse(%q): %v", after, err)
			}
			if got := u.Query().Get("channel"); got != tt.want {
				t.Errorf("registry URL %q carries channel %q, want %q", d.config.Registry, got, tt.want)
			}
		})
	}
}

func TestNewKeepsExistingRegistryOptionsWithChannel(t *testing.T) {
	c := DefaultConfig()
	c.Registry = "ghr://linyows/dewy?artifact=app.tar.gz"
	c.Channel = "canary"

	d, err := New(c, testLogger())
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	_, after, _ := strings.Cut(d.config.Registry, "://")
	u, err := url.Parse(after)
	if err != nil {
		t.Fatalf("url.Parse(%q): %v", after, err)
	}
	if got := u.Query().Get("artifact"); got != "app.tar.gz" {
		t.Errorf("artifact option = %q, want app.tar.gz", got)
	}
}
