package cache

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/linyows/dewy/internal/sysdeps/fake"
)

// memAtomic is an in-memory AtomicCache used to drive CASLocker without a
// network backend. Versions are monotonic counters, mirroring the way S3
// ETags and GCS generations change on every write.
type memAtomic struct {
	mu       sync.Mutex
	store    map[string][]byte
	versions map[string]int64
	// writeErr, when set, makes every conditional write fail with it.
	writeErr error
}

func newMemAtomic() *memAtomic {
	return &memAtomic{store: map[string][]byte{}, versions: map[string]int64{}}
}

func (m *memAtomic) Read(key string) ([]byte, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	v, ok := m.store[key]
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrNotFound, key)
	}
	return append([]byte(nil), v...), nil
}

func (m *memAtomic) Write(key string, data []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.store[key] = append([]byte(nil), data...)
	m.versions[key]++
	return nil
}

func (m *memAtomic) Delete(key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.store, key)
	delete(m.versions, key)
	return nil
}

func (m *memAtomic) List() ([]string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	keys := make([]string, 0, len(m.store))
	for k := range m.store {
		keys = append(keys, k)
	}
	return keys, nil
}

func (m *memAtomic) GetDir() string             { return "" }
func (m *memAtomic) RegistryTTL() time.Duration { return 0 }

func (m *memAtomic) ReadWithVersion(key string) ([]byte, string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	v, ok := m.store[key]
	if !ok {
		return nil, "", fmt.Errorf("%w: %s", ErrNotFound, key)
	}
	return append([]byte(nil), v...), strconv.FormatInt(m.versions[key], 10), nil
}

func (m *memAtomic) WriteIfMatch(key, version string, data []byte) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.writeErr != nil {
		return "", m.writeErr
	}
	current := ""
	if g := m.versions[key]; g > 0 {
		current = strconv.FormatInt(g, 10)
	}
	if version != current {
		return "", fmt.Errorf("%w: %s", ErrConflict, key)
	}
	m.store[key] = append([]byte(nil), data...)
	m.versions[key]++
	return strconv.FormatInt(m.versions[key], 10), nil
}

var _ AtomicCache = (*memAtomic)(nil)

// newTestLocker returns a CASLocker on a shared store, driven by clk.
func newTestLocker(store *memAtomic, clk *fake.Clock, holder string) *CASLocker {
	return NewCASLocker(store,
		WithLockClock(clk),
		WithLockHolderID(holder),
		WithLockRetryWait(time.Millisecond))
}

func newTestClock() *fake.Clock {
	return fake.NewClock(time.Date(2026, 9, 15, 0, 0, 0, 0, time.UTC))
}

func TestCASLockerExclusion(t *testing.T) {
	store := newMemAtomic()
	clk := newTestClock()
	a := newTestLocker(store, clk, "node-a")
	b := newTestLocker(store, clk, "node-b")

	held, err := a.Acquire(context.Background(), "refresh", LockOptions{TTL: time.Minute})
	if err != nil {
		t.Fatalf("a.Acquire: %v", err)
	}

	if _, err := b.Acquire(context.Background(), "refresh", LockOptions{TTL: time.Minute, TryOnce: true}); !errors.Is(err, ErrLockBusy) {
		t.Fatalf("b.Acquire while held: want ErrLockBusy, got %v", err)
	}

	if err := held.Unlock(); err != nil {
		t.Fatalf("Unlock: %v", err)
	}

	after, err := b.Acquire(context.Background(), "refresh", LockOptions{TTL: time.Minute, TryOnce: true})
	if err != nil {
		t.Fatalf("b.Acquire after release: %v", err)
	}
	if err := after.Unlock(); err != nil {
		t.Fatalf("Unlock: %v", err)
	}
}

func TestCASLockerDistinctNamesDoNotContend(t *testing.T) {
	store := newMemAtomic()
	clk := newTestClock()
	l := newTestLocker(store, clk, "node-a")

	first, err := l.Acquire(context.Background(), "registry/aaa", LockOptions{TTL: time.Minute})
	if err != nil {
		t.Fatalf("first: %v", err)
	}
	defer first.Unlock() //nolint:errcheck // best effort in test cleanup

	second, err := l.Acquire(context.Background(), "registry/bbb", LockOptions{TTL: time.Minute, TryOnce: true})
	if err != nil {
		t.Fatalf("second lock name should be independent, got %v", err)
	}
	if err := second.Unlock(); err != nil {
		t.Fatalf("Unlock: %v", err)
	}
}

func TestCASLockerExpiredLockIsClaimable(t *testing.T) {
	// A holder that crashes never calls Unlock. The lock must become
	// claimable once its TTL has elapsed, or a single crash would wedge the
	// whole fleet.
	store := newMemAtomic()
	clk := newTestClock()
	a := newTestLocker(store, clk, "node-a")
	b := newTestLocker(store, clk, "node-b")

	if _, err := a.Acquire(context.Background(), "refresh", LockOptions{TTL: 30 * time.Second}); err != nil {
		t.Fatalf("a.Acquire: %v", err)
	}
	// "node-a" is gone; it never releases.

	if _, err := b.Acquire(context.Background(), "refresh", LockOptions{TTL: 30 * time.Second, TryOnce: true}); !errors.Is(err, ErrLockBusy) {
		t.Fatalf("within TTL: want ErrLockBusy, got %v", err)
	}

	clk.Advance(31 * time.Second)

	took, err := b.Acquire(context.Background(), "refresh", LockOptions{TTL: 30 * time.Second, TryOnce: true})
	if err != nil {
		t.Fatalf("after TTL: want acquisition, got %v", err)
	}
	if err := took.Unlock(); err != nil {
		t.Fatalf("Unlock: %v", err)
	}
}

func TestCASLockerWaitTimesOut(t *testing.T) {
	// Both lockers run on the real clock so that Wait actually elapses and
	// the holder's expiry is measured against the same time base.
	store := newMemAtomic()
	a := NewCASLocker(store, WithLockHolderID("node-a"), WithLockRetryWait(time.Millisecond))
	b := NewCASLocker(store, WithLockHolderID("node-b"), WithLockRetryWait(time.Millisecond))

	if _, err := a.Acquire(context.Background(), "refresh", LockOptions{TTL: time.Hour}); err != nil {
		t.Fatalf("a.Acquire: %v", err)
	}

	start := time.Now()
	_, err := b.Acquire(context.Background(), "refresh", LockOptions{TTL: time.Hour, Wait: 20 * time.Millisecond})
	if !errors.Is(err, ErrLockBusy) {
		t.Fatalf("want ErrLockBusy after Wait, got %v", err)
	}
	if elapsed := time.Since(start); elapsed < 20*time.Millisecond {
		t.Errorf("Acquire returned after %v, expected it to wait at least 20ms", elapsed)
	}
}

func TestCASLockerWaitSucceedsAfterRelease(t *testing.T) {
	store := newMemAtomic()
	a := NewCASLocker(store, WithLockHolderID("node-a"), WithLockRetryWait(time.Millisecond))
	b := NewCASLocker(store, WithLockHolderID("node-b"), WithLockRetryWait(time.Millisecond))

	held, err := a.Acquire(context.Background(), "refresh", LockOptions{TTL: time.Hour})
	if err != nil {
		t.Fatalf("a.Acquire: %v", err)
	}
	go func() {
		time.Sleep(10 * time.Millisecond)
		_ = held.Unlock()
	}()

	got, err := b.Acquire(context.Background(), "refresh", LockOptions{TTL: time.Hour, Wait: 2 * time.Second})
	if err != nil {
		t.Fatalf("b.Acquire should succeed once a releases, got %v", err)
	}
	if err := got.Unlock(); err != nil {
		t.Fatalf("Unlock: %v", err)
	}
}

func TestCASLockerContextCancel(t *testing.T) {
	store := newMemAtomic()
	a := NewCASLocker(store, WithLockHolderID("node-a"), WithLockRetryWait(time.Millisecond))
	b := NewCASLocker(store, WithLockHolderID("node-b"), WithLockRetryWait(time.Millisecond))

	if _, err := a.Acquire(context.Background(), "refresh", LockOptions{TTL: time.Hour}); err != nil {
		t.Fatalf("a.Acquire: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(10 * time.Millisecond)
		cancel()
	}()

	// Wait == 0 means "block until ctx is done".
	_, err := b.Acquire(ctx, "refresh", LockOptions{TTL: time.Hour})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("want context.Canceled, got %v", err)
	}
}

func TestCASLockerSingleWinnerUnderContention(t *testing.T) {
	// The property the whole design rests on: N instances racing for the
	// same lock produce exactly one holder.
	const contenders = 16
	store := newMemAtomic()

	var wg sync.WaitGroup
	var mu sync.Mutex
	winners := 0
	busy := 0

	for i := range contenders {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			l := NewCASLocker(store, WithLockHolderID(fmt.Sprintf("node-%d", i)))
			lk, err := l.Acquire(context.Background(), "refresh", LockOptions{TTL: time.Hour, TryOnce: true})
			mu.Lock()
			defer mu.Unlock()
			switch {
			case err == nil:
				winners++
				_ = lk
			case errors.Is(err, ErrLockBusy):
				busy++
			default:
				t.Errorf("unexpected error: %v", err)
			}
		}(i)
	}
	wg.Wait()

	if winners != 1 {
		t.Errorf("want exactly 1 lock holder, got %d (busy=%d)", winners, busy)
	}
}

func TestCASLockerLostFiresOnTTL(t *testing.T) {
	store := newMemAtomic()
	clk := newTestClock()
	l := newTestLocker(store, clk, "node-a")

	lk, err := l.Acquire(context.Background(), "refresh", LockOptions{TTL: 30 * time.Second})
	if err != nil {
		t.Fatalf("Acquire: %v", err)
	}

	select {
	case <-lk.Lost():
		t.Fatal("Lost fired while the lock was still held")
	default:
	}

	clk.Advance(31 * time.Second)

	select {
	case <-lk.Lost():
	case <-time.After(time.Second):
		t.Fatal("Lost did not fire after the TTL elapsed")
	}
}

func TestCASLockerLostFiresOnUnlock(t *testing.T) {
	store := newMemAtomic()
	l := NewCASLocker(store, WithLockHolderID("node-a"))

	lk, err := l.Acquire(context.Background(), "refresh", LockOptions{TTL: time.Hour})
	if err != nil {
		t.Fatalf("Acquire: %v", err)
	}
	if err := lk.Unlock(); err != nil {
		t.Fatalf("Unlock: %v", err)
	}

	select {
	case <-lk.Lost():
	case <-time.After(time.Second):
		t.Fatal("Lost did not fire after Unlock")
	}
}

func TestCASLockerUnlockIsIdempotent(t *testing.T) {
	store := newMemAtomic()
	l := NewCASLocker(store, WithLockHolderID("node-a"))

	lk, err := l.Acquire(context.Background(), "refresh", LockOptions{TTL: time.Hour})
	if err != nil {
		t.Fatalf("Acquire: %v", err)
	}
	if err := lk.Unlock(); err != nil {
		t.Fatalf("first Unlock: %v", err)
	}
	if err := lk.Unlock(); err != nil {
		t.Fatalf("second Unlock should be a no-op, got %v", err)
	}
}

func TestCASLockerUnlockAfterTakeoverDoesNotSteal(t *testing.T) {
	// A slow holder whose TTL expired must not clobber its successor's lock
	// record when it finally calls Unlock.
	store := newMemAtomic()
	clk := newTestClock()
	slow := newTestLocker(store, clk, "node-slow")
	next := newTestLocker(store, clk, "node-next")

	stale, err := slow.Acquire(context.Background(), "refresh", LockOptions{TTL: 30 * time.Second})
	if err != nil {
		t.Fatalf("slow.Acquire: %v", err)
	}

	clk.Advance(31 * time.Second)
	if _, err := next.Acquire(context.Background(), "refresh", LockOptions{TTL: 30 * time.Second, TryOnce: true}); err != nil {
		t.Fatalf("next.Acquire: %v", err)
	}

	if err := stale.Unlock(); err != nil {
		t.Fatalf("late Unlock should be discarded silently, got %v", err)
	}

	// The successor must still hold it.
	third := newTestLocker(store, clk, "node-third")
	if _, err := third.Acquire(context.Background(), "refresh", LockOptions{TTL: 30 * time.Second, TryOnce: true}); !errors.Is(err, ErrLockBusy) {
		t.Fatalf("late Unlock released the successor's lock: got %v", err)
	}
}

func TestCASLockerRejectsSemaphore(t *testing.T) {
	l := NewCASLocker(newMemAtomic())
	if _, err := l.Acquire(context.Background(), "refresh", LockOptions{TTL: time.Hour, Limit: 2}); err == nil {
		t.Fatal("expected Limit > 1 to be rejected by the CAS locker")
	}
}

func TestCASLockerBackendErrorSurfaces(t *testing.T) {
	store := newMemAtomic()
	store.writeErr = errors.New("backend down")
	l := NewCASLocker(store, WithLockHolderID("node-a"))

	_, err := l.Acquire(context.Background(), "refresh", LockOptions{TTL: time.Hour, TryOnce: true})
	if err == nil {
		t.Fatal("expected backend failure to surface")
	}
	if errors.Is(err, ErrLockBusy) {
		t.Errorf("backend failure must not be reported as contention: %v", err)
	}
}

func TestCASLockerCorruptRecordIsClaimable(t *testing.T) {
	store := newMemAtomic()
	if err := store.Write(lockKeyPrefix+"refresh", []byte("{not json")); err != nil {
		t.Fatal(err)
	}
	l := NewCASLocker(store, WithLockHolderID("node-a"))

	lk, err := l.Acquire(context.Background(), "refresh", LockOptions{TTL: time.Hour, TryOnce: true})
	if err != nil {
		t.Fatalf("a corrupt lock record must not wedge the lock forever, got %v", err)
	}
	if err := lk.Unlock(); err != nil {
		t.Fatalf("Unlock: %v", err)
	}
}

func TestLockerFor(t *testing.T) {
	if _, ok := LockerFor(newMemAtomic()); !ok {
		t.Error("an AtomicCache should yield a CAS-based locker")
	}

	f := &File{}
	f.Default()
	if _, ok := LockerFor(f); ok {
		t.Error("the file backend supports no conditional writes and must not yield a locker")
	}
}

func TestLockerForPrefersNativeLocker(t *testing.T) {
	n := &nativeLockCache{memAtomic: newMemAtomic()}
	l, ok := LockerFor(n)
	if !ok {
		t.Fatal("expected a locker")
	}
	if _, isCAS := l.(*CASLocker); isCAS {
		t.Error("a backend implementing Locker must be used directly, not wrapped in the CAS emulation")
	}
}

// nativeLockCache stands in for a backend with its own lock primitive
// (Consul), to pin down LockerFor's preference order.
type nativeLockCache struct {
	*memAtomic
}

func (n *nativeLockCache) Acquire(_ context.Context, _ string, _ LockOptions) (Lock, error) {
	return nil, ErrLockBusy
}

var _ Locker = (*nativeLockCache)(nil)

func TestRemoveLocalLeavesTheSharedBackendAlone(t *testing.T) {
	// The cloud backends delete from the bucket as well as from local
	// staging, so an instance that finds its own staged copy bad needs a way
	// to discard it without taking the artifact away from every peer.
	dir := t.TempDir()
	f := &File{}
	f.Default()
	f.SetDir(dir)
	f.MaxSize = DefaultMaxSize

	if err := f.Write("blobs/app.json", []byte("staged")); err != nil {
		t.Fatalf("Write: %v", err)
	}
	if _, err := f.Read("blobs/app.json"); err != nil {
		t.Fatalf("Read: %v", err)
	}

	if err := RemoveLocal(f, "blobs/app.json"); err != nil {
		t.Fatalf("RemoveLocal: %v", err)
	}
	if _, err := f.Read("blobs/app.json"); !IsNotFound(err) {
		t.Errorf("staged copy survived RemoveLocal: %v", err)
	}
}

func TestRemoveLocalOnAMissingKeyIsNotAnError(t *testing.T) {
	f := &File{}
	f.Default()
	f.SetDir(t.TempDir())
	if err := RemoveLocal(f, "absent"); err != nil {
		t.Errorf("removing nothing should succeed, got %v", err)
	}
}

func TestFileWriteCreatesNestedKeyDirectories(t *testing.T) {
	// Keys name paths now ("blobs/<key>.json"), and OpenFile will not create
	// the directories along the way.
	f := &File{}
	f.Default()
	f.SetDir(t.TempDir())
	f.MaxSize = DefaultMaxSize

	if err := f.Write("blobs/nested/app.json", []byte("x")); err != nil {
		t.Fatalf("Write with a nested key: %v", err)
	}
	got, err := f.Read("blobs/nested/app.json")
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if string(got) != "x" {
		t.Errorf("got %q, want %q", got, "x")
	}
}

func TestFileReadReportsNotFound(t *testing.T) {
	// Callers distinguish "never written" from "could not be read", so the
	// file backend has to use the package's not-found sentinel.
	f := &File{}
	f.Default()
	f.SetDir(t.TempDir())
	if _, err := f.Read("absent"); !IsNotFound(err) {
		t.Errorf("want a not-found error, got %v", err)
	}
}
