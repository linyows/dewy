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
		{"host and port", []string{"127.0.0.1:9000"}, "/health", "http://127.0.0.1:9000/health"},
		{"path without a leading slash", []string{"8000"}, "health", "http://127.0.0.1:8000/health"},
		{"first port wins", []string{"8000", "8001"}, "/health", "http://127.0.0.1:8000/health"},
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
