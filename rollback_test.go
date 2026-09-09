package dewy

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/linyows/dewy/registry"
)

// catchHUP makes SIGHUP harmless for the duration of a test and returns a
// channel that receives it. restartServer signals the test process itself, so
// without this the default disposition would terminate the test binary.
func catchHUP(t *testing.T) chan os.Signal {
	t.Helper()
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGHUP)
	t.Cleanup(func() { signal.Stop(ch) })
	return ch
}

// newRollbackTestDewy returns a SERVER-mode Dewy rooted in a temp directory
// with two release directories staged and "current" pointing at the newer one.
func newRollbackTestDewy(t *testing.T) (d *Dewy, oldRelease, newRelease string) {
	t.Helper()
	d = newPhaseTestDewy(t)
	d.config.Command = SERVER
	d.config.Starter = &StarterConfig{ports: []string{"8000"}}
	d.notifier = &mockNotify{}

	oldRelease = filepath.Join(d.root, releasesDir, "20240101T000000Z")
	newRelease = filepath.Join(d.root, releasesDir, "20240102T000000Z")
	for _, dir := range []string{oldRelease, newRelease} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatalf("MkdirAll: %v", err)
		}
	}
	if err := d.swapSymlink(newRelease); err != nil {
		t.Fatalf("swapSymlink: %v", err)
	}
	return d, oldRelease, newRelease
}

func currentTarget(t *testing.T, d *Dewy) string {
	t.Helper()
	target, err := os.Readlink(filepath.Join(d.root, symlinkDir))
	if err != nil {
		t.Fatalf("Readlink: %v", err)
	}
	return target
}

func TestServerHealthURL(t *testing.T) {
	tests := []struct {
		name  string
		ports []string
		path  string
		want  string
	}{
		{"plain port", []string{"8000"}, "/health", "http://127.0.0.1:8000/health"},
		{"path without a leading slash", []string{"8000"}, "health", "http://127.0.0.1:8000/health"},
		{"no port", nil, "/health", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := newPhaseTestDewy(t)
			d.config.Starter = &StarterConfig{ports: tt.ports}
			d.config.Health.Path = tt.path
			if got := d.serverHealthURL(); got != tt.want {
				t.Errorf("serverHealthURL() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestVerifyServerHealth(t *testing.T) {
	t.Run("no-op without a health path", func(t *testing.T) {
		d := newPhaseTestDewy(t)
		d.config.Command = SERVER
		d.config.Starter = &StarterConfig{ports: []string{"8000"}}
		if err := d.verifyServerHealth(context.Background()); err != nil {
			t.Errorf("verifyServerHealth() = %v, want nil", err)
		}
	})

	t.Run("no-op in assets mode", func(t *testing.T) {
		d := newPhaseTestDewy(t)
		d.config.Command = ASSETS
		d.config.Health.Path = "/health"
		if err := d.verifyServerHealth(context.Background()); err != nil {
			t.Errorf("verifyServerHealth() = %v, want nil", err)
		}
	})

	t.Run("passes when the server answers", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/health" {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			w.WriteHeader(http.StatusOK)
		}))
		defer srv.Close()

		d := newPhaseTestDewy(t)
		d.config.Command = SERVER
		d.config.Starter = &StarterConfig{ports: []string{portOf(t, srv.URL)}}
		d.config.Health.Path = "/health"
		if err := d.verifyServerHealth(context.Background()); err != nil {
			t.Errorf("verifyServerHealth() = %v, want nil", err)
		}
	})

	t.Run("fails on an error status", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusServiceUnavailable)
		}))
		defer srv.Close()

		d := newPhaseTestDewy(t)
		d.config.Command = SERVER
		d.config.Starter = &StarterConfig{ports: []string{portOf(t, srv.URL)}}
		d.config.Health.Path = "/health"
		d.config.Health.Timeout = 100 * time.Millisecond
		if err := d.verifyServerHealth(context.Background()); err == nil {
			t.Error("verifyServerHealth() = nil, want an error for a 503 response")
		}
	})
}

// portOf extracts the port from an httptest server URL.
func portOf(t *testing.T, rawURL string) string {
	t.Helper()
	_, port, ok := strings.Cut(strings.TrimPrefix(rawURL, "http://"), ":")
	if !ok {
		t.Fatalf("cannot find a port in %q", rawURL)
	}
	return port
}

func TestRollbackServer_RestoresPreviousRelease(t *testing.T) {
	hup := catchHUP(t)
	d, oldRelease, newRelease := newRollbackTestDewy(t)
	// A server that is up is restarted onto the restored release.
	d.isServerRunning = true

	st := cacheState{
		key:         "v2.0.0--app.tar.gz",
		prevKey:     "v1.0.0--app.tar.gz",
		prevRelease: oldRelease,
	}
	res := &registry.CurrentResponse{ID: "id", Tag: "v2.0.0"}

	d.rollbackServer(context.Background(), res, st, errors.New("unhealthy status 503"))

	if got := currentTarget(t, d); got != oldRelease {
		t.Errorf("current points at %q, want the previous release %q (new was %q)", got, oldRelease, newRelease)
	}
	if got := d.blockedVersion(); got != st.key {
		t.Errorf("blockedVersion() = %q, want %q", got, st.key)
	}
	current, err := d.cache.Read(currentkeyName)
	if err != nil {
		t.Fatalf("cache.Read(current): %v", err)
	}
	if string(current) != st.prevKey {
		t.Errorf("current cache key = %q, want %q", current, st.prevKey)
	}
	if d.cVer != "v1.0.0" {
		t.Errorf("cVer = %q, want v1.0.0", d.cVer)
	}

	select {
	case <-hup:
	case <-time.After(time.Second):
		t.Error("rollback did not restart the managed server")
	}

	notify, ok := d.notifier.(*mockNotify)
	if !ok {
		t.Fatalf("notifier is %T, want *mockNotify", d.notifier)
	}
	msgs := notify.messages
	if len(msgs) != 1 || !strings.Contains(msgs[0], "Rolled back to `v1.0.0`") {
		t.Errorf("notifications = %v, want one message reporting the rollback", msgs)
	}
}

func TestRollbackServer_NoPreviousRelease(t *testing.T) {
	d, _, newRelease := newRollbackTestDewy(t)

	st := cacheState{key: "v1.0.0--app.tar.gz"}
	res := &registry.CurrentResponse{ID: "id", Tag: "v1.0.0"}

	d.rollbackServer(context.Background(), res, st, errors.New("unhealthy status 503"))

	if got := currentTarget(t, d); got != newRelease {
		t.Errorf("current points at %q, want it left at %q", got, newRelease)
	}
	if got := d.blockedVersion(); got != st.key {
		t.Errorf("blockedVersion() = %q, want the failed version to be recorded", got)
	}
	notify, ok := d.notifier.(*mockNotify)
	if !ok {
		t.Fatalf("notifier is %T, want *mockNotify", d.notifier)
	}
	msgs := notify.messages
	if len(msgs) != 1 || !strings.Contains(msgs[0], "no previous release") {
		t.Errorf("notifications = %v, want one message about the missing previous release", msgs)
	}
}

func TestRollbackServer_NoRollbackFlag(t *testing.T) {
	d, _, newRelease := newRollbackTestDewy(t)
	d.config.Health.NoRollback = true

	st := cacheState{
		key:         "v2.0.0--app.tar.gz",
		prevKey:     "v1.0.0--app.tar.gz",
		prevRelease: filepath.Join(d.root, releasesDir, "20240101T000000Z"),
	}
	res := &registry.CurrentResponse{ID: "id", Tag: "v2.0.0"}

	d.rollbackServer(context.Background(), res, st, errors.New("unhealthy status 503"))

	if got := currentTarget(t, d); got != newRelease {
		t.Errorf("current points at %q, want the failed release left in place at %q", got, newRelease)
	}
	if got := d.blockedVersion(); got != st.key {
		t.Errorf("blockedVersion() = %q, want the failed version to be recorded even with --no-rollback", got)
	}
	notify, ok := d.notifier.(*mockNotify)
	if !ok {
		t.Fatalf("notifier is %T, want *mockNotify", d.notifier)
	}
	msgs := notify.messages
	if len(msgs) != 1 || !strings.Contains(msgs[0], "Rollback is disabled") {
		t.Errorf("notifications = %v, want one message stating that rollback is disabled", msgs)
	}
}

func TestResolveCacheState_SkipsBlockedVersion(t *testing.T) {
	d := newPhaseTestDewy(t)
	d.config.Command = SERVER
	d.config.Health.Path = "/health"

	res := &registry.CurrentResponse{ID: "id", Tag: "v2.0.0", ArtifactURL: "ghr://linyows/dewy/tag/v2.0.0/app.tar.gz"}
	key := d.cachekeyName(res)
	if err := d.cache.Write(blockedkeyName, []byte(key)); err != nil {
		t.Fatalf("cache.Write: %v", err)
	}

	st, err := d.resolveCacheState(context.Background(), res)
	if err != nil {
		t.Fatalf("resolveCacheState: %v", err)
	}
	if !st.skip {
		t.Error("a version that failed its health check should be skipped, not redeployed")
	}
}

func TestResolveCacheState_DoesNotSkipOtherVersions(t *testing.T) {
	d := newPhaseTestDewy(t)
	d.config.Command = SERVER
	d.config.Health.Path = "/health"

	if err := d.cache.Write(blockedkeyName, []byte("v2.0.0--app.tar.gz")); err != nil {
		t.Fatalf("cache.Write: %v", err)
	}

	res := &registry.CurrentResponse{ID: "id", Tag: "v2.0.1", ArtifactURL: "ghr://linyows/dewy/tag/v2.0.1/app.tar.gz"}
	st, err := d.resolveCacheState(context.Background(), res)
	if err != nil {
		t.Fatalf("resolveCacheState: %v", err)
	}
	if st.skip {
		t.Error("a version other than the blocked one should still deploy")
	}
}

func TestClearBlockedVersion(t *testing.T) {
	d := newPhaseTestDewy(t)
	if err := d.cache.Write(blockedkeyName, []byte("v2.0.0--app.tar.gz")); err != nil {
		t.Fatalf("cache.Write: %v", err)
	}

	d.clearBlockedVersion()

	if got := d.blockedVersion(); got != "" {
		t.Errorf("blockedVersion() = %q, want it cleared", got)
	}
	// Clearing again is a no-op rather than an error.
	d.clearBlockedVersion()
}

func TestTagFromCacheKey(t *testing.T) {
	tests := []struct {
		key  string
		want string
	}{
		{"v1.2.3--app_linux_amd64.tar.gz", "v1.2.3"},
		{"2024.01.1--app.zip", "2024.01.1"},
		{"v1.2.3", "v1.2.3"},
		{"", ""},
	}
	for _, tt := range tests {
		if got := tagFromCacheKey(tt.key); got != tt.want {
			t.Errorf("tagFromCacheKey(%q) = %q, want %q", tt.key, got, tt.want)
		}
	}
}

func TestServerHealthURLUsesTheLowestPort(t *testing.T) {
	// The probe port comes from the parsed port list, which is deduplicated
	// and sorted numerically, so it is the lowest port rather than the one
	// given first.
	ports, err := parsePorts([]string{"9000", "8080"})
	if err != nil {
		t.Fatalf("parsePorts: %v", err)
	}

	d := newPhaseTestDewy(t)
	d.config.Starter = &StarterConfig{ports: ports}
	d.config.Health.Path = "/health"

	if got, want := d.serverHealthURL(), "http://127.0.0.1:8080/health"; got != want {
		t.Errorf("serverHealthURL() = %q, want %q", got, want)
	}
}

func TestRollbackServer_StartsAServerThatNeverCameUp(t *testing.T) {
	hup := catchHUP(t)
	d, oldRelease, _ := newRollbackTestDewy(t)
	// A deploy can fail before the server exists: the starter could not be
	// created, so nothing is running and there is nothing to send SIGHUP to.
	d.isServerRunning = false

	st := cacheState{
		key:         "v2.0.0--app.tar.gz",
		prevKey:     "v1.0.0--app.tar.gz",
		prevRelease: oldRelease,
	}
	res := &registry.CurrentResponse{ID: "id", Tag: "v2.0.0"}

	d.rollbackServer(context.Background(), res, st, errors.New("starter failure"))

	if got := currentTarget(t, d); got != oldRelease {
		t.Errorf("current points at %q, want the previous release %q", got, oldRelease)
	}
	if got := d.blockedVersion(); got != st.key {
		t.Errorf("blockedVersion() = %q, want %q", got, st.key)
	}

	// The test config has no command, so starting cannot succeed. What matters
	// is that the rollback tried to start rather than signaling a process
	// that is not there, and that it says so instead of reporting success.
	select {
	case <-hup:
		t.Error("rollback signaled a restart although no server was running")
	case <-time.After(200 * time.Millisecond):
	}

	notify, ok := d.notifier.(*mockNotify)
	if !ok {
		t.Fatalf("notifier is %T, want *mockNotify", d.notifier)
	}
	msgs := notify.messages
	if len(msgs) != 1 || !strings.Contains(msgs[0], "bringing the server up failed") {
		t.Errorf("notifications = %v, want one message reporting that the server could not be started", msgs)
	}
}

func TestResolveCacheState_IgnoresBlockedWithoutHealthPath(t *testing.T) {
	d := newPhaseTestDewy(t)
	d.config.Command = SERVER

	res := &registry.CurrentResponse{ID: "id", Tag: "v2.0.0", ArtifactURL: "ghr://linyows/dewy/tag/v2.0.0/app.tar.gz"}
	if err := d.cache.Write(blockedkeyName, []byte(d.cachekeyName(res))); err != nil {
		t.Fatalf("cache.Write: %v", err)
	}

	st, err := d.resolveCacheState(context.Background(), res)
	if err != nil {
		t.Fatalf("resolveCacheState: %v", err)
	}
	if st.skip {
		t.Error("without --health-path the marker must not be consulted, so dropping the option releases the version")
	}
}

func TestRecoverBlockedServer(t *testing.T) {
	t.Run("no-op in assets mode", func(t *testing.T) {
		d := newPhaseTestDewy(t)
		d.config.Command = ASSETS
		d.notifier = &mockNotify{}

		d.recoverBlockedServer(context.Background(), "v1.0.0--app.tar.gz")

		notify, _ := d.notifier.(*mockNotify)
		if len(notify.messages) != 0 {
			t.Errorf("notifications = %v, want none in assets mode", notify.messages)
		}
	})

	t.Run("no-op while the server is running", func(t *testing.T) {
		d, _, _ := newRollbackTestDewy(t)
		d.isServerRunning = true

		d.recoverBlockedServer(context.Background(), "v1.0.0--app.tar.gz")

		notify, _ := d.notifier.(*mockNotify)
		if len(notify.messages) != 0 {
			t.Errorf("notifications = %v, want none while the server is up", notify.messages)
		}
	})

	t.Run("starts a stopped server and adopts its version", func(t *testing.T) {
		d, _, _ := newRollbackTestDewy(t)
		d.isServerRunning = false

		d.recoverBlockedServer(context.Background(), "v1.0.0--app.tar.gz")

		if d.cVer != "v1.0.0" {
			t.Errorf("cVer = %q, want the running version v1.0.0", d.cVer)
		}
		// The test config has no command, so the start fails and is reported
		// as an error rather than as a successful recovery.
		notify, _ := d.notifier.(*mockNotify)
		if notify.errorCount == 0 {
			t.Error("a failed recovery must be notified as an error")
		}
	})
}

func TestStarterStatus(t *testing.T) {
	dir := t.TempDir()

	tests := []struct {
		name    string
		content string
		want    []int
	}{
		{"one worker", "3:1234\n", []int{3}},
		{"a swap in progress", "2:1200\n3:1234\n", []int{2, 3}},
		{"blank lines are ignored", "\n3:1234\n\n", []int{3}},
		{"a half-written line is ignored", "3", nil},
		{"a non-numeric generation is ignored", "old:1234\n", nil},
		{"empty file", "", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(dir, "status")
			if err := os.WriteFile(path, []byte(tt.content), 0600); err != nil {
				t.Fatalf("WriteFile: %v", err)
			}
			got := starterStatus(path)
			if len(got) != len(tt.want) {
				t.Fatalf("starterStatus() = %v, want %v", got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Fatalf("starterStatus() = %v, want %v", got, tt.want)
				}
			}
		})
	}

	t.Run("missing file", func(t *testing.T) {
		if got := starterStatus(filepath.Join(dir, "absent")); got != nil {
			t.Errorf("starterStatus() = %v, want nil for a missing file", got)
		}
	})

	t.Run("unconfigured path", func(t *testing.T) {
		if got := starterStatus(""); got != nil {
			t.Errorf("starterStatus() = %v, want nil when no status file is configured", got)
		}
	})
}

func TestAwaitWorkerSwap(t *testing.T) {
	newDewy := func(t *testing.T, content string) (*Dewy, string) {
		t.Helper()
		d := newPhaseTestDewy(t)
		path := filepath.Join(t.TempDir(), "status")
		if content != "" {
			if err := os.WriteFile(path, []byte(content), 0600); err != nil {
				t.Fatalf("WriteFile: %v", err)
			}
		}
		d.config.Starter = &StarterConfig{ports: []string{"8000"}, statusfile: path}
		return d, path
	}

	t.Run("the new worker is alone", func(t *testing.T) {
		d, _ := newDewy(t, "4:2000\n")
		if !d.awaitWorkerSwap(context.Background(), 3, time.Second) {
			t.Error("awaitWorkerSwap() = false, want true once only the new generation is left")
		}
	})

	t.Run("waits for the old worker to go", func(t *testing.T) {
		d, path := newDewy(t, "3:1000\n4:2000\n")
		go func() {
			time.Sleep(100 * time.Millisecond)
			_ = os.WriteFile(path, []byte("4:2000\n"), 0600)
		}()
		if !d.awaitWorkerSwap(context.Background(), 3, 3*time.Second) {
			t.Error("awaitWorkerSwap() = false, want true after the old worker exits")
		}
	})

	t.Run("gives up when the old worker stays", func(t *testing.T) {
		d, _ := newDewy(t, "3:1000\n4:2000\n")
		if d.awaitWorkerSwap(context.Background(), 3, 300*time.Millisecond) {
			t.Error("awaitWorkerSwap() = true, want false while the previous worker is still alive")
		}
	})

	t.Run("gives up when the generation did not advance", func(t *testing.T) {
		d, _ := newDewy(t, "3:1000\n")
		if d.awaitWorkerSwap(context.Background(), 3, 300*time.Millisecond) {
			t.Error("awaitWorkerSwap() = true, want false when the worker was not replaced")
		}
	})

	t.Run("no status file configured", func(t *testing.T) {
		d := newPhaseTestDewy(t)
		d.config.Starter = &StarterConfig{ports: []string{"8000"}}
		if d.awaitWorkerSwap(context.Background(), 3, time.Second) {
			t.Error("awaitWorkerSwap() = true, want false when no status file is configured")
		}
	})
}

func TestLatestGeneration(t *testing.T) {
	d := newPhaseTestDewy(t)
	d.config.Starter = &StarterConfig{ports: []string{"8000"}}
	if got := d.latestGeneration(); got != -1 {
		t.Errorf("latestGeneration() = %d, want -1 without a status file", got)
	}

	path := filepath.Join(t.TempDir(), "status")
	if err := os.WriteFile(path, []byte("2:1200\n5:1500\n"), 0600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	d.config.Starter = &StarterConfig{ports: []string{"8000"}, statusfile: path}
	if got := d.latestGeneration(); got != 5 {
		t.Errorf("latestGeneration() = %d, want 5", got)
	}
}
