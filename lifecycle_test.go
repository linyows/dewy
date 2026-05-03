package dewy

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/linyows/dewy/registry"
)

// newPhaseTestDewy returns a Dewy with the bare-minimum dependencies wired up
// for phase-level unit tests: real default config + injected registry +
// per-test isolated file cache (so cache list is empty by default).
func newPhaseTestDewy(t *testing.T) *Dewy {
	t.Helper()
	c := DefaultConfig()
	c.Command = ASSETS
	c.Registry = "ghr://linyows/dewy"
	c.Cache = CacheConfig{
		Type:       FILE,
		Expiration: 10,
		URL:        "file://" + t.TempDir(),
	}
	d, err := New(c, testLogger())
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	d.root = t.TempDir()
	return d
}

func TestResolveCurrent_GracePeriod(t *testing.T) {
	d := newPhaseTestDewy(t)
	releaseTime := time.Now().Add(-5 * time.Minute) // within 30-min grace
	d.registry = &mockRegistry{
		currentFunc: func(ctx context.Context) (*registry.CurrentResponse, error) {
			return nil, &registry.ArtifactNotFoundError{
				ArtifactName: "missing.tar.gz",
				ReleaseTime:  &releaseTime,
				Message:      "artifact not found",
			}
		},
	}

	res, err := d.resolveCurrent(context.Background())
	if err != nil {
		t.Fatalf("expected (nil, nil) within grace period, got err=%v", err)
	}
	if res != nil {
		t.Errorf("expected nil res within grace period, got %+v", res)
	}
}

func TestResolveCurrent_GracePeriodExpired(t *testing.T) {
	d := newPhaseTestDewy(t)
	releaseTime := time.Now().Add(-2 * time.Hour) // outside 30-min grace
	wantErr := &registry.ArtifactNotFoundError{
		ArtifactName: "missing.tar.gz",
		ReleaseTime:  &releaseTime,
		Message:      "artifact not found",
	}
	d.registry = &mockRegistry{
		currentFunc: func(ctx context.Context) (*registry.CurrentResponse, error) {
			return nil, wantErr
		},
	}

	res, err := d.resolveCurrent(context.Background())
	if !errors.Is(err, wantErr) {
		t.Errorf("expected wrapped ArtifactNotFoundError, got %v", err)
	}
	if res != nil {
		t.Errorf("expected nil res on expired grace, got %+v", res)
	}
}

func TestResolveCurrent_SlotMismatch(t *testing.T) {
	d := newPhaseTestDewy(t)
	d.config.Slot = "blue"
	d.registry = &mockRegistry{
		currentFunc: func(ctx context.Context) (*registry.CurrentResponse, error) {
			return &registry.CurrentResponse{Tag: "v1.0.0+green", Slot: "green"}, nil
		},
	}

	res, err := d.resolveCurrent(context.Background())
	if err != nil {
		t.Fatalf("expected nil err, got %v", err)
	}
	if res != nil {
		t.Errorf("expected nil res on slot mismatch, got %+v", res)
	}
}

func TestResolveCurrent_SlotMatch(t *testing.T) {
	d := newPhaseTestDewy(t)
	d.config.Slot = "blue"
	want := &registry.CurrentResponse{Tag: "v1.0.0+blue", Slot: "blue"}
	d.registry = &mockRegistry{
		currentFunc: func(ctx context.Context) (*registry.CurrentResponse, error) {
			return want, nil
		},
	}

	res, err := d.resolveCurrent(context.Background())
	if err != nil {
		t.Fatalf("got err %v", err)
	}
	if res != want {
		t.Errorf("got res %+v, want %+v", res, want)
	}
}

func TestResolveCurrent_GenericError(t *testing.T) {
	d := newPhaseTestDewy(t)
	wantErr := errors.New("registry boom")
	d.registry = &mockRegistry{
		currentFunc: func(ctx context.Context) (*registry.CurrentResponse, error) {
			return nil, wantErr
		},
	}

	res, err := d.resolveCurrent(context.Background())
	if !errors.Is(err, wantErr) {
		t.Errorf("expected wantErr passthrough, got %v", err)
	}
	if res != nil {
		t.Errorf("expected nil res on error, got %+v", res)
	}
}

func TestResolveCacheState_EmptyCache(t *testing.T) {
	d := newPhaseTestDewy(t)
	res := &registry.CurrentResponse{
		Tag:         "v1.0.0",
		ArtifactURL: "https://example.com/app.zip",
	}

	st, err := d.resolveCacheState(context.Background(), res)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if st.foundInCache {
		t.Error("foundInCache should be false on empty cache")
	}
	if st.skip {
		t.Error("skip should be false on empty cache")
	}
	if st.key != "v1.0.0--app.zip" {
		t.Errorf("key = %q, want v1.0.0--app.zip", st.key)
	}
}

func TestResolveCacheState_AlreadyCurrentAssets(t *testing.T) {
	d := newPhaseTestDewy(t)
	d.config.Command = ASSETS
	res := &registry.CurrentResponse{
		Tag:         "v1.0.0",
		ArtifactURL: "https://example.com/app.zip",
	}
	key := d.cachekeyName(res)
	if err := d.cache.Write(key, []byte("payload")); err != nil {
		t.Fatal(err)
	}
	if err := d.cache.Write(currentkeyName, []byte(key)); err != nil {
		t.Fatal(err)
	}

	st, err := d.resolveCacheState(context.Background(), res)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if !st.skip {
		t.Error("ASSETS at current version should skip")
	}
}

func TestResolveCacheState_AlreadyCurrentServerRunning(t *testing.T) {
	d := newPhaseTestDewy(t)
	d.config.Command = SERVER
	d.isServerRunning = true
	res := &registry.CurrentResponse{
		Tag:         "v1.0.0",
		ArtifactURL: "https://example.com/app.zip",
	}
	key := d.cachekeyName(res)
	if err := d.cache.Write(key, []byte("payload")); err != nil {
		t.Fatal(err)
	}
	if err := d.cache.Write(currentkeyName, []byte(key)); err != nil {
		t.Fatal(err)
	}

	st, err := d.resolveCacheState(context.Background(), res)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if !st.skip {
		t.Error("SERVER at current version while running should skip")
	}
}

func TestResolveCacheState_AlreadyCurrentServerCrashed(t *testing.T) {
	// Server is at the right version on disk but the process is not running:
	// fall through to redeploy from the cached artifact (foundInCache=true).
	d := newPhaseTestDewy(t)
	d.config.Command = SERVER
	d.isServerRunning = false
	res := &registry.CurrentResponse{
		Tag:         "v1.0.0",
		ArtifactURL: "https://example.com/app.zip",
	}
	key := d.cachekeyName(res)
	if err := d.cache.Write(key, []byte("payload")); err != nil {
		t.Fatal(err)
	}
	if err := d.cache.Write(currentkeyName, []byte(key)); err != nil {
		t.Fatal(err)
	}

	st, err := d.resolveCacheState(context.Background(), res)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if st.skip {
		t.Error("SERVER at current version but not running should NOT skip (redeploy path)")
	}
	if !st.foundInCache {
		t.Error("foundInCache should be true to avoid re-download")
	}
}

func TestPromoteAndReport_DisableReport(t *testing.T) {
	d := newPhaseTestDewy(t)
	d.disableReport = true
	d.config.Command = ASSETS
	reportCalled := false
	d.registry = &mockRegistry{
		reportFunc: func(ctx context.Context, _ *registry.ReportRequest) error {
			reportCalled = true
			return nil
		},
	}
	notify := &mockNotify{}
	d.notifier = notify

	res := &registry.CurrentResponse{ID: "id-1", Tag: "v1.0.0"}
	if err := d.promoteAndReport(context.Background(), res); err != nil {
		t.Fatalf("err: %v", err)
	}
	if reportCalled {
		t.Error("registry.Report should not be called when disableReport=true")
	}
	if d.cVer != "v1.0.0" {
		t.Errorf("cVer = %q, want v1.0.0", d.cVer)
	}
}

func TestPromoteAndReport_ReportEnabled(t *testing.T) {
	d := newPhaseTestDewy(t)
	d.disableReport = false
	d.config.Command = ASSETS
	var got *registry.ReportRequest
	d.registry = &mockRegistry{
		reportFunc: func(ctx context.Context, req *registry.ReportRequest) error {
			got = req
			return nil
		},
	}
	d.notifier = &mockNotify{}

	res := &registry.CurrentResponse{ID: "id-2", Tag: "v2.0.0"}
	if err := d.promoteAndReport(context.Background(), res); err != nil {
		t.Fatalf("err: %v", err)
	}
	if got == nil {
		t.Fatal("registry.Report should be called when disableReport=false")
	}
	if got.ID != "id-2" || got.Tag != "v2.0.0" || got.Command != "assets" {
		t.Errorf("ReportRequest = %+v, want {id-2, v2.0.0, assets}", got)
	}
}

func TestReportDeployment_DisableReport(t *testing.T) {
	d := newPhaseTestDewy(t)
	d.disableReport = true
	called := false
	d.registry = &mockRegistry{
		reportFunc: func(ctx context.Context, _ *registry.ReportRequest) error {
			called = true
			return nil
		},
	}
	d.reportDeployment(context.Background(), &registry.CurrentResponse{Tag: "v1"})
	if called {
		t.Error("Report should not be called when disabled")
	}
}
