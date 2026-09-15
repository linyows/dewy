package cache

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/linyows/dewy/internal/sysdeps"
)

// ErrLockBusy indicates that a lock could not be acquired because another
// instance holds it. Callers detect it with errors.Is.
var ErrLockBusy = errors.New("lock held by another instance")

// Locker is an optional capability for cache backends that can coordinate
// mutual exclusion across Dewy instances. It exists so that a fleet pointed
// at one registry issues a single upstream request instead of one per
// instance.
//
// Backends with a native session or lease primitive implement Locker
// directly. Backends that only support conditional writes get an equivalent
// implementation from NewCASLocker.
type Locker interface {
	// Acquire blocks until the named lock is held, opts.Wait elapses, or ctx
	// is done. It returns an error for which errors.Is(err, ErrLockBusy)
	// reports true when the lock could not be taken in time.
	//
	// name is a logical lock name, not a cache key; implementations namespace
	// it so that lock records cannot collide with cached data.
	Acquire(ctx context.Context, name string, opts LockOptions) (Lock, error)
}

// LockOptions configures a single Acquire call.
type LockOptions struct {
	// TTL bounds how long the lock is held without the holder proving it is
	// still alive. A holder that crashes loses the lock after TTL. Zero
	// selects DefaultLockTTL.
	TTL time.Duration
	// Wait bounds how long Acquire blocks before giving up with ErrLockBusy.
	// Zero means "wait until ctx is done".
	Wait time.Duration
	// TryOnce makes Acquire return ErrLockBusy immediately rather than
	// waiting for the holder to release.
	TryOnce bool
	// Limit turns the lock into a semaphore admitting Limit holders. Zero and
	// one both mean mutual exclusion. Backends that cannot express a
	// semaphore reject Limit > 1.
	Limit int
	// Value is informational metadata about the holder (node ID, tag) stored
	// alongside the lock so that operators can see who holds it.
	Value []byte
}

// Lock is a held lock. Callers release it with Unlock, and abort long
// critical sections when Lost is closed.
type Lock interface {
	// Lost is closed when the lock is no longer held: the TTL elapsed, the
	// underlying session was invalidated, or Unlock was called. Work that can
	// outlive the TTL must select on it rather than assuming the lock is
	// still held.
	Lost() <-chan struct{}
	// Unlock releases the lock. It is safe to call more than once.
	Unlock() error
}

// LockerFor returns a Locker for c: the backend's native implementation when
// it has one, a CAS-based equivalent when the backend supports conditional
// writes, and (nil, false) when the backend can coordinate neither. The
// options apply only when a CAS-based locker is constructed.
func LockerFor(c Cache, opts ...CASLockerOption) (Locker, bool) {
	if l, ok := c.(Locker); ok {
		return l, true
	}
	if ac, ok := c.(AtomicCache); ok {
		return NewCASLocker(ac, opts...), true
	}
	return nil, false
}

const (
	// lockKeyPrefix namespaces lock records away from cached data so that a
	// lock name can never collide with an artifact or registry cache key.
	lockKeyPrefix = "locks/"

	// DefaultLockTTL is used when LockOptions.TTL is zero.
	DefaultLockTTL = 30 * time.Second

	// defaultLockRetryWait is the back-off between attempts while waiting for
	// a peer to release a lock.
	defaultLockRetryWait = 250 * time.Millisecond
)

// lockRecord is the JSON shape of a CAS-based lock. An empty Holder means the
// lock is free; the record is rewritten rather than deleted on release so
// that the conditional write always has a version to match against.
type lockRecord struct {
	Holder     string    `json:"holder,omitempty"`
	Value      string    `json:"value,omitempty"`
	AcquiredAt time.Time `json:"acquired_at,omitempty"`
	ExpiresAt  time.Time `json:"expires_at,omitempty"`
}

// heldAt reports whether the record represents a lock that is still held at
// now. A nil receiver (no record yet) is not held.
func (r *lockRecord) heldAt(now time.Time) bool {
	return r != nil && r.Holder != "" && now.Before(r.ExpiresAt)
}

// CASLocker implements Locker on top of an AtomicCache's conditional writes.
//
// Liveness is inferred from a wall-clock expiry written into the lock record,
// not from a server-side session: a holder that crashes keeps the lock until
// its TTL elapses, and instances whose clocks disagree disagree about when
// that happens. Backends with a real session primitive should implement
// Locker themselves instead of using this.
//
// CASLocker does not support semaphores; Acquire rejects Limit > 1.
type CASLocker struct {
	cache  AtomicCache
	clock  sysdeps.Clock
	holder string
	retry  time.Duration
}

// CASLockerOption configures a CASLocker.
type CASLockerOption func(*CASLocker)

// WithLockClock injects a custom Clock. A nil clock is ignored so the default
// stays in effect.
func WithLockClock(c sysdeps.Clock) CASLockerOption {
	return func(l *CASLocker) {
		if c != nil {
			l.clock = c
		}
	}
}

// WithLockHolderID sets the identifier recorded as the lock holder. It is
// informational; an empty id is ignored.
func WithLockHolderID(id string) CASLockerOption {
	return func(l *CASLocker) {
		if id != "" {
			l.holder = id
		}
	}
}

// WithLockRetryWait sets the back-off between acquisition attempts. A
// non-positive duration is ignored.
func WithLockRetryWait(d time.Duration) CASLockerOption {
	return func(l *CASLocker) {
		if d > 0 {
			l.retry = d
		}
	}
}

// NewCASLocker returns a Locker backed by ac's conditional writes.
func NewCASLocker(ac AtomicCache, opts ...CASLockerOption) *CASLocker {
	l := &CASLocker{
		cache:  ac,
		clock:  sysdeps.RealClock(),
		holder: HolderID(sysdeps.RealEnv()),
		retry:  defaultLockRetryWait,
	}
	for _, opt := range opts {
		opt(l)
	}
	return l
}

// HolderID builds a "hostname:pid" lock-holder identifier from an Env.
// Hostname errors fall back to a placeholder: the identifier is
// informational, so a missing hostname must not prevent locking.
func HolderID(e sysdeps.Env) string {
	host, err := e.Hostname()
	if err != nil || host == "" {
		host = "unknown-host"
	}
	return host + ":" + strconv.Itoa(e.Pid())
}

// Acquire implements Locker.
func (l *CASLocker) Acquire(ctx context.Context, name string, opts LockOptions) (Lock, error) {
	if opts.Limit > 1 {
		return nil, fmt.Errorf("cache: CAS locker cannot express a semaphore (Limit=%d)", opts.Limit)
	}
	ttl := opts.TTL
	if ttl <= 0 {
		ttl = DefaultLockTTL
	}
	key := lockKeyPrefix + name

	var deadline time.Time
	if opts.Wait > 0 {
		deadline = l.clock.Now().Add(opts.Wait)
	}

	for {
		rec, version, err := l.readRecord(key)
		if err != nil && !IsNotFound(err) {
			return nil, fmt.Errorf("read lock %s: %w", name, err)
		}

		now := l.clock.Now()
		if !rec.heldAt(now) {
			newVersion, werr := l.writeRecord(key, version, &lockRecord{
				Holder:     l.holder,
				Value:      string(opts.Value),
				AcquiredAt: now,
				ExpiresAt:  now.Add(ttl),
			})
			if werr == nil {
				return newCASLock(l, key, newVersion, ttl), nil
			}
			if !IsConflict(werr) {
				return nil, fmt.Errorf("claim lock %s: %w", name, werr)
			}
			// A peer claimed it between our read and our write. Fall through
			// and wait like any other contender.
		}

		if opts.TryOnce {
			return nil, fmt.Errorf("%w: %s", ErrLockBusy, name)
		}
		if !deadline.IsZero() && !l.clock.Now().Before(deadline) {
			return nil, fmt.Errorf("%w: %s", ErrLockBusy, name)
		}
		if err := sleepCtx(ctx, l.clock, l.retry); err != nil {
			return nil, err
		}
	}
}

// readRecord reads and decodes the lock record. A missing key is reported
// with IsNotFound and a nil record, which Acquire treats as "free".
func (l *CASLocker) readRecord(key string) (*lockRecord, string, error) {
	data, version, err := l.cache.ReadWithVersion(key)
	if err != nil {
		return nil, "", err
	}
	rec := &lockRecord{}
	if err := json.Unmarshal(data, rec); err != nil {
		// A corrupt record must not wedge the fleet forever: treat it as free
		// and let the conditional write replace it.
		return nil, version, nil
	}
	return rec, version, nil
}

func (l *CASLocker) writeRecord(key, version string, rec *lockRecord) (string, error) {
	data, err := json.Marshal(rec)
	if err != nil {
		return "", err
	}
	return l.cache.WriteIfMatch(key, version, data)
}

// sleepCtx waits d on the given clock, returning early when ctx is done.
func sleepCtx(ctx context.Context, clock sysdeps.Clock, d time.Duration) error {
	t := clock.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C():
		return nil
	}
}

// casLock is a held CASLocker lock.
type casLock struct {
	locker  *CASLocker
	key     string
	version string

	lost      chan struct{}
	closeLost sync.Once
	stop      chan struct{}
	stopOnce  sync.Once
	unlocked  sync.Once
	unlockErr error
}

func newCASLock(l *CASLocker, key, version string, ttl time.Duration) *casLock {
	lk := &casLock{
		locker:  l,
		key:     key,
		version: version,
		lost:    make(chan struct{}),
		stop:    make(chan struct{}),
	}

	// Without a server-side session the only signal available is the expiry
	// we wrote ourselves: once the TTL elapses a peer may legitimately claim
	// the lock, so the holder must stop assuming it is exclusive.
	timer := l.clock.NewTimer(ttl)
	go func() {
		defer timer.Stop()
		select {
		case <-timer.C():
			lk.markLost()
		case <-lk.stop:
		}
	}()

	return lk
}

func (lk *casLock) markLost() {
	lk.closeLost.Do(func() { close(lk.lost) })
}

// Lost implements Lock.
func (lk *casLock) Lost() <-chan struct{} { return lk.lost }

// Unlock implements Lock. Releasing is a conditional write back to a free
// record: if a peer already claimed the lock after our TTL expired, the write
// conflicts and is discarded rather than stealing the lock back.
func (lk *casLock) Unlock() error {
	lk.unlocked.Do(func() {
		lk.stopOnce.Do(func() { close(lk.stop) })
		lk.markLost()

		if _, err := lk.locker.writeRecord(lk.key, lk.version, &lockRecord{}); err != nil {
			if IsConflict(err) {
				// The lock had already been taken over. Nothing to release.
				return
			}
			lk.unlockErr = fmt.Errorf("release lock %s: %w", lk.key, err)
		}
	})
	return lk.unlockErr
}

// Compile-time checks.
var (
	_ Locker = (*CASLocker)(nil)
	_ Lock   = (*casLock)(nil)
)
