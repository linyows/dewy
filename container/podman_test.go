package container

import (
	"errors"
	"log/slog"
	"os"
	"testing"
	"time"
)

func TestNewPodman(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	drainTime := 30 * time.Second

	rt, err := New("podman", logger, drainTime)

	// This test may fail if podman is not installed
	// In CI environments without podman, this is expected
	if err != nil {
		if errors.Is(err, ErrRuntimeNotFound) {
			t.Skip("Podman not found, skipping test")
		}
		t.Fatalf("Failed to create Podman runtime: %v", err)
	}

	if rt.cmd != "podman" {
		t.Errorf("Expected cmd to be 'podman', got %s", rt.cmd)
	}

	if rt.drainTime != drainTime {
		t.Errorf("Expected drainTime to be %v, got %v", drainTime, rt.drainTime)
	}

	if rt.logger == nil {
		t.Error("Expected logger to be set")
	}
}

// Note: Tests for actual Podman operations (Pull, Run, Stop, etc.) are not
// included in unit tests because they require a running Podman daemon.
// These will be tested in integration tests instead.
