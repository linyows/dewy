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
	"github.com/linyows/dewy/internal/sysdeps/fake"
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
	c := NewCached(upstream, "ghr://test/scope", fakeCache, ttl, testLogger())
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
	a := NewCached(upstream, "ghr://test/scope", fakeCache, time.Minute, testLogger())
	b := NewCached(upstream, "ghr://test/scope", fakeCache, time.Minute, testLogger())
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

func TestCachedReleasesLockAfterUpstreamFailure(t *testing.T) {
	// On upstream failure with no prior entry, the leader should release
	// the lock so a peer can immediately attempt the next refresh once
	// upstream recovers, rather than waiting out lockTTL.
	upstream := &mockUpstream{tag: "v1.2.3", err: errors.New("upstream down")}
	fakeCache := newFakeAtomicCache()
	leader := NewCached(upstream, "ghr://test/scope", fakeCache, 50*time.Millisecond, testLogger())
	leader.wait = 5 * time.Millisecond

	// Upstream fails and there is no cached entry to fall back to —
	// expect an error.
	if _, err := leader.Current(context.Background()); err == nil {
		t.Fatal("expected error: upstream down with empty cache")
	}

	// Recover upstream and create a peer instance. The peer should not be
	// blocked by a stale lock; it should claim immediately and succeed.
	upstream.mu.Lock()
	upstream.err = nil
	upstream.mu.Unlock()

	peer := NewCached(upstream, "ghr://test/scope", fakeCache, 50*time.Millisecond, testLogger())
	peer.wait = 5 * time.Millisecond

	start := time.Now()
	res, err := peer.Current(context.Background())
	if err != nil {
		t.Fatalf("peer Current after lock release: %v", err)
	}
	if res == nil || res.Tag != "v1.2.3" {
		t.Errorf("unexpected response: %+v", res)
	}
	// We should be well under lockTTL, otherwise the lock was not released.
	if elapsed := time.Since(start); elapsed > leader.lockTTL/2 {
		t.Errorf("peer waited %v before refreshing — lock-release path is broken", elapsed)
	}
}

func TestCachedScopeCanonicalization(t *testing.T) {
	// Two scopes that differ only in query-parameter order should produce
	// the same cache key, so peers configured with semantically identical
	// registry URLs deduplicate as expected.
	a := cacheKeyForScope("ghr://owner/repo?artifact=foo&pre-release=true")
	b := cacheKeyForScope("ghr://owner/repo?pre-release=true&artifact=foo")
	if a != b {
		t.Errorf("scope canonicalization failed: %q != %q", a, b)
	}

	c := cacheKeyForScope("ghr://owner/other?artifact=foo&pre-release=true")
	if a == c {
		t.Errorf("different registries collided: %q == %q", a, c)
	}
}

func TestCachedDifferentScopesDoNotShare(t *testing.T) {
	// Two Cached instances backed by the same fake cache but with different
	// scopes (e.g., different registry URLs) must not share entries.
	fakeCache := newFakeAtomicCache()
	upstreamA := &mockUpstream{tag: "v1.0.0"}
	upstreamB := &mockUpstream{tag: "v2.0.0"}
	a := NewCached(upstreamA, "ghr://owner/repoA", fakeCache, time.Minute, testLogger())
	b := NewCached(upstreamB, "ghr://owner/repoB", fakeCache, time.Minute, testLogger())

	resA, err := a.Current(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	resB, err := b.Current(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if resA.Tag == resB.Tag {
		t.Errorf("scoped instances should not share entries; both got %q", resA.Tag)
	}
	if upstreamA.Calls() != 1 || upstreamB.Calls() != 1 {
		t.Errorf("each scope should hit its own upstream; got A=%d B=%d", upstreamA.Calls(), upstreamB.Calls())
	}
}

func TestCachedRefreshAfterTTLDeterministic(t *testing.T) {
	// With an injected fake clock, the post-TTL refresh check is deterministic:
	// no real sleep, no flakiness from scheduler jitter.
	upstream := &mockUpstream{tag: "v1.2.3"}
	fakeCache := newFakeAtomicCache()
	clk := fake.NewClock(time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC))
	c := NewCached(upstream, "ghr://test/scope", fakeCache, time.Minute, testLogger(), WithClock(clk))
	c.wait = 5 * time.Millisecond

	if _, err := c.Current(context.Background()); err != nil {
		t.Fatalf("first Current: %v", err)
	}
	if got := upstream.Calls(); got != 1 {
		t.Fatalf("after first call: want 1, got %d", got)
	}

	// Within TTL — should hit cache.
	clk.Advance(30 * time.Second)
	if _, err := c.Current(context.Background()); err != nil {
		t.Fatalf("within-TTL Current: %v", err)
	}
	if got := upstream.Calls(); got != 1 {
		t.Errorf("within TTL: want 1 upstream call, got %d", got)
	}

	// Past TTL — should refresh upstream exactly once.
	clk.Advance(2 * time.Minute)
	if _, err := c.Current(context.Background()); err != nil {
		t.Fatalf("post-TTL Current: %v", err)
	}
	if got := upstream.Calls(); got != 2 {
		t.Errorf("post TTL: want 2 upstream calls, got %d", got)
	}
}

func TestCachedNodeIDFromEnv(t *testing.T) {
	upstream := &mockUpstream{tag: "v1"}
	fakeCache := newFakeAtomicCache()
	env := fake.NewEnv().SetHostname("host-a").SetPid(42)
	c := NewCached(upstream, "ghr://x", fakeCache, time.Minute, testLogger(), WithEnv(env))
	if c.nodeID != "host-a:42" {
		t.Errorf("nodeID = %q, want host-a:42", c.nodeID)
	}
}

// Passing nil to WithClock / WithEnv must leave the real defaults installed
// rather than panicking later inside Current().
func TestCachedOptionsIgnoreNil(t *testing.T) {
	upstream := &mockUpstream{tag: "v1.2.3"}
	fakeCache := newFakeAtomicCache()
	c := NewCached(upstream, "ghr://x", fakeCache, time.Minute, testLogger(), WithClock(nil), WithEnv(nil))
	if c.clock == nil {
		t.Fatal("WithClock(nil) wiped the default clock; expected real clock to remain")
	}
	if c.nodeID == "" {
		t.Fatal("WithEnv(nil) wiped the default nodeID; expected real env to remain")
	}
	// Smoke: Current must not panic with the defaults still in place.
	if _, err := c.Current(context.Background()); err != nil {
		t.Fatalf("Current after nil options: %v", err)
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
