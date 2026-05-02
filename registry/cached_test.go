package registry

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/linyows/dewy/cache"
)

// fakeAtomicCache is an in-memory cache.AtomicCache used to drive the
// registry.Cached decorator's logic without S3/GCS network.
type fakeAtomicCache struct {
	mu          sync.Mutex
	store       map[string][]byte
	versions    map[string]int64
	registryTTL time.Duration
}

func newFakeAtomicCache() *fakeAtomicCache {
	return &fakeAtomicCache{
		store:    map[string][]byte{},
		versions: map[string]int64{},
	}
}

func (f *fakeAtomicCache) Read(key string) ([]byte, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	v, ok := f.store[key]
	if !ok {
		return nil, fmt.Errorf("%w: %s", cache.ErrNotFound, key)
	}
	return append([]byte(nil), v...), nil
}

func (f *fakeAtomicCache) Write(key string, data []byte) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.store[key] = append([]byte(nil), data...)
	f.versions[key]++
	return nil
}

func (f *fakeAtomicCache) Delete(key string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.store, key)
	delete(f.versions, key)
	return nil
}

func (f *fakeAtomicCache) List() ([]string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	keys := make([]string, 0, len(f.store))
	for k := range f.store {
		keys = append(keys, k)
	}
	return keys, nil
}

func (f *fakeAtomicCache) GetDir() string             { return "" }
func (f *fakeAtomicCache) RegistryTTL() time.Duration { return f.registryTTL }

func (f *fakeAtomicCache) ReadWithVersion(key string) ([]byte, string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	v, ok := f.store[key]
	if !ok {
		return nil, "", fmt.Errorf("%w: %s", cache.ErrNotFound, key)
	}
	return append([]byte(nil), v...), strconv.FormatInt(f.versions[key], 10), nil
}

func (f *fakeAtomicCache) WriteIfMatch(key, version string, data []byte) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	current := ""
	if g := f.versions[key]; g > 0 {
		current = strconv.FormatInt(g, 10)
	}
	if version != current {
		return "", fmt.Errorf("%w: %s", cache.ErrConflict, key)
	}
	f.store[key] = append([]byte(nil), data...)
	f.versions[key]++
	return strconv.FormatInt(f.versions[key], 10), nil
}

// Compile-time check.
var _ cache.AtomicCache = (*fakeAtomicCache)(nil)

// mockUpstream is the Registry the decorator wraps.
type mockUpstream struct {
	mu       sync.Mutex
	tag      string
	calls    int
	err      error
	delay    time.Duration
	reportFn func(ctx context.Context, req *ReportRequest) error
}

func (m *mockUpstream) Current(ctx context.Context) (*CurrentResponse, error) {
	m.mu.Lock()
	m.calls++
	tag := m.tag
	if tag == "" {
		tag = "v1.0.0"
	}
	delay := m.delay
	err := m.err
	m.mu.Unlock()
	if delay > 0 {
		time.Sleep(delay)
	}
	if err != nil {
		return nil, err
	}
	return &CurrentResponse{
		ID:          "id",
		Tag:         tag,
		ArtifactURL: "https://example.com/" + tag + ".tar.gz",
	}, nil
}

func (m *mockUpstream) Report(ctx context.Context, req *ReportRequest) error {
	if m.reportFn != nil {
		return m.reportFn(ctx, req)
	}
	return nil
}

func (m *mockUpstream) Calls() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.calls
}

func newCachedForTest(t *testing.T, ttl time.Duration) (*Cached, *mockUpstream, *fakeAtomicCache) {
	t.Helper()
	upstream := &mockUpstream{tag: "v1.2.3"}
	fakeCache := newFakeAtomicCache()
	c := NewCached(upstream, fakeCache, ttl, testLogger())
	c.wait = 5 * time.Millisecond
	return c, upstream, fakeCache
}

func TestCachedFirstCallHitsUpstream(t *testing.T) {
	c, upstream, _ := newCachedForTest(t, time.Minute)
	res, err := c.Current(context.Background())
	if err != nil {
		t.Fatalf("Current: %v", err)
	}
	if res.Tag != "v1.2.3" {
		t.Errorf("got tag %q", res.Tag)
	}
	if got := upstream.Calls(); got != 1 {
		t.Errorf("expected 1 upstream call, got %d", got)
	}
}

func TestCachedSubsequentCallsHitCache(t *testing.T) {
	c, upstream, _ := newCachedForTest(t, time.Minute)
	for i := 0; i < 5; i++ {
		if _, err := c.Current(context.Background()); err != nil {
			t.Fatalf("call %d: %v", i, err)
		}
	}
	if got := upstream.Calls(); got != 1 {
		t.Errorf("expected 1 upstream call across 5 reads, got %d", got)
	}
}

func TestCachedRefreshesAfterTTL(t *testing.T) {
	c, upstream, _ := newCachedForTest(t, 50*time.Millisecond)
	if _, err := c.Current(context.Background()); err != nil {
		t.Fatal(err)
	}
	time.Sleep(80 * time.Millisecond)
	if _, err := c.Current(context.Background()); err != nil {
		t.Fatal(err)
	}
	if got := upstream.Calls(); got != 2 {
		t.Errorf("expected 2 upstream calls (initial + post-TTL), got %d", got)
	}
}

func TestCachedSharedAcrossInstances(t *testing.T) {
	// Two Cached instances share one fake cache; only one of them should
	// hit upstream per TTL window.
	upstream := &mockUpstream{tag: "v1.2.3"}
	fakeCache := newFakeAtomicCache()
	a := NewCached(upstream, fakeCache, time.Minute, testLogger())
	b := NewCached(upstream, fakeCache, time.Minute, testLogger())
	a.wait = 5 * time.Millisecond
	b.wait = 5 * time.Millisecond

	if _, err := a.Current(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, err := b.Current(context.Background()); err != nil {
		t.Fatal(err)
	}
	if got := upstream.Calls(); got != 1 {
		t.Errorf("expected 1 upstream call across paired instances, got %d", got)
	}
}

func TestCachedFailOpenServesStale(t *testing.T) {
	c, upstream, _ := newCachedForTest(t, 50*time.Millisecond)

	// Seed the cache with a successful call.
	if _, err := c.Current(context.Background()); err != nil {
		t.Fatal(err)
	}

	// Force upstream to fail and let the entry go stale.
	upstream.mu.Lock()
	upstream.err = errors.New("upstream down")
	upstream.mu.Unlock()
	time.Sleep(80 * time.Millisecond)

	res, err := c.Current(context.Background())
	if err != nil {
		t.Fatalf("expected stale-but-usable, got error: %v", err)
	}
	if res == nil || res.Tag != "v1.2.3" {
		t.Errorf("expected stale tag v1.2.3, got %+v", res)
	}
}

func TestCachedReportPassthrough(t *testing.T) {
	c, upstream, _ := newCachedForTest(t, time.Minute)
	called := false
	upstream.reportFn = func(ctx context.Context, req *ReportRequest) error {
		called = true
		if req.Tag != "v1.2.3" {
			t.Errorf("unexpected tag in Report: %q", req.Tag)
		}
		return nil
	}
	if err := c.Report(context.Background(), &ReportRequest{Tag: "v1.2.3", Command: "server"}); err != nil {
		t.Fatal(err)
	}
	if !called {
		t.Error("upstream Report was not invoked")
	}
}
