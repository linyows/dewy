package cache

import (
	"context"
	"strings"
	"testing"
)

func TestNewSchemeDispatch(t *testing.T) {
	tests := []struct {
		desc      string
		url       string
		expectErr string // substring; "" means expect success
	}{
		{"empty defaults to file", "", ""},
		{"file scheme", "file:///tmp/dewy-cache-test", ""},
		{"consul unsupported", "consul://localhost:8500", "unsupported cache scheme"},
		{"redis unsupported", "redis://localhost:6379", "unsupported cache scheme"},
		{"memory unsupported", "memory://", "unsupported cache scheme"},
		{"unknown scheme", "ftp://example.com", "unsupported cache scheme"},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			c, err := New(context.Background(), tt.url, nil)
			if tt.expectErr == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if c == nil {
					t.Fatal("expected non-nil cache")
				}
				return
			}
			if err == nil {
				t.Fatalf("expected error containing %q, got nil", tt.expectErr)
			}
			if !strings.Contains(err.Error(), tt.expectErr) {
				t.Errorf("error %q does not contain %q", err.Error(), tt.expectErr)
			}
		})
	}
}
