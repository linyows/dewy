package registry

import (
	"context"
	"encoding/json"
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
	for i := range 5 {
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

// stubLocker returns a fixed outcome from every Acquire, so the decorator's
// behavior under contention and under a broken lock backend can be pinned
// down without racing real goroutines.
type stubLocker struct {
	err      error
	acquires int
	// onAcquire runs just before a successful Acquire returns, so tests can
	// simulate a peer winning the race in the window between our read and our
	// acquisition.
	onAcquire func()
	// lock, when set, is handed out instead of a plain held lock.
	lock cache.Lock
	mu   sync.Mutex
}

func (s *stubLocker) Acquire(_ context.Context, _ string, _ cache.LockOptions) (cache.Lock, error) {
	s.mu.Lock()
	s.acquires++
	fn := s.onAcquire
	s.mu.Unlock()
	if s.err != nil {
		return nil, s.err
	}
	if fn != nil {
		fn()
	}
	if s.lock != nil {
		return s.lock, nil
	}
	return noopLock{}, nil
}

func (s *stubLocker) Acquires() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.acquires
}

type noopLock struct{}

func (noopLock) Lost() <-chan struct{} { return nil }
func (noopLock) Unlock() error         { return nil }

// lostLock is a lock whose lease has already expired, standing in for a
// refresh that outlived its lock.
type lostLock struct{}

func (lostLock) Lost() <-chan struct{} {
	ch := make(chan struct{})
	close(ch)
	return ch
}
func (lostLock) Unlock() error { return nil }

var _ cache.Locker = (*stubLocker)(nil)

// seedEntry writes a cache entry directly, bypassing Current, so tests can
// set up a specific staleness and lock state.
func seedEntry(t *testing.T, c *Cached, fc *fakeAtomicCache, entry *cachedEntry) {
	t.Helper()
	data, err := json.Marshal(entry)
	if err != nil {
		t.Fatalf("marshal entry: %v", err)
	}
	if err := fc.Write(c.cacheKey, data); err != nil {
		t.Fatalf("seed entry: %v", err)
	}
}

func TestCachedConcurrentCallersHitUpstreamOnce(t *testing.T) {
	// The reason this decorator exists: a fleet that wakes up together must
	// produce one upstream request, not one per instance.
	const instances = 8
	upstream := &mockUpstream{tag: "v1.2.3", delay: 30 * time.Millisecond}
	fakeCache := newFakeAtomicCache()

	var wg sync.WaitGroup
	results := make([]*CurrentResponse, instances)
	errs := make([]error, instances)

	for i := range instances {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			c := NewCached(upstream, "ghr://test/scope", fakeCache, time.Minute, testLogger())
			c.wait = 2 * time.Millisecond
			results[i], errs[i] = c.Current(context.Background())
		}(i)
	}
	wg.Wait()

	for i := range instances {
		if errs[i] != nil {
			t.Fatalf("instance %d: %v", i, errs[i])
		}
		if results[i] == nil || results[i].Tag != "v1.2.3" {
			t.Errorf("instance %d got %+v, want tag v1.2.3", i, results[i])
		}
	}
	if got := upstream.Calls(); got != 1 {
		t.Errorf("want exactly 1 upstream call across %d instances, got %d", instances, got)
	}
}

func TestCachedLockBusyServesStale(t *testing.T) {
	// A peer holds the refresh lock for its entire TTL without publishing.
	// Adding our own upstream request to whatever is already wrong would
	// make it worse; the last known response is the better answer.
	upstream := &mockUpstream{tag: "v2.0.0"}
	fakeCache := newFakeAtomicCache()
	busy := &stubLocker{err: fmt.Errorf("%w: registry", cache.ErrLockBusy)}
	c := NewCached(upstream, "ghr://test/scope", fakeCache, time.Minute, testLogger(), WithLocker(busy))
	c.wait = 2 * time.Millisecond
	c.lockTTL = 20 * time.Millisecond

	seedEntry(t, c, fakeCache, &cachedEntry{
		Response:  &CurrentResponse{Tag: "v1.0.0"},
		FetchedAt: time.Now().Add(-time.Hour),
	})

	res, err := c.Current(context.Background())
	if err != nil {
		t.Fatalf("Current: %v", err)
	}
	if res == nil || res.Tag != "v1.0.0" {
		t.Errorf("want the stale cached response v1.0.0, got %+v", res)
	}
	if got := upstream.Calls(); got != 0 {
		t.Errorf("want 0 upstream calls while a peer holds the lock, got %d", got)
	}
	if busy.Acquires() == 0 {
		t.Error("expected at least one acquisition attempt")
	}
}

func TestCachedLockBusyColdStartFallsBackToUpstream(t *testing.T) {
	// With nothing cached there is no stale response to serve. Waiting
	// forever on a peer would deadlock a fleet's first deploy, so the
	// fallback is a direct upstream call.
	upstream := &mockUpstream{tag: "v2.0.0"}
	fakeCache := newFakeAtomicCache()
	busy := &stubLocker{err: fmt.Errorf("%w: registry", cache.ErrLockBusy)}
	c := NewCached(upstream, "ghr://test/scope", fakeCache, time.Minute, testLogger(), WithLocker(busy))
	c.wait = 2 * time.Millisecond
	c.lockTTL = 20 * time.Millisecond

	res, err := c.Current(context.Background())
	if err != nil {
		t.Fatalf("Current: %v", err)
	}
	if res == nil || res.Tag != "v2.0.0" {
		t.Errorf("want the upstream response, got %+v", res)
	}
	if got := upstream.Calls(); got != 1 {
		t.Errorf("want 1 upstream call on cold start, got %d", got)
	}
}

func TestCachedLockBackendErrorDegradesGracefully(t *testing.T) {
	// The lock backend being down must not stop deployments.
	upstream := &mockUpstream{tag: "v2.0.0"}
	fakeCache := newFakeAtomicCache()
	broken := &stubLocker{err: errors.New("lock backend unreachable")}
	c := NewCached(upstream, "ghr://test/scope", fakeCache, time.Minute, testLogger(), WithLocker(broken))
	c.wait = 2 * time.Millisecond

	res, err := c.Current(context.Background())
	if err != nil {
		t.Fatalf("Current: %v", err)
	}
	if res == nil || res.Tag != "v2.0.0" {
		t.Errorf("want the upstream response, got %+v", res)
	}
	if got := upstream.Calls(); got != 1 {
		t.Errorf("want 1 upstream call when the lock backend is down, got %d", got)
	}
	if got := broken.Acquires(); got != 1 {
		t.Errorf("a broken lock backend must not be retried in a loop, got %d attempts", got)
	}
}

func TestCachedHonorsLegacyEntryLock(t *testing.T) {
	// A peer running a pre-Locker Dewy marks the entry itself. We still wait
	// for it, so that a fleet mid-upgrade does not double up on upstream.
	upstream := &mockUpstream{tag: "v2.0.0"}
	fakeCache := newFakeAtomicCache()
	c := NewCached(upstream, "ghr://test/scope", fakeCache, time.Minute, testLogger())
	c.wait = 2 * time.Millisecond
	c.lockTTL = 100 * time.Millisecond

	// Locked by a legacy peer with 50ms of its lock TTL left.
	seedEntry(t, c, fakeCache, &cachedEntry{
		Response:  &CurrentResponse{Tag: "v1.0.0"},
		FetchedAt: time.Now().Add(-time.Hour),
		LockedAt:  time.Now().Add(-50 * time.Millisecond),
		LockedBy:  "legacy-peer:1",
	})

	start := time.Now()
	res, err := c.Current(context.Background())
	if err != nil {
		t.Fatalf("Current: %v", err)
	}
	if elapsed := time.Since(start); elapsed < 40*time.Millisecond {
		t.Errorf("returned after %v without waiting out the legacy peer's lock", elapsed)
	}
	if res == nil || res.Tag != "v2.0.0" {
		t.Errorf("want the refreshed response after the legacy lock expired, got %+v", res)
	}
}

func TestCachedNeverWritesLegacyLockFields(t *testing.T) {
	// Writing LockedAt would make a pre-Locker peer wait out lockTTL on every
	// poll, because nothing clears it any more.
	c, _, fakeCache := newCachedForTest(t, time.Minute)
	if _, err := c.Current(context.Background()); err != nil {
		t.Fatal(err)
	}

	data, err := fakeCache.Read(c.cacheKey)
	if err != nil {
		t.Fatalf("read published entry: %v", err)
	}
	entry := &cachedEntry{}
	if err := json.Unmarshal(data, entry); err != nil {
		t.Fatalf("decode published entry: %v", err)
	}
	if !entry.LockedAt.IsZero() || entry.LockedBy != "" {
		t.Errorf("published entry carries a legacy lock: LockedAt=%v LockedBy=%q", entry.LockedAt, entry.LockedBy)
	}
}

func TestCachedReleasesLockOnPublish(t *testing.T) {
	// After a refresh the lock must be free, or the next TTL window would
	// stall until the lock TTL expired.
	upstream := &mockUpstream{tag: "v1.2.3"}
	fakeCache := newFakeAtomicCache()
	c := NewCached(upstream, "ghr://test/scope", fakeCache, time.Minute, testLogger())

	if _, err := c.Current(context.Background()); err != nil {
		t.Fatal(err)
	}

	locker, ok := cache.LockerFor(fakeCache)
	if !ok {
		t.Fatal("fake cache should yield a locker")
	}
	lk, err := locker.Acquire(context.Background(), c.lockName, cache.LockOptions{TTL: time.Minute, TryOnce: true})
	if err != nil {
		t.Fatalf("lock was not released after publish: %v", err)
	}
	if err := lk.Unlock(); err != nil {
		t.Fatalf("Unlock: %v", err)
	}
}

func TestCachedRechecksAfterAcquiringLock(t *testing.T) {
	// A peer can publish in the window between our read and our acquisition.
	// Without a re-check we would hold the lock and repeat the request that
	// the peer has already made.
	upstream := &mockUpstream{tag: "v2.0.0"}
	fakeCache := newFakeAtomicCache()
	locker := &stubLocker{}
	c := NewCached(upstream, "ghr://test/scope", fakeCache, time.Minute, testLogger(), WithLocker(locker))
	c.wait = 2 * time.Millisecond

	locker.onAcquire = func() {
		seedEntry(t, c, fakeCache, &cachedEntry{
			Response:  &CurrentResponse{Tag: "v1.9.0"},
			FetchedAt: time.Now(),
		})
	}

	res, err := c.Current(context.Background())
	if err != nil {
		t.Fatalf("Current: %v", err)
	}
	if res == nil || res.Tag != "v1.9.0" {
		t.Errorf("want the peer's freshly published response v1.9.0, got %+v", res)
	}
	if got := upstream.Calls(); got != 0 {
		t.Errorf("want 0 upstream calls when a peer published first, got %d", got)
	}
}

func TestCachedLockBackendErrorStillPollsUpstream(t *testing.T) {
	// A locker that stays broken must not freeze the fleet on the last
	// response it managed to cache: no new release would ever be deployed.
	upstream := &mockUpstream{tag: "v2.0.0"}
	fakeCache := newFakeAtomicCache()
	broken := &stubLocker{err: errors.New("lock backend unreachable")}
	c := NewCached(upstream, "ghr://test/scope", fakeCache, time.Minute, testLogger(), WithLocker(broken))
	c.wait = 2 * time.Millisecond

	seedEntry(t, c, fakeCache, &cachedEntry{
		Response:  &CurrentResponse{Tag: "v1.0.0"},
		FetchedAt: time.Now().Add(-time.Hour),
	})

	res, err := c.Current(context.Background())
	if err != nil {
		t.Fatalf("Current: %v", err)
	}
	if res == nil || res.Tag != "v2.0.0" {
		t.Errorf("want the upstream response v2.0.0, got %+v", res)
	}
	if got := upstream.Calls(); got != 1 {
		t.Errorf("want 1 upstream call when the lock backend is down, got %d", got)
	}
}

func TestCachedLockBackendErrorFallsBackToStale(t *testing.T) {
	// Both the locker and upstream are down. The cached response is all
	// there is, so serve it rather than failing the tick.
	upstream := &mockUpstream{tag: "v2.0.0", err: errors.New("upstream down")}
	fakeCache := newFakeAtomicCache()
	broken := &stubLocker{err: errors.New("lock backend unreachable")}
	c := NewCached(upstream, "ghr://test/scope", fakeCache, time.Minute, testLogger(), WithLocker(broken))
	c.wait = 2 * time.Millisecond

	seedEntry(t, c, fakeCache, &cachedEntry{
		Response:  &CurrentResponse{Tag: "v1.0.0"},
		FetchedAt: time.Now().Add(-time.Hour),
	})

	res, err := c.Current(context.Background())
	if err != nil {
		t.Fatalf("Current: %v", err)
	}
	if res == nil || res.Tag != "v1.0.0" {
		t.Errorf("want the stale cached response v1.0.0, got %+v", res)
	}
}

func TestCachedLockBackendErrorSurfacesUpstreamFailure(t *testing.T) {
	// Nothing cached, locker down, upstream down: there is no answer to give.
	upstream := &mockUpstream{err: errors.New("upstream down")}
	fakeCache := newFakeAtomicCache()
	broken := &stubLocker{err: errors.New("lock backend unreachable")}
	c := NewCached(upstream, "ghr://test/scope", fakeCache, time.Minute, testLogger(), WithLocker(broken))
	c.wait = 2 * time.Millisecond

	if _, err := c.Current(context.Background()); err == nil {
		t.Fatal("expected the upstream error to surface")
	}
}

func TestCachedDoesNotPublishAfterLosingLock(t *testing.T) {
	// A refresh that outlives its lock must not publish: a successor holds
	// the lock by then, and our response would be stamped with a FetchedAt it
	// did not earn.
	upstream := &mockUpstream{tag: "v2.0.0"}
	fakeCache := newFakeAtomicCache()
	expired := &stubLocker{lock: lostLock{}}
	c := NewCached(upstream, "ghr://test/scope", fakeCache, time.Minute, testLogger(), WithLocker(expired))
	c.wait = 2 * time.Millisecond

	res, err := c.Current(context.Background())
	if err != nil {
		t.Fatalf("Current: %v", err)
	}
	if res == nil || res.Tag != "v2.0.0" {
		t.Errorf("the caller should still get its own result, got %+v", res)
	}
	if _, rerr := fakeCache.Read(c.cacheKey); !cache.IsNotFound(rerr) {
		t.Errorf("entry was published despite the lock being lost (read: %v)", rerr)
	}
}
