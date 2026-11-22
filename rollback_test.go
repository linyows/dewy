package dewy

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/linyows/dewy/registry"
)

// fakeServer stands in for the process supervisor and records the calls dewy
// makes.
type fakeServer struct {
	mu         sync.Mutex
	restarts   int
	shutdowns  int
	restartErr error
	retired    chan struct{} // WaitRetired blocks until closed; nil returns at once
}

func (f *fakeServer) Restart(context.Context) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.restarts++
	return f.restartErr
}

func (f *fakeServer) WaitRetired(ctx context.Context) error {
	if f.retired == nil {
		return nil
	}
	select {
	case <-f.retired:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (f *fakeServer) Shutdown(context.Context) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.shutdowns++
	return nil
}

func (f *fakeServer) restartCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.restarts
}

// newRollbackTestDewy returns a SERVER-mode Dewy rooted in a temp directory
// with two release directories staged and "current" pointing at the newer one.
func newRollbackTestDewy(t *testing.T) (d *Dewy, oldRelease, newRelease string) {
	t.Helper()
	d = newPhaseTestDewy(t)
	d.config.Command = SERVER
	d.config.Server = &ServerConfig{Addrs: []string{"8000"}}
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
		{"specific host", []string{"192.168.10.11:8000"}, "/health", "http://192.168.10.11:8000/health"},
		{"loopback host", []string{"127.0.0.1:8000"}, "/health", "http://127.0.0.1:8000/health"},
		{"unspecified host probes the loopback", []string{"0.0.0.0:8000"}, "/health", "http://127.0.0.1:8000/health"},
		{"IPv6 host", []string{"[2001:db8::1]:8000"}, "/health", "http://[2001:db8::1]:8000/health"},
		{"IPv6 unspecified host probes the IPv6 loopback", []string{"[::]:8000"}, "/health", "http://[::1]:8000/health"},
		{"IPv6 zone is percent-encoded", []string{"[fe80::1%eth0]:8000"}, "/health", "http://[fe80::1%25eth0]:8000/health"},
		{"UDP ports are skipped", []string{"53/udp", "8000"}, "/health", "http://127.0.0.1:8000/health"},
		{"only UDP ports", []string{"53/udp"}, "/health", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := newPhaseTestDewy(t)
			d.config.Server = &ServerConfig{Addrs: tt.ports}
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
		d.config.Server = &ServerConfig{Addrs: []string{"8000"}}
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
		d.config.Server = &ServerConfig{Addrs: []string{portOf(t, srv.URL)}}
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
		d.config.Server = &ServerConfig{Addrs: []string{portOf(t, srv.URL)}}
		d.config.Health.Path = "/health"
		d.config.Health.Timeout = 100 * time.Millisecond
		if err := d.verifyServerHealth(context.Background()); err == nil {
			t.Error("verifyServerHealth() = nil, want an error for a 503 response")
		}
	})
}

func TestVerifyServerHealthOverUnixSocket(t *testing.T) {
	// A short directory: t.TempDir is too long on macOS for a socket path.
	dir, err := os.MkdirTemp("/tmp", "dewy-health")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })

	serve := func(t *testing.T, name string, h http.HandlerFunc) string {
		t.Helper()
		path := filepath.Join(dir, name)
		ln, err := net.Listen("unix", path)
		if err != nil {
			t.Fatal(err)
		}
		srv := &http.Server{Handler: h, ReadHeaderTimeout: time.Second}
		go func() { _ = srv.Serve(ln) }()
		t.Cleanup(func() { _ = srv.Close() })
		return path
	}

	t.Run("probes the first socket without a TCP port", func(t *testing.T) {
		var gotPath, gotHost string
		first := serve(t, "first.sock", func(w http.ResponseWriter, r *http.Request) {
			gotPath, gotHost = r.URL.Path, r.Host
			w.WriteHeader(http.StatusOK)
		})
		second := serve(t, "second.sock", func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusServiceUnavailable)
		})

		d := newPhaseTestDewy(t)
		d.config.Command = SERVER
		d.config.Server = &ServerConfig{Addrs: []string{"53/udp"}, Sockets: []string{first, second}}
		d.config.Health.Path = "health"
		if err := d.verifyServerHealth(context.Background()); err != nil {
			t.Fatalf("verifyServerHealth() = %v, want nil", err)
		}
		if gotPath != "/health" || gotHost != "localhost" {
			t.Errorf("request path %q host %q, want /health on localhost", gotPath, gotHost)
		}
	})

	t.Run("fails on an error status", func(t *testing.T) {
		path := serve(t, "unhealthy.sock", func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusServiceUnavailable)
		})
		d := newPhaseTestDewy(t)
		d.config.Command = SERVER
		d.config.Server = &ServerConfig{Sockets: []string{path}}
		d.config.Health.Path = "/health"
		d.config.Health.Timeout = 100 * time.Millisecond
		if err := d.verifyServerHealth(context.Background()); err == nil {
			t.Error("verifyServerHealth() = nil, want an error for a 503 response")
		}
	})

	t.Run("a TCP port takes precedence over a socket", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))
		defer srv.Close()
		path := serve(t, "ignored.sock", func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusServiceUnavailable)
		})

		d := newPhaseTestDewy(t)
		d.config.Command = SERVER
		d.config.Server = &ServerConfig{Addrs: []string{portOf(t, srv.URL)}, Sockets: []string{path}}
		d.config.Health.Path = "/health"
		d.config.Health.Timeout = 2 * time.Second
		if err := d.verifyServerHealth(context.Background()); err != nil {
			t.Errorf("verifyServerHealth() = %v, want the TCP port to be probed", err)
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
	d, oldRelease, newRelease := newRollbackTestDewy(t)
	// A server that is up is restarted onto the restored release.
	srv := &fakeServer{}
	d.server = srv
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

	if srv.restartCount() != 1 {
		t.Errorf("restarts = %d, want the rollback to restart the managed server once", srv.restartCount())
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

func TestRollbackServer_NotSwitched(t *testing.T) {
	d, oldRelease, _ := newRollbackTestDewy(t)
	// The restart failed before the switch, so the previous process still
	// serves the previous release and must not be restarted.
	srv := &fakeServer{}
	d.server = srv
	d.isServerRunning = true

	st := cacheState{
		key:         "v2.0.0--app.tar.gz",
		prevKey:     "v1.0.0--app.tar.gz",
		prevRelease: oldRelease,
	}
	res := &registry.CurrentResponse{ID: "id", Tag: "v2.0.0"}
	cause := fmt.Errorf("%w: worker failed the health check", errNotSwitched)

	d.rollbackServer(context.Background(), res, st, cause)

	if got := currentTarget(t, d); got != oldRelease {
		t.Errorf("current points at %q, want the previous release %q", got, oldRelease)
	}
	if got := d.blockedVersion(); got != st.key {
		t.Errorf("blockedVersion() = %q, want %q", got, st.key)
	}
	if srv.restartCount() != 0 {
		t.Errorf("restarts = %d, want none while the previous process still serves", srv.restartCount())
	}
	notify, _ := d.notifier.(*mockNotify)
	if len(notify.messages) != 1 || !strings.Contains(notify.messages[0], "never stopped serving") {
		t.Errorf("notifications = %v, want one message reporting that the previous process kept serving", notify.messages)
	}
}

func TestRestartServerWrapsNotSwitched(t *testing.T) {
	d := newPhaseTestDewy(t)
	d.server = &fakeServer{restartErr: errors.New("worker exited during the startup window")}
	if err := d.restartServer(context.Background()); !errors.Is(err, errNotSwitched) {
		t.Errorf("restartServer() = %v, want it to wrap errNotSwitched", err)
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
	d.config.Server = &ServerConfig{Addrs: ports}
	d.config.Health.Path = "/health"

	if got, want := d.serverHealthURL(), "http://127.0.0.1:8080/health"; got != want {
		t.Errorf("serverHealthURL() = %q, want %q", got, want)
	}
}

func TestRollbackServer_StartsAServerThatNeverCameUp(t *testing.T) {
	d, oldRelease, _ := newRollbackTestDewy(t)
	// A deploy can fail before the server exists: its port could not be
	// bound, so nothing is running and there is nothing to restart.
	d.isServerRunning = false

	st := cacheState{
		key:         "v2.0.0--app.tar.gz",
		prevKey:     "v1.0.0--app.tar.gz",
		prevRelease: oldRelease,
	}
	res := &registry.CurrentResponse{ID: "id", Tag: "v2.0.0"}

	d.rollbackServer(context.Background(), res, st, errors.New("address already in use"))

	if got := currentTarget(t, d); got != oldRelease {
		t.Errorf("current points at %q, want the previous release %q", got, oldRelease)
	}
	if got := d.blockedVersion(); got != st.key {
		t.Errorf("blockedVersion() = %q, want %q", got, st.key)
	}

	// The test config has no command, so starting cannot succeed. What matters
	// is that the rollback tried to start rather than restarting a server
	// that is not there, and that it says so instead of reporting success.
	if d.server != nil {
		t.Error("a server is recorded although starting it cannot succeed")
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

func TestAwaitWorkerSwap(t *testing.T) {
	t.Run("the previous worker has exited", func(t *testing.T) {
		d := newPhaseTestDewy(t)
		d.server = &fakeServer{}
		if !d.awaitWorkerSwap(context.Background(), time.Second) {
			t.Error("awaitWorkerSwap() = false, want true once only the new worker is left")
		}
	})

	t.Run("waits for the previous worker to go", func(t *testing.T) {
		d := newPhaseTestDewy(t)
		retired := make(chan struct{})
		d.server = &fakeServer{retired: retired}
		go func() {
			time.Sleep(100 * time.Millisecond)
			close(retired)
		}()
		if !d.awaitWorkerSwap(context.Background(), 3*time.Second) {
			t.Error("awaitWorkerSwap() = false, want true after the previous worker exits")
		}
	})

	t.Run("gives up when the previous worker stays", func(t *testing.T) {
		d := newPhaseTestDewy(t)
		d.server = &fakeServer{retired: make(chan struct{})}
		if d.awaitWorkerSwap(context.Background(), 100*time.Millisecond) {
			t.Error("awaitWorkerSwap() = true, want false while the previous worker is still alive")
		}
	})

	t.Run("no server running", func(t *testing.T) {
		d := newPhaseTestDewy(t)
		if d.awaitWorkerSwap(context.Background(), time.Second) {
			t.Error("awaitWorkerSwap() = true, want false without a running server")
		}
	})
}
