package dewy

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/linyows/dewy/logging"
)

// newHookTestDewy builds the minimum Dewy needed to run a hook: a working
// directory for the shell command, a notifier to record the result, and a
// logger writing into buf so the failure wording can be asserted.
func newHookTestDewy(t *testing.T, buf *bytes.Buffer) (*Dewy, *mockNotify) {
	t.Helper()
	n := &mockNotify{}
	return &Dewy{
		root:     t.TempDir(),
		notifier: n,
		logger:   logging.SetupLogger("ERROR", "text", buf),
	}, n
}

// A blank hook command is a no-op: nothing runs, nothing is notified.
func TestRunDeployHook_Blank(t *testing.T) {
	var buf bytes.Buffer
	d, n := newHookTestDewy(t, &buf)

	if err := d.runDeployHook(context.Background(), beforeDeployHook, ""); err != nil {
		t.Errorf("runDeployHook() with a blank command = %v, want nil", err)
	}
	if got := n.GetHookResults(); len(got) != 0 {
		t.Errorf("hook results = %d, want 0", len(got))
	}
}

// A successful hook is notified as successful and returns no error.
func TestRunDeployHook_Success(t *testing.T) {
	var buf bytes.Buffer
	d, n := newHookTestDewy(t, &buf)

	if err := d.runDeployHook(context.Background(), beforeDeployHook, "echo hello"); err != nil {
		t.Fatalf("runDeployHook() = %v, want nil", err)
	}

	results := n.GetHookResults()
	if len(results) != 1 {
		t.Fatalf("hook results = %d, want 1", len(results))
	}
	if !results[0].Success {
		t.Error("hook result Success = false, want true")
	}
	if results[0].Stdout != "hello" {
		t.Errorf("hook result Stdout = %q, want %q", results[0].Stdout, "hello")
	}
	if buf.Len() != 0 {
		t.Errorf("a successful hook logged at ERROR: %s", buf.String())
	}
}

// A failing hook is still notified (so the operator sees the output), is
// logged with the wording the call sites used before this helper existed, and
// returns the error for the caller to act on.
func TestRunDeployHook_Failure(t *testing.T) {
	tests := []struct {
		name   string
		hook   deployHook
		logMsg string
	}{
		{"before", beforeDeployHook, "Before deploy hook failure"},
		{"after", afterDeployHook, "After deploy hook failure"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			d, n := newHookTestDewy(t, &buf)

			err := d.runDeployHook(context.Background(), tt.hook, "exit 3")
			if err == nil {
				t.Fatal("runDeployHook() = nil, want an error")
			}

			results := n.GetHookResults()
			if len(results) != 1 {
				t.Fatalf("hook results = %d, want 1", len(results))
			}
			if results[0].Success {
				t.Error("hook result Success = true, want false")
			}
			if results[0].ExitCode != 3 {
				t.Errorf("hook result ExitCode = %d, want 3", results[0].ExitCode)
			}
			if !strings.Contains(buf.String(), tt.logMsg) {
				t.Errorf("log = %q, want it to contain %q", buf.String(), tt.logMsg)
			}
		})
	}
}
