package registry

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"runtime"
	"strconv"
	"time"

	"github.com/linyows/dewy/cache"
	"github.com/linyows/dewy/internal/sysdeps"
	"github.com/linyows/dewy/logging"
)

// registryCacheKeyPrefix is the cache-key prefix under which Cached stores
// upstream registry responses. The actual key per Cached instance includes a
// hash of the registry URL and the local platform so that two Dewy clusters
// or heterogeneous nodes that happen to share a cache prefix do not
// overwrite each other with mismatched artifact metadata.
const registryCacheKeyPrefix = "registry-cache/"

// defaultRefreshWait is the back-off between cache re-reads while waiting
// on a peer to finish refreshing.
const defaultRefreshWait = 250 * time.Millisecond

// cachedEntry is the on-disk JSON shape of the shared registry-result cache.
type cachedEntry struct {
	Response  *CurrentResponse `json:"response,omitempty"`
	FetchedAt time.Time        `json:"fetched_at"`
	// LockedAt and LockedBy are how Dewy releases before the cache.Locker
	// existed marked an in-progress refresh. They are no longer written, but
	// they are still honored on read so that a fleet mid-upgrade does not
	// stampede upstream: a new instance still waits for an old peer that
	// claimed the entry this way.
	LockedAt time.Time `json:"locked_at"`
	LockedBy string    `json:"locked_by,omitempty"`
}

// Cached wraps an upstream Registry with a shared, TTL-based result cache.
//
// Multiple Dewy instances sharing the same AtomicCache prefix coordinate so
// that only one of them calls the upstream registry per TTL window. Other
// instances read the cached response from the shared cache.
//
// Refresh is serialized by a cache.Locker: the leader holds the lock while it
// calls upstream and publishes the result with a conditional write. Followers
// that find the lock taken back off briefly and re-read; if the entry is
// still stale once the lock TTL elapses they fall back to the last known
// response (stale-but-usable).
type Cached struct {
	inner    Registry
	cache    cache.AtomicCache
	ttl      time.Duration
	lockTTL  time.Duration
	wait     time.Duration
	logger   *logging.Logger
	clock    sysdeps.Clock
	nodeID   string
	cacheKey string
	// locker serializes the upstream refresh across instances. It is derived
	// from the cache backend: a native distributed lock when the backend has
	// one, a conditional-write emulation otherwise.
	locker   cache.Locker
	lockName string
}

// CachedOption configures optional dependencies of a Cached registry.
type CachedOption func(*Cached)

// WithClock injects a custom Clock. Defaults to sysdeps.RealClock.
// A nil clock is ignored so the default stays in effect.
func WithClock(c sysdeps.Clock) CachedOption {
	return func(x *Cached) {
		if c != nil {
			x.clock = c
		}
	}
}

// WithEnv injects a custom Env. The Env is consulted at NewCached time to
// derive the refresh-lock node ID (hostname:pid); after construction the
// node ID is frozen. Defaults to sysdeps.RealEnv. A nil env is ignored.
func WithEnv(e sysdeps.Env) CachedOption {
	return func(x *Cached) {
		if e != nil {
			x.nodeID = nodeIDFrom(e)
		}
	}
}

// WithLocker injects the Locker used to serialize upstream refreshes.
// Defaults to the cache backend's own lock when it has one, and to a
// conditional-write lock otherwise. A nil locker is ignored.
func WithLocker(l cache.Locker) CachedOption {
	return func(x *Cached) {
		if l != nil {
			x.locker = l
		}
	}
}

// NewCached wraps inner with a shared registry-result cache backed by
// atomicCache. ttl controls how long a cached response is considered fresh.
//
// scope is an opaque identifier used to derive the cache key — typically the
// upstream registry URL. Two Cached instances that share an AtomicCache
// prefix coordinate single-flight refresh only when they pass the same scope
// (and run on the same platform), preventing instances with different
// registry URLs or differing OS/arch from overwriting each other's entries.
func NewCached(inner Registry, scope string, atomicCache cache.AtomicCache, ttl time.Duration, log *logging.Logger, opts ...CachedOption) *Cached {
	c := &Cached{
		inner:    inner,
		cache:    atomicCache,
		ttl:      ttl,
		lockTTL:  maxLockTTL(ttl),
		wait:     defaultRefreshWait,
		logger:   log,
		clock:    sysdeps.RealClock(),
		nodeID:   nodeIDFrom(sysdeps.RealEnv()),
		cacheKey: cacheKeyForScope(scope),
		lockName: lockNameForScope(scope),
	}
	for _, opt := range opts {
		opt(c)
	}
	// Built after the options so the locker inherits the injected clock and
	// node ID rather than the real ones.
	if c.locker == nil {
		c.locker, _ = cache.LockerFor(atomicCache,
			cache.WithLockClock(c.clock),
			cache.WithLockHolderID(c.nodeID))
	}
	return c
}

// nodeIDFrom builds the per-process refresh-lock identifier from an Env.
// Hostname errors fall back to a stable placeholder so the lock can still be
// claimed; the lock owner is informational only.
func nodeIDFrom(e sysdeps.Env) string {
	host, err := e.Hostname()
	if err != nil || host == "" {
		host = "unknown-host"
	}
	return host + ":" + strconv.Itoa(e.Pid())
}

// cacheKeyForScope returns the cache key used by a Cached instance with the
// given scope. The local OS/arch are folded in because Current() responses
// vary by platform (artifact name selection). The scope is canonicalized so
// that registry URLs that differ only in query-parameter ordering hash to
// the same key.
func cacheKeyForScope(scope string) string {
	return registryCacheKeyPrefix + scopeHash(scope) + ".json"
}

// lockNameForScope returns the Locker name guarding the upstream refresh for
// the given scope. It is derived from the same hash as the cache key so that
// the lock and the entry it protects always travel together.
func lockNameForScope(scope string) string {
	return "registry/" + scopeHash(scope)
}

func scopeHash(scope string) string {
	h := sha256.Sum256([]byte(canonicalizeScope(scope) + "|" + runtime.GOOS + "|" + runtime.GOARCH))
	return hex.EncodeToString(h[:8])
}

// canonicalizeScope returns scope with URL query parameters sorted by key
// (via url.Values.Encode), so that ?a=1&b=2 and ?b=2&a=1 produce the same
// canonical form. Non-URL scopes are returned unchanged.
func canonicalizeScope(scope string) string {
	u, err := url.Parse(scope)
	if err != nil {
		return scope
	}
	if u.RawQuery != "" {
		u.RawQuery = u.Query().Encode()
	}
	return u.String()
}

// maxLockTTL is the time after which an abandoned refresh lock is considered
// stale and may be claimed by another node. Generous enough to absorb a slow
// upstream call but bounded so a crashed leader does not block forever.
func maxLockTTL(ttl time.Duration) time.Duration {
	d := ttl * 2
	if d < 30*time.Second {
		return 30 * time.Second
	}
	if d > 5*time.Minute {
		return 5 * time.Minute
	}
	return d
}

// Current returns the latest registry response, possibly served from the
// shared cache.
//
// The loop is bounded by lockTTL rather than a fixed retry count: while a
// peer holds the refresh lock we keep waiting, so that we do not stampede
// upstream just because the peer's refresh takes longer than a few hundred
// milliseconds. Once lockTTL elapses we stop waiting and serve what we have.
func (c *Cached) Current(ctx context.Context) (*CurrentResponse, error) {
	deadline := c.clock.Now().Add(c.lockTTL + c.wait)

	for {
		entry, version, err := c.readEntry()
		if err != nil && !cache.IsNotFound(err) {
			c.warn("failed to read shared registry cache", err)
			return c.inner.Current(ctx)
		}

		// Fresh hit — return without contacting upstream.
		if entry != nil && c.isFresh(entry) {
			return entry.Response, nil
		}

		// A pre-Locker peer marked the entry as being refreshed. Wait it out
		// the way that peer expects, up to lockTTL.
		if entry != nil && c.isLocked(entry) && c.clock.Now().Before(deadline) {
			if err := c.sleepCtx(ctx, c.wait); err != nil {
				if entry.Response != nil {
					return entry.Response, nil
				}
				return nil, err
			}
			continue
		}

		// Stale or absent. Exactly one instance gets to call upstream.
		lock, err := c.acquire(ctx)
		switch {
		case errors.Is(err, cache.ErrLockBusy):
			// A peer is refreshing. Re-read shortly: it will publish and we
			// will take the fresh hit above.
			if c.clock.Now().Before(deadline) {
				if serr := c.sleepCtx(ctx, c.wait); serr != nil {
					if entry != nil && entry.Response != nil {
						return entry.Response, nil
					}
					return nil, serr
				}
				continue
			}
			// The peer kept the lock alive for our whole wait without
			// publishing, so it is slow rather than dead: adding our own
			// request to whatever it is struggling with would not help.
			//
			// Only a locker that renews its lease reaches this branch. Under
			// CASLocker the lock simply expires and the next caller takes it
			// over, which is the pre-Locker behavior.
			if entry != nil && entry.Response != nil {
				c.warn("peer held the refresh lock past its TTL; serving stale cache", err)
				return entry.Response, nil
			}
			// Nothing cached at all: a cold-start fleet must not deadlock
			// waiting for a peer, so fall back to a direct upstream call.
			return c.inner.Current(ctx)

		case err != nil:
			// The lock backend is unavailable. Degrade to uncoordinated
			// behavior — every instance polls upstream for itself — rather
			// than to a frozen one. Serving the cached response here instead
			// would mean a locker that stays broken stops the fleet from ever
			// seeing another release.
			c.warn("failed to acquire registry refresh lock", err)
			res, uerr := c.inner.Current(ctx)
			if uerr == nil {
				return res, nil
			}
			if entry != nil && entry.Response != nil {
				c.warn("upstream registry failed; serving stale cache", uerr)
				return entry.Response, nil
			}
			return nil, uerr
		}

		// We hold the lock. Re-read before calling upstream: a peer may have
		// published between our read and our acquisition, and its result is
		// the one we were about to go and fetch.
		if latest, latestVersion, rerr := c.readEntry(); rerr == nil || cache.IsNotFound(rerr) {
			if latest != nil && c.isFresh(latest) {
				c.release(lock)
				return latest.Response, nil
			}
			entry, version = latest, latestVersion
		}

		res, rerr := c.refreshAndPublish(ctx, entry, version, lock.Lost())
		c.release(lock)
		return res, rerr
	}
}

// lockLost reports whether the refresh lock has been lost. A nil channel —
// a lock with no liveness signal at all — never reports a loss.
func lockLost(lost <-chan struct{}) bool {
	select {
	case <-lost:
		return true
	default:
		return false
	}
}

// release drops the refresh lock, logging rather than propagating a failure:
// the lock TTL bounds the damage, and the caller has a response to return.
func (c *Cached) release(lock cache.Lock) {
	if err := lock.Unlock(); err != nil {
		c.warn("failed to release registry refresh lock", err)
	}
}

// acquire takes the refresh lock without waiting. Waiting is the caller's
// job: it re-reads the entry between attempts, so that a peer publishing a
// fresh response ends the wait immediately instead of at lock release.
func (c *Cached) acquire(ctx context.Context) (cache.Lock, error) {
	if c.locker == nil {
		return nil, fmt.Errorf("no locker available for cache backend")
	}
	return c.locker.Acquire(ctx, c.lockName, cache.LockOptions{
		TTL:     c.lockTTL,
		TryOnce: true,
		Value:   []byte(c.nodeID),
	})
}

// sleepCtx waits d using the injected clock, returning early on ctx cancel.
func (c *Cached) sleepCtx(ctx context.Context, d time.Duration) error {
	t := c.clock.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C():
		return nil
	}
}

// Report passes through to the underlying registry. The audit upload is not
// cached because each instance must record its own deployment.
func (c *Cached) Report(ctx context.Context, req *ReportRequest) error {
	return c.inner.Report(ctx, req)
}

// readEntry reads and decodes the shared cache entry. Returns
// (nil, "", IsNotFound) when the entry does not exist yet.
func (c *Cached) readEntry() (*cachedEntry, string, error) {
	data, version, err := c.cache.ReadWithVersion(c.cacheKey)
	if err != nil {
		return nil, "", err
	}
	entry := &cachedEntry{}
	if err := json.Unmarshal(data, entry); err != nil {
		return nil, version, fmt.Errorf("decode cache entry: %w", err)
	}
	return entry, version, nil
}

// writeEntry encodes and writes the entry with a CAS condition on version.
func (c *Cached) writeEntry(entry *cachedEntry, version string) (string, error) {
	data, err := json.Marshal(entry)
	if err != nil {
		return "", err
	}
	return c.cache.WriteIfMatch(c.cacheKey, version, data)
}

func (c *Cached) isFresh(entry *cachedEntry) bool {
	return entry.Response != nil && c.clock.Now().Sub(entry.FetchedAt) < c.ttl
}

// isLocked reports whether a pre-Locker peer has marked the entry as being
// refreshed. Current Dewy never writes these fields; see cachedEntry.
func (c *Cached) isLocked(entry *cachedEntry) bool {
	return !entry.LockedAt.IsZero() && c.clock.Now().Sub(entry.LockedAt) < c.lockTTL
}

// refreshAndPublish calls the upstream registry and publishes the result with
// a conditional write. On upstream failure it publishes nothing, leaving the
// previous entry in place so the cache continues to serve stale-but-usable.
//
// lost is the refresh lock's liveness channel. An upstream call that outlives
// the lock is still returned to our own caller, but it is not published: by
// then a successor holds the lock, and publishing would stamp a response
// fetched up to a lock TTL ago with a fresh FetchedAt. The conditional write
// prevents the successor's entry from being clobbered either way; this check
// also keeps the successor's newer result from being discarded in favor of
// ours.
func (c *Cached) refreshAndPublish(ctx context.Context, prev *cachedEntry, version string, lost <-chan struct{}) (*CurrentResponse, error) {
	res, err := c.inner.Current(ctx)
	if err != nil {
		if prev != nil && prev.Response != nil {
			c.warn("upstream registry failed; serving stale cache", err)
			return prev.Response, nil
		}
		return nil, err
	}
	if c.logger != nil {
		c.logger.Info("Registry result refreshed from upstream",
			slog.String("node", c.nodeID))
	}

	if lockLost(lost) {
		if c.logger != nil {
			c.logger.Warn("Refresh lock expired before the result could be published",
				slog.String("node", c.nodeID))
		}
		return res, nil
	}

	final := &cachedEntry{
		Response:  res,
		FetchedAt: c.clock.Now(),
		// LockedAt zero — released.
	}
	if _, werr := c.writeEntry(final, version); werr != nil {
		// We still got a fresh result; just couldn't publish it.
		c.warn("failed to publish refreshed registry cache", werr)
	}
	return res, nil
}

func (c *Cached) warn(msg string, err error) {
	if c.logger == nil {
		return
	}
	c.logger.Warn(msg, slog.String("error", err.Error()))
}
