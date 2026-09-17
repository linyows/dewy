package dewy

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/linyows/dewy/cache"
	"github.com/linyows/dewy/registry"
)

// sharedStore models the S3 and GCS backends: every key written by any
// instance is visible to all of them.
type sharedStore struct {
	mu    sync.Mutex
	store map[string][]byte
}

func newSharedStore() *sharedStore {
	return &sharedStore{store: map[string][]byte{}}
}

func (s *sharedStore) Read(key string) ([]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, ok := s.store[key]
	if !ok {
		return nil, fmt.Errorf("%w: %s", cache.ErrNotFound, key)
	}
	return append([]byte(nil), v...), nil
}

func (s *sharedStore) Write(key string, data []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.store[key] = append([]byte(nil), data...)
	return nil
}

func (s *sharedStore) Delete(key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.store, key)
	return nil
}

func (s *sharedStore) List() ([]string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	keys := make([]string, 0, len(s.store))
	for k := range s.store {
		keys = append(keys, k)
	}
	return keys, nil
}

func (s *sharedStore) Has(key string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, ok := s.store[key]
	return ok
}

func (s *sharedStore) GetDir() string             { return "" }
func (s *sharedStore) RegistryTTL() time.Duration { return 0 }

// instanceView is one instance's handle on the shared store: the same objects,
// its own local directory. Two views are two hosts pointed at one bucket.
type instanceView struct {
	*sharedStore
	dir string
}

func (v instanceView) GetDir() string { return v.dir }

var _ cache.Cache = instanceView{}

// newFleetMember returns a Dewy sharing store with its peers while keeping its
// own local directory.
func newFleetMember(t *testing.T, store *sharedStore, cmd Command) *Dewy {
	t.Helper()
	c := DefaultConfig()
	c.Command = cmd
	c.Registry = "ghr://linyows/dewy"
	c.Cache = CacheConfig{Type: FILE, Expiration: 10, URL: "file://" + t.TempDir()}
	d, err := New(c, testLogger())
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	d.cache = instanceView{sharedStore: store, dir: t.TempDir()}
	d.root = t.TempDir()
	return d
}

func stateTestResponse() *registry.CurrentResponse {
	return &registry.CurrentResponse{
		ID:          "id",
		Tag:         "v2.0.0",
		ArtifactURL: "https://example.com/testapp_linux_amd64.zip",
	}
}

// publishAsPeer puts an artifact in the shared cache and records the deploy in
// the publishing instance's own state, the way a real deploy does.
func publishAsPeer(t *testing.T, peer *Dewy, res *registry.CurrentResponse) string {
	t.Helper()
	key := peer.cachekeyName(res)
	if err := peer.cache.Write(key, []byte("artifact bytes")); err != nil {
		t.Fatalf("publish artifact: %v", err)
	}
	if err := peer.state().Write(currentkeyName, []byte(key)); err != nil {
		t.Fatalf("record deploy: %v", err)
	}
	return key
}

func TestResolveCacheState_PeerDeployDoesNotSkipOurs_Server(t *testing.T) {
	// One instance deploying a release must not make its peers believe they
	// have deployed it. They would skip it silently, and keep skipping every
	// release after it.
	store := newSharedStore()
	res := stateTestResponse()

	leader := newFleetMember(t, store, SERVER)
	publishAsPeer(t, leader, res)

	follower := newFleetMember(t, store, SERVER)
	follower.isServerRunning = true // already serving an older release

	st, err := follower.resolveCacheState(context.Background(), res)
	if err != nil {
		t.Fatalf("resolveCacheState: %v", err)
	}
	if st.skip {
		t.Error("the follower skipped a release it has never deployed")
	}
	if !st.foundInCache {
		t.Error("the follower should still find the peer's artifact in the shared cache")
	}
}

func TestResolveCacheState_PeerDeployDoesNotSkipOurs_Assets(t *testing.T) {
	// Assets mode has no running-server state to fall through on, so it skips
	// unconditionally when the pointer matches.
	store := newSharedStore()
	res := stateTestResponse()

	leader := newFleetMember(t, store, ASSETS)
	publishAsPeer(t, leader, res)

	follower := newFleetMember(t, store, ASSETS)
	st, err := follower.resolveCacheState(context.Background(), res)
	if err != nil {
		t.Fatalf("resolveCacheState: %v", err)
	}
	if st.skip {
		t.Error("the follower skipped a release it has never deployed")
	}
}

func TestResolveCacheState_OurOwnDeployStillSkips(t *testing.T) {
	// The skip itself must keep working: an instance that has deployed the
	// release and is serving it does nothing on the next tick.
	store := newSharedStore()
	res := stateTestResponse()

	d := newFleetMember(t, store, SERVER)
	publishAsPeer(t, d, res)
	d.isServerRunning = true

	st, err := d.resolveCacheState(context.Background(), res)
	if err != nil {
		t.Fatalf("resolveCacheState: %v", err)
	}
	if !st.skip {
		t.Error("an instance already serving this release should skip")
	}
}

func TestDeployStateIsNotSharedBetweenInstances(t *testing.T) {
	store := newSharedStore()
	a := newFleetMember(t, store, SERVER)
	b := newFleetMember(t, store, SERVER)

	if err := a.state().Write(currentkeyName, []byte("a-key")); err != nil {
		t.Fatal(err)
	}
	if err := a.state().Write(blockedkeyName, []byte("a-blocked")); err != nil {
		t.Fatal(err)
	}

	if _, err := b.state().Read(currentkeyName); err == nil {
		t.Error("one instance's current pointer reached another")
	}
	if _, err := b.state().Read(blockedkeyName); err == nil {
		t.Error("one instance's blocked marker reached another")
	}
	if store.Has(currentkeyName) || store.Has(blockedkeyName) {
		t.Error("deploy state was written to the shared cache backend")
	}
}

func TestStateFollowsTheCacheDirectory(t *testing.T) {
	// The state store is derived from the cache on each use rather than
	// snapshotted, so swapping the cache cannot leave the two pointing at
	// different directories.
	store := newSharedStore()
	d := newFleetMember(t, store, SERVER)

	if err := d.state().Write(currentkeyName, []byte("first")); err != nil {
		t.Fatal(err)
	}

	d.cache = instanceView{sharedStore: store, dir: t.TempDir()}
	if _, err := d.state().Read(currentkeyName); err == nil {
		t.Error("the state store did not follow the cache to its new directory")
	}
}
