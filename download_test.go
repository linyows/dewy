package dewy

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/linyows/dewy/cache"
	"github.com/linyows/dewy/registry"
)

// memCache is an in-memory cache.AtomicCache, standing in for the S3 and GCS
// backends so that download coordination can be exercised without a network.
type memCache struct {
	mu       sync.Mutex
	store    map[string][]byte
	versions map[string]int64
	dir      string
	// readErrs makes Read fail for specific keys, standing in for a backend
	// that is reachable but unhappy.
	readErrs map[string]error
}

func newMemCache(dir string) *memCache {
	return &memCache{
		store:    map[string][]byte{},
		versions: map[string]int64{},
		readErrs: map[string]error{},
		dir:      dir,
	}
}

func (m *memCache) Read(key string) ([]byte, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err, ok := m.readErrs[key]; ok {
		return nil, err
	}
	v, ok := m.store[key]
	if !ok {
		return nil, fmt.Errorf("%w: %s", cache.ErrNotFound, key)
	}
	return append([]byte(nil), v...), nil
}

func (m *memCache) Write(key string, data []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.store[key] = append([]byte(nil), data...)
	m.versions[key]++
	return nil
}

func (m *memCache) Delete(key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.store, key)
	delete(m.versions, key)
	return nil
}

func (m *memCache) List() ([]string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	keys := make([]string, 0, len(m.store))
	for k := range m.store {
		keys = append(keys, k)
	}
	return keys, nil
}

func (m *memCache) GetDir() string             { return m.dir }
func (m *memCache) RegistryTTL() time.Duration { return 0 }

func (m *memCache) ReadWithVersion(key string) ([]byte, string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	v, ok := m.store[key]
	if !ok {
		return nil, "", fmt.Errorf("%w: %s", cache.ErrNotFound, key)
	}
	return append([]byte(nil), v...), strconv.FormatInt(m.versions[key], 10), nil
}

func (m *memCache) WriteIfMatch(key, version string, data []byte) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	current := ""
	if g := m.versions[key]; g > 0 {
		current = strconv.FormatInt(g, 10)
	}
	if version != current {
		return "", fmt.Errorf("%w: %s", cache.ErrConflict, key)
	}
	m.store[key] = append([]byte(nil), data...)
	m.versions[key]++
	return strconv.FormatInt(m.versions[key], 10), nil
}

func (m *memCache) Has(key string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	_, ok := m.store[key]
	return ok
}

var _ cache.AtomicCache = (*memCache)(nil)

// sharedCacheView is one instance's handle on a shared store: the same
// objects, its own local directory. Two views model two hosts pointed at one
// bucket, which is what per-instance state has to survive.
type sharedCacheView struct {
	*memCache
	dir string
}

func (v sharedCacheView) GetDir() string { return v.dir }

var _ cache.AtomicCache = (*sharedCacheView)(nil)

// newSharedCacheTestDewy returns a Dewy whose cache backend supports
// conditional writes, so that a locker is available the way it is with S3 or
// GCS.
func newSharedCacheTestDewy(t *testing.T) (*Dewy, *memCache) {
	t.Helper()
	d := newPhaseTestDewy(t)
	kv := newMemCache(t.TempDir())
	d.cache = kv
	locker, ok := cache.LockerFor(kv, cache.WithLockHolderID(d.nodeID))
	if !ok {
		t.Fatal("a conditional-write cache should yield a locker")
	}
	d.locker = locker
	return d, kv
}

func testResponse() *registry.CurrentResponse {
	return &registry.CurrentResponse{
		ID:          "id",
		Tag:         "v1.2.3",
		ArtifactURL: "https://example.com/testapp_linux_amd64.zip",
	}
}

// peerHoldsArtifactLock takes the download lock as another instance would.
func peerHoldsArtifactLock(t *testing.T, kv *memCache, cacheKey string) {
	t.Helper()
	peer := cache.NewCASLocker(kv, cache.WithLockHolderID("peer:1"))
	if _, err := peer.Acquire(context.Background(), artifactLockName(cacheKey), cache.LockOptions{
		TTL:     time.Hour,
		TryOnce: true,
	}); err != nil {
		t.Fatalf("peer could not take the lock: %v", err)
	}
}

// peerPublishes writes an artifact and its index as a peer instance would.
func peerPublishes(t *testing.T, kv *memCache, cacheKey string, data []byte) {
	t.Helper()
	sum := sha256.Sum256(data)
	idx, err := json.Marshal(&blobIndex{
		SHA256:      hex.EncodeToString(sum[:]),
		Size:        int64(len(data)),
		PublishedAt: time.Now(),
		PublishedBy: "peer:1",
	})
	if err != nil {
		t.Fatalf("marshal index: %v", err)
	}
	if err := kv.Write(cacheKey, data); err != nil {
		t.Fatalf("write artifact: %v", err)
	}
	if err := kv.Write(blobIndexKey(cacheKey), idx); err != nil {
		t.Fatalf("write index: %v", err)
	}
}

func TestDownloadAndCache_DefersWhileAPeerDownloads(t *testing.T) {
	// The whole point of the lock: a fleet waking together must not all pull
	// the same artifact from the registry.
	d, kv := newSharedCacheTestDewy(t)
	res := testResponse()
	st := cacheState{key: d.cachekeyName(res)}
	peerHoldsArtifactLock(t, kv, st.key)

	art := &mockArtifact{binary: "testapp", url: res.ArtifactURL}
	d.artifact = art

	err := d.downloadAndCache(context.Background(), res, st)
	if !errors.Is(err, errDeferred) {
		t.Fatalf("want errDeferred while a peer holds the lock, got %v", err)
	}
	if got := art.GetDownloadCount(); got != 0 {
		t.Errorf("want 0 downloads while deferring, got %d", got)
	}
}

func TestDownloadAndCache_LoadsWhatAPeerPublished(t *testing.T) {
	// A follower that deferred earlier comes back, takes the lock, and finds
	// the artifact already there. This is the step that turns N downloads
	// into one.
	d, kv := newSharedCacheTestDewy(t)
	res := testResponse()
	st := cacheState{key: d.cachekeyName(res)}
	peerPublishes(t, kv, st.key, []byte("artifact bytes"))

	art := &mockArtifact{binary: "testapp", url: res.ArtifactURL}
	d.artifact = art

	if err := d.downloadAndCache(context.Background(), res, st); err != nil {
		t.Fatalf("downloadAndCache: %v", err)
	}
	if got := art.GetDownloadCount(); got != 0 {
		t.Errorf("want 0 downloads when a peer already published, got %d", got)
	}
	current, err := d.state().Read(currentkeyName)
	if err != nil {
		t.Fatalf("read current: %v", err)
	}
	if string(current) != st.key {
		t.Errorf("current = %q, want %q", current, st.key)
	}
	if _, err := kv.Read(currentkeyName); err == nil {
		t.Error("current was written to the shared cache; it is per-instance state")
	}
}

func TestDownloadAndCache_PublishesIndexAfterDownloading(t *testing.T) {
	// Without the index a peer has no way to tell that the bytes it finds
	// were verified, nor to check them.
	d, kv := newSharedCacheTestDewy(t)
	res := testResponse()
	st := cacheState{key: d.cachekeyName(res)}
	d.artifact = &mockArtifact{binary: "testapp", url: res.ArtifactURL}

	if err := d.downloadAndCache(context.Background(), res, st); err != nil {
		t.Fatalf("downloadAndCache: %v", err)
	}

	idx, err := d.readBlobIndex(st.key)
	if err != nil {
		t.Fatalf("read index: %v", err)
	}
	data, err := kv.Read(st.key)
	if err != nil {
		t.Fatalf("read artifact: %v", err)
	}
	sum := sha256.Sum256(data)
	if idx.SHA256 != hex.EncodeToString(sum[:]) {
		t.Errorf("index digest does not match the cached bytes")
	}
	if idx.Size != int64(len(data)) {
		t.Errorf("index size = %d, want %d", idx.Size, len(data))
	}
	if idx.PublishedBy != d.nodeID {
		t.Errorf("index publisher = %q, want %q", idx.PublishedBy, d.nodeID)
	}
}

func TestDownloadAndCache_ReleasesLockForTheNextInstance(t *testing.T) {
	d, kv := newSharedCacheTestDewy(t)
	res := testResponse()
	st := cacheState{key: d.cachekeyName(res)}
	d.artifact = &mockArtifact{binary: "testapp", url: res.ArtifactURL}

	if err := d.downloadAndCache(context.Background(), res, st); err != nil {
		t.Fatalf("downloadAndCache: %v", err)
	}

	peer := cache.NewCASLocker(kv, cache.WithLockHolderID("peer:1"))
	if _, err := peer.Acquire(context.Background(), artifactLockName(st.key), cache.LockOptions{
		TTL:     time.Minute,
		TryOnce: true,
	}); err != nil {
		t.Fatalf("lock was not released after the download: %v", err)
	}
}

func TestDownloadAndCache_RejectsSharedArtifactThatFailsItsDigest(t *testing.T) {
	// An object replaced or corrupted in a shared bucket must not be deployed
	// across the fleet just because it arrived through the cache rather than
	// through a download.
	d, kv := newSharedCacheTestDewy(t)
	res := testResponse()
	st := cacheState{key: d.cachekeyName(res)}
	peerPublishes(t, kv, st.key, []byte("artifact bytes"))
	if err := kv.Write(st.key, []byte("tampered bytes")); err != nil {
		t.Fatal(err)
	}

	err := d.downloadAndCache(context.Background(), res, st)
	if err == nil {
		t.Fatal("expected a digest mismatch to fail the deploy")
	}
	if !kv.Has(st.key) {
		t.Error("one instance's verdict deleted the shared object; every peer would have to re-download")
	}
}

func TestDownloadAndCache_DownloadsWhenSharedBytesCarryNoDigest(t *testing.T) {
	// Bytes with no index cannot be checked, so on the path where we hold the
	// lock and have a choice, fetch them ourselves rather than trust them.
	d, kv := newSharedCacheTestDewy(t)
	res := testResponse()
	st := cacheState{key: d.cachekeyName(res)}
	if err := kv.Write(st.key, []byte("artifact bytes")); err != nil {
		t.Fatal(err)
	}

	art := &mockArtifact{binary: "testapp", url: res.ArtifactURL}
	d.artifact = art

	// No index, so the re-check finds nothing and we download for ourselves
	// rather than trusting bytes we cannot verify.
	if err := d.downloadAndCache(context.Background(), res, st); err != nil {
		t.Fatalf("downloadAndCache: %v", err)
	}
	if got := art.GetDownloadCount(); got != 1 {
		t.Errorf("want 1 download when the cached bytes carry no digest, got %d", got)
	}
}

func TestDownloadAndCache_UncoordinatedWithoutALocker(t *testing.T) {
	// The file backend is single-instance; it must keep behaving exactly as
	// it did before coordination existed.
	d := newPhaseTestDewy(t)
	if d.locker != nil {
		t.Fatal("the file backend should yield no locker")
	}
	res := testResponse()
	st := cacheState{key: d.cachekeyName(res)}
	art := &mockArtifact{binary: "testapp", url: res.ArtifactURL}
	d.artifact = art

	if err := d.downloadAndCache(context.Background(), res, st); err != nil {
		t.Fatalf("downloadAndCache: %v", err)
	}
	if got := art.GetDownloadCount(); got != 1 {
		t.Errorf("want 1 download on the uncoordinated path, got %d", got)
	}
}

func TestDownloadAndCache_SkipsWhenAlreadyStaged(t *testing.T) {
	d, _ := newSharedCacheTestDewy(t)
	res := testResponse()
	st := cacheState{key: d.cachekeyName(res), foundInCache: true}
	art := &mockArtifact{binary: "testapp", url: res.ArtifactURL}
	d.artifact = art

	if err := d.downloadAndCache(context.Background(), res, st); err != nil {
		t.Fatalf("downloadAndCache: %v", err)
	}
	if got := art.GetDownloadCount(); got != 0 {
		t.Errorf("want 0 downloads for an artifact already staged, got %d", got)
	}
}

func TestRun_DeferredTickIsNeitherDeployNorFailure(t *testing.T) {
	// A follower that defers has not failed. Reporting an error would put it
	// into polling backoff and fire a notification, on every instance, every
	// time a large artifact is rolled out.
	d, kv := newSharedCacheTestDewy(t)
	res := testResponse()
	d.registry = &mockRegistry{
		currentFunc: func(ctx context.Context) (*registry.CurrentResponse, error) {
			return res, nil
		},
	}
	d.notifier = &mockNotify{}
	peerHoldsArtifactLock(t, kv, d.cachekeyName(res))

	art := &mockArtifact{binary: "testapp", url: res.ArtifactURL}
	d.artifact = art

	if err := d.Run(); err != nil {
		t.Fatalf("a deferred tick must not surface an error, got %v", err)
	}
	if got := art.GetDownloadCount(); got != 0 {
		t.Errorf("want 0 downloads, got %d", got)
	}
}

func TestVerifyAgainstIndex_KeepsArtifactOnMatch(t *testing.T) {
	d, kv := newSharedCacheTestDewy(t)
	data := []byte("artifact bytes")
	sum := sha256.Sum256(data)
	if err := kv.Write("k", data); err != nil {
		t.Fatal(err)
	}

	if err := d.verifyAgainstIndex("k", data, &blobIndex{SHA256: hex.EncodeToString(sum[:])}); err != nil {
		t.Fatalf("matching digest should verify, got %v", err)
	}
	if !kv.Has("k") {
		t.Error("a verified artifact was dropped from the cache")
	}
}

func TestResolveCacheState_RejectsCorruptCachedArtifact(t *testing.T) {
	// The staging path reached through resolveCacheState has the same
	// exposure as the download path: the bytes come from a shared backend and
	// were verified by somebody else.
	d, kv := newSharedCacheTestDewy(t)
	res := testResponse()
	key := d.cachekeyName(res)
	peerPublishes(t, kv, key, []byte("artifact bytes"))
	if err := kv.Write(key, []byte("tampered bytes")); err != nil {
		t.Fatal(err)
	}

	if _, err := d.resolveCacheState(context.Background(), res); err == nil {
		t.Fatal("expected a digest mismatch to fail the tick")
	}
}

func TestResolveCacheState_AllowsCachedArtifactWithNoDigest(t *testing.T) {
	// An artifact staged by a Dewy release older than the index has nothing
	// to check against. Failing it would break the upgrade for every instance
	// whose cache was already warm.
	d, kv := newSharedCacheTestDewy(t)
	res := testResponse()
	key := d.cachekeyName(res)
	if err := kv.Write(key, []byte("artifact bytes")); err != nil {
		t.Fatal(err)
	}

	st, err := d.resolveCacheState(context.Background(), res)
	if err != nil {
		t.Fatalf("resolveCacheState: %v", err)
	}
	if !st.foundInCache {
		t.Error("want the artifact treated as cached")
	}
}

// newFleetMember returns a Dewy sharing kv with its peers but keeping its own
// local directory, the way two hosts share a bucket.
func newFleetMember(t *testing.T, kv *memCache) *Dewy {
	t.Helper()
	d := newPhaseTestDewy(t)
	view := sharedCacheView{memCache: kv, dir: t.TempDir()}
	d.cache = view
	locker, ok := cache.LockerFor(view, cache.WithLockHolderID(d.nodeID))
	if !ok {
		t.Fatal("a conditional-write cache should yield a locker")
	}
	d.locker = locker
	return d
}

func TestResolveCacheState_FollowerDeploysWhatALeaderPublished(t *testing.T) {
	// The deploy state pointer says what *this* instance is running. If it
	// lived in the shared cache, the leader publishing an artifact would look
	// to every follower like the deploy had already happened on their own
	// node, and they would skip it forever.
	kv := newMemCache(t.TempDir())
	leader := newFleetMember(t, kv)
	follower := newFleetMember(t, kv)
	res := testResponse()

	leader.artifact = &mockArtifact{binary: "testapp", url: res.ArtifactURL}
	if err := leader.downloadAndCache(context.Background(), res, cacheState{key: leader.cachekeyName(res)}); err != nil {
		t.Fatalf("leader download: %v", err)
	}

	st, err := follower.resolveCacheState(context.Background(), res)
	if err != nil {
		t.Fatalf("follower resolveCacheState: %v", err)
	}
	if st.skip {
		t.Error("the follower skipped a version it has never deployed")
	}
	if !st.foundInCache {
		t.Error("the follower should find the leader's artifact in the shared cache")
	}
}

func TestStateIsNotSharedBetweenInstances(t *testing.T) {
	kv := newMemCache(t.TempDir())
	a := newFleetMember(t, kv)
	b := newFleetMember(t, kv)

	if err := a.state().Write(currentkeyName, []byte("a-key")); err != nil {
		t.Fatal(err)
	}
	if _, err := b.state().Read(currentkeyName); !cache.IsNotFound(err) {
		t.Errorf("one instance's deploy state reached another: %v", err)
	}
}

func TestDownloadAndCache_IgnoresIndexWithoutADigest(t *testing.T) {
	// A truncated or tampered index must not be a way to get bytes deployed
	// without a check. Holding the lock, we can simply fetch them instead.
	d, kv := newSharedCacheTestDewy(t)
	res := testResponse()
	st := cacheState{key: d.cachekeyName(res)}
	if err := kv.Write(st.key, []byte("unverifiable bytes")); err != nil {
		t.Fatal(err)
	}
	if err := kv.Write(blobIndexKey(st.key), []byte(`{"sha256":"","size":0}`)); err != nil {
		t.Fatal(err)
	}

	art := &mockArtifact{binary: "testapp", url: res.ArtifactURL}
	d.artifact = art

	if err := d.downloadAndCache(context.Background(), res, st); err != nil {
		t.Fatalf("downloadAndCache: %v", err)
	}
	if got := art.GetDownloadCount(); got != 1 {
		t.Errorf("want 1 download when the index carries no digest, got %d", got)
	}
}

func TestDownloadAndCache_DownloadsWhenTheIndexOutlivedItsArtifact(t *testing.T) {
	// An index left behind by bytes that are gone must not wedge the deploy;
	// every tick would otherwise fail until somebody removed it by hand.
	d, kv := newSharedCacheTestDewy(t)
	res := testResponse()
	st := cacheState{key: d.cachekeyName(res)}
	peerPublishes(t, kv, st.key, []byte("artifact bytes"))
	if err := kv.Delete(st.key); err != nil {
		t.Fatal(err)
	}

	art := &mockArtifact{binary: "testapp", url: res.ArtifactURL}
	d.artifact = art

	if err := d.downloadAndCache(context.Background(), res, st); err != nil {
		t.Fatalf("downloadAndCache: %v", err)
	}
	if got := art.GetDownloadCount(); got != 1 {
		t.Errorf("want 1 download when the indexed bytes are gone, got %d", got)
	}
}

func TestDownloadAndCache_DownloadsWhenTheIndexCannotBeRead(t *testing.T) {
	d, kv := newSharedCacheTestDewy(t)
	res := testResponse()
	st := cacheState{key: d.cachekeyName(res)}
	kv.readErrs[blobIndexKey(st.key)] = errors.New("backend unhappy")

	art := &mockArtifact{binary: "testapp", url: res.ArtifactURL}
	d.artifact = art

	if err := d.downloadAndCache(context.Background(), res, st); err != nil {
		t.Fatalf("downloadAndCache: %v", err)
	}
	if got := art.GetDownloadCount(); got != 1 {
		t.Errorf("want 1 download when the index cannot be read, got %d", got)
	}
}

func TestResolveCacheState_UnreadableIndexFailsRatherThanSkipsVerification(t *testing.T) {
	// A record we cannot read is not a record that was never written. Treating
	// the two alike would make a malformed index a way around the check.
	d, kv := newSharedCacheTestDewy(t)
	res := testResponse()
	key := d.cachekeyName(res)
	if err := kv.Write(key, []byte("artifact bytes")); err != nil {
		t.Fatal(err)
	}
	kv.readErrs[blobIndexKey(key)] = errors.New("backend unhappy")

	if _, err := d.resolveCacheState(context.Background(), res); err == nil {
		t.Fatal("expected an unreadable index to fail the tick")
	}
}
