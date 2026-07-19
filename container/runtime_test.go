package container

import (
	"context"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/linyows/dewy/internal/sysdeps/fake"
)

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
}

// newFakeRuntime builds a docker Runtime whose subprocess calls are served by
// the returned fake, bypassing any real container CLI.
func newFakeRuntime(t *testing.T) (*Runtime, *fake.CommandRunner) {
	t.Helper()
	runner := fake.NewCommandRunner().SetPath("docker", "/usr/bin/docker")
	rt, err := New("docker", testLogger(), 30*time.Second, WithCommandRunner(runner))
	if err != nil {
		t.Fatalf("New with fake runner: %v", err)
	}
	return rt, runner
}

func TestWithCommandRunner_LookPathFailure(t *testing.T) {
	// A fake with no registered path makes LookPath fail, so construction
	// must surface ErrRuntimeNotFound without touching the host.
	runner := fake.NewCommandRunner()
	_, err := New("docker", testLogger(), time.Second, WithCommandRunner(runner))
	if err == nil {
		t.Fatal("expected error when runtime binary is absent")
	}
}

func TestExecCommandOutputRoutesThroughRunner(t *testing.T) {
	rt, runner := newFakeRuntime(t)
	runner.SetOutput("docker", []byte("  container-id\n"))

	out, err := rt.execCommandOutput(context.Background(), "ps", "-q")
	if err != nil {
		t.Fatalf("execCommandOutput: %v", err)
	}
	if out != "container-id" {
		t.Errorf("output = %q, want trimmed %q", out, "container-id")
	}

	calls := runner.Calls()
	if len(calls) != 1 || calls[0].Name != "docker" {
		t.Fatalf("expected one docker call, got %+v", calls)
	}
	if len(calls[0].Args) != 2 || calls[0].Args[0] != "ps" || calls[0].Args[1] != "-q" {
		t.Errorf("args = %v, want [ps -q]", calls[0].Args)
	}
}

// dockerInspectJSON is a trimmed but representative `docker inspect` array for
// two containers: one running, one OOM-killed and exited with a restart.
const dockerInspectJSON = `[
  {
    "Id": "aaa111",
    "Name": "/app-1700000000-0",
    "RestartCount": 0,
    "State": {"Status": "running", "StartedAt": "2026-07-16T10:00:00.5Z", "FinishedAt": "0001-01-01T00:00:00Z", "ExitCode": 0, "OOMKilled": false},
    "Config": {"Image": "ghcr.io/acme/app:v1.2.3", "Labels": {"dewy.managed": "true", "dewy.app": "app", "dewy.replica": "0", "dewy.version": "v1.2.3"}}
  },
  {
    "Id": "bbb222",
    "Name": "/app-1700000000-1",
    "RestartCount": 3,
    "State": {"Status": "exited", "StartedAt": "2026-07-16T10:00:00Z", "FinishedAt": "2026-07-16T10:05:00Z", "ExitCode": 137, "OOMKilled": true},
    "Config": {"Image": "ghcr.io/acme/app:v1.2.3", "Labels": {"dewy.managed": "true", "dewy.app": "app", "dewy.replica": "1", "dewy.version": "v1.2.3"}}
  }
]`

func TestInspectManaged(t *testing.T) {
	rt, runner := newFakeRuntime(t)
	runner.SetOutputFunc("docker", func(args []string) ([]byte, error) {
		switch args[0] {
		case "ps":
			return []byte("aaa111\nbbb222\n"), nil
		case "inspect":
			return []byte(dockerInspectJSON), nil
		}
		return nil, nil
	})

	statuses, err := rt.InspectManaged(context.Background(), "app")
	if err != nil {
		t.Fatalf("InspectManaged: %v", err)
	}
	if len(statuses) != 2 {
		t.Fatalf("got %d statuses, want 2", len(statuses))
	}

	// ps must list stopped containers too (-a) and filter on the managed
	// and app labels.
	var psArgs []string
	for _, c := range runner.Calls() {
		if len(c.Args) > 0 && c.Args[0] == "ps" {
			psArgs = c.Args
		}
	}
	if !contains(psArgs, "-a") {
		t.Errorf("ps args %v missing -a (stopped containers would be invisible)", psArgs)
	}
	if !contains(psArgs, "label=dewy.managed=true") || !contains(psArgs, "label=dewy.app=app") {
		t.Errorf("ps args %v missing expected label filters", psArgs)
	}

	// inspect must be a single batched call carrying both IDs.
	inspectCalls := 0
	for _, c := range runner.Calls() {
		if len(c.Args) > 0 && c.Args[0] == "inspect" {
			inspectCalls++
			if !contains(c.Args, "aaa111") || !contains(c.Args, "bbb222") {
				t.Errorf("inspect args %v should batch both container IDs", c.Args)
			}
		}
	}
	if inspectCalls != 1 {
		t.Errorf("expected exactly 1 batched inspect call, got %d", inspectCalls)
	}

	running := statuses[0]
	if running.State != "running" || running.Restarts != 0 || running.Terminated() {
		t.Errorf("running status parsed wrong: %+v", running)
	}
	if running.Replica != "0" || running.Version != "v1.2.3" {
		t.Errorf("running labels parsed wrong: replica=%q version=%q", running.Replica, running.Version)
	}
	want := time.Date(2026, 7, 16, 10, 0, 0, 500_000_000, time.UTC)
	if !running.StartedAt.Equal(want) {
		t.Errorf("StartedAt = %v, want %v", running.StartedAt, want)
	}

	crashed := statuses[1]
	if !crashed.Terminated() {
		t.Error("exited container should report Terminated()")
	}
	if crashed.Restarts != 3 || crashed.ExitCode != 137 || !crashed.OOMKilled {
		t.Errorf("crashed status parsed wrong: restarts=%d exit=%d oom=%v",
			crashed.Restarts, crashed.ExitCode, crashed.OOMKilled)
	}
}

func TestInspectManagedEmpty(t *testing.T) {
	rt, runner := newFakeRuntime(t)
	runner.SetOutputFunc("docker", func(args []string) ([]byte, error) {
		return []byte(""), nil // no containers
	})

	statuses, err := rt.InspectManaged(context.Background(), "app")
	if err != nil {
		t.Fatalf("InspectManaged: %v", err)
	}
	if len(statuses) != 0 {
		t.Errorf("got %d statuses, want 0", len(statuses))
	}
	// With no IDs, inspect must not run.
	for _, c := range runner.Calls() {
		if len(c.Args) > 0 && c.Args[0] == "inspect" {
			t.Error("inspect should not be called when no containers match")
		}
	}
}

// TestFindContainersByLabelStaysRunningOnly is a regression guard: the deploy
// path relies on FindContainersByLabel returning running-only IDs. If it ever
// starts passing -a, old stopped containers would be torn down.
func TestFindContainersByLabelStaysRunningOnly(t *testing.T) {
	rt, runner := newFakeRuntime(t)
	runner.SetOutput("docker", []byte("ccc333\n"))

	if _, err := rt.FindContainersByLabel(context.Background(), map[string]string{"dewy.app": "app"}); err != nil {
		t.Fatalf("FindContainersByLabel: %v", err)
	}
	calls := runner.Calls()
	if len(calls) != 1 {
		t.Fatalf("expected 1 call, got %d", len(calls))
	}
	if contains(calls[0].Args, "-a") {
		t.Errorf("FindContainersByLabel must not pass -a; args=%v", calls[0].Args)
	}
}

func TestRemoveExited(t *testing.T) {
	rt, runner := newFakeRuntime(t)
	var rmArgs [][]string
	runner.SetOutputFunc("docker", func(args []string) ([]byte, error) {
		switch args[0] {
		case "ps":
			return []byte("dead1\nexited2\n"), nil
		case "rm":
			rmArgs = append(rmArgs, args)
			return []byte(""), nil
		}
		return nil, nil
	})

	n, err := rt.RemoveExited(context.Background(), "app")
	if err != nil {
		t.Fatalf("RemoveExited: %v", err)
	}
	if n != 2 {
		t.Errorf("removed = %d, want 2", n)
	}

	// The ps filter must scope to stopped, managed, app-matched containers.
	var psArgs []string
	for _, c := range runner.Calls() {
		if len(c.Args) > 0 && c.Args[0] == "ps" {
			psArgs = c.Args
		}
	}
	for _, want := range []string{"-aq", "status=exited", "label=dewy.managed=true", "label=dewy.app=app"} {
		if !contains(psArgs, want) {
			t.Errorf("ps args %v missing %q", psArgs, want)
		}
	}
	// podman rejects status=dead, so it must not be passed.
	if contains(psArgs, "status=dead") {
		t.Errorf("ps args %v must not include status=dead (podman rejects it)", psArgs)
	}
	if len(rmArgs) != 2 {
		t.Errorf("expected 2 rm calls, got %d", len(rmArgs))
	}
}

// TestRemoveExitedEmptyAppScopes guards that an empty app name still scopes by
// dewy.app (matching the deploy path) rather than reaping every managed app's
// exited containers on a shared runtime.
func TestRemoveExitedEmptyAppScopes(t *testing.T) {
	rt, runner := newFakeRuntime(t)
	runner.SetOutputFunc("docker", func([]string) ([]byte, error) { return []byte(""), nil })

	if _, err := rt.RemoveExited(context.Background(), ""); err != nil {
		t.Fatalf("RemoveExited: %v", err)
	}
	var psArgs []string
	for _, c := range runner.Calls() {
		if len(c.Args) > 0 && c.Args[0] == "ps" {
			psArgs = c.Args
		}
	}
	if !contains(psArgs, "label=dewy.app=") {
		t.Errorf("ps args %v must scope by dewy.app even when app is empty", psArgs)
	}
}

func TestRemoveExitedNone(t *testing.T) {
	rt, runner := newFakeRuntime(t)
	runner.SetOutputFunc("docker", func(args []string) ([]byte, error) { return []byte(""), nil })

	n, err := rt.RemoveExited(context.Background(), "app")
	if err != nil {
		t.Fatalf("RemoveExited: %v", err)
	}
	if n != 0 {
		t.Errorf("removed = %d, want 0", n)
	}
	for _, c := range runner.Calls() {
		if len(c.Args) > 0 && c.Args[0] == "rm" {
			t.Error("rm should not be called when no exited containers match")
		}
	}
}

func contains(s []string, v string) bool {
	for _, x := range s {
		if x == v {
			return true
		}
	}
	return false
}
