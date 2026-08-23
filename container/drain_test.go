package container

import (
	"context"
	"slices"
	"testing"
	"time"

	"github.com/linyows/dewy/internal/sysdeps/fake"
)

func TestDrainTimeout(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		drainTime time.Duration
		want      time.Duration
	}{
		{
			name:      "honors configured drain time",
			drainTime: 60 * time.Second,
			want:      60 * time.Second,
		},
		{
			name:      "falls back when unset",
			drainTime: 0,
			want:      defaultStopTimeoutOld,
		},
		{
			name:      "falls back when negative",
			drainTime: -1 * time.Second,
			want:      defaultStopTimeoutOld,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			rt := &Runtime{drainTime: tt.drainTime}
			if got := rt.drainTimeout(); got != tt.want {
				t.Errorf("drainTimeout() = %v, want %v", got, tt.want)
			}
		})
	}
}

// stopTimeArg returns the --time= value the fake saw on the last stop call.
func stopTimeArg(t *testing.T, runner *fake.CommandRunner) string {
	t.Helper()

	for _, call := range slices.Backward(runner.Calls()) {
		if len(call.Args) == 0 || call.Args[0] != "stop" {
			continue
		}
		for _, arg := range call.Args {
			if len(arg) > 7 && arg[:7] == "--time=" {
				return arg[7:]
			}
		}
		t.Fatalf("stop call has no --time argument: %v", call.Args)
	}
	t.Fatal("no stop call recorded")
	return ""
}

func TestStop_PassesConfiguredDrainTime(t *testing.T) {
	t.Parallel()

	runner := fake.NewCommandRunner().SetPath("docker", "/usr/bin/docker")
	rt, err := New("docker", testLogger(), 45*time.Second, WithCommandRunner(runner))
	if err != nil {
		t.Fatalf("New with fake runner: %v", err)
	}
	runner.SetOutput("docker", nil)

	if err := rt.Stop(context.Background(), "abc123", rt.drainTimeout()); err != nil {
		t.Fatalf("Stop: %v", err)
	}
	if got := stopTimeArg(t, runner); got != "45" {
		t.Errorf("stop --time = %s, want 45", got)
	}
}

func TestStop_RoundsFractionalTimeoutUp(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		timeout time.Duration
		want    string
	}{
		{
			// Truncating would yield 0, which the CLI treats as an immediate
			// KILL rather than a graceful stop.
			name:    "sub-second keeps a grace period",
			timeout: 500 * time.Millisecond,
			want:    "1",
		},
		{
			// Truncating would silently shorten the caller's grace period.
			name:    "fractional above a second is not shortened",
			timeout: 1500 * time.Millisecond,
			want:    "2",
		},
		{
			name:    "whole seconds pass through",
			timeout: 5 * time.Second,
			want:    "5",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			rt, runner := newFakeRuntime(t)
			runner.SetOutput("docker", nil)

			if err := rt.Stop(context.Background(), "abc123", tt.timeout); err != nil {
				t.Fatalf("Stop: %v", err)
			}
			if got := stopTimeArg(t, runner); got != tt.want {
				t.Errorf("stop --time = %s, want %s", got, tt.want)
			}
		})
	}
}

func TestStop_ZeroTimeoutStaysImmediate(t *testing.T) {
	t.Parallel()

	rt, runner := newFakeRuntime(t)
	runner.SetOutput("docker", nil)

	// An explicit zero still means "kill now" - only sub-second positive
	// values are rounded up.
	if err := rt.Stop(context.Background(), "abc123", 0); err != nil {
		t.Fatalf("Stop: %v", err)
	}
	if got := stopTimeArg(t, runner); got != "0" {
		t.Errorf("stop --time = %s, want 0", got)
	}
}
