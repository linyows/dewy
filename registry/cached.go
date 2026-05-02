package registry

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"runtime"
	"strconv"
	"time"

	"github.com/linyows/dewy/cache"
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
	// LockedAt records when a peer began refreshing. Zero means no peer
	// is currently refreshing.
	LockedAt time.Time `json:"locked_at,omitempty"`
	LockedBy string    `json:"locked_by,omitempty"`
}

// Cached wraps an upstream Registry with a shared, TTL-based result cache.
//
// Multiple Dewy instances sharing the same AtomicCache prefix coordinate so
// that only one of them calls the upstream registry per TTL window. Other
// instances read the cached response from the shared cache.
//
// The cache entry doubles as a refresh lock: the leader CAS-updates LockedAt
// before calling upstream, and clears it (along with the new response) on
// success. Followers that observe a recent LockedAt back off briefly and
// re-read; if the entry is still stale on retry they fall back to the last
// known response (stale-but-usable).
type Cached struct {
	inner    Registry
	cache    cache.AtomicCache
	ttl      time.Duration
	lockTTL  time.Duration
	wait     time.Duration
	logger   *logging.Logger
	nodeID   string
	cacheKey string
}

// NewCached wraps inner with a shared registry-result cache backed by
// atomicCache. ttl controls how long a cached response is considered fresh.
//
// scope is an opaque identifier used to derive the cache key — typically the
// upstream registry URL. Two Cached instances that share an AtomicCache
// prefix coordinate single-flight refresh only when they pass the same scope
// (and run on the same platform), preventing instances with different
// registry URLs or differing OS/arch from overwriting each other's entries.
func NewCached(inner Registry, scope string, atomicCache cache.AtomicCache, ttl time.Duration, log *logging.Logger) *Cached {
	hostname, _ := os.Hostname()
	return &Cached{
		inner:    inner,
		cache:    atomicCache,
		ttl:      ttl,
		lockTTL:  maxLockTTL(ttl),
		wait:     defaultRefreshWait,
		logger:   log,
		nodeID:   hostname + ":" + strconv.Itoa(os.Getpid()),
		cacheKey: cacheKeyForScope(scope),
	}
}

// cacheKeyForScope returns the cache key used by a Cached instance with the
// given scope. The local OS/arch are folded in because Current() responses
// vary by platform (artifact name selection). The scope is canonicalized so
// that registry URLs that differ only in query-parameter ordering hash to
// the same key.
func cacheKeyForScope(scope string) string {
	h := sha256.Sum256([]byte(canonicalizeScope(scope) + "|" + runtime.GOOS + "|" + runtime.GOARCH))
	return registryCacheKeyPrefix + hex.EncodeToString(h[:8]) + ".json"
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
// peer is actively refreshing (LockedAt within lockTTL) we keep waiting so
// that we do not stampede upstream just because the peer's refresh takes
// longer than a few hundred milliseconds. If lockTTL elapses without the
// peer publishing, we treat the lock as abandoned and claim it ourselves.
func (c *Cached) Current(ctx context.Context) (*CurrentResponse, error) {
	deadline := time.Now().Add(c.lockTTL + c.wait)

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

		// A peer is refreshing — wait briefly and try again, up to lockTTL.
		if entry != nil && c.isLocked(entry) && time.Now().Before(deadline) {
			if err := sleepCtx(ctx, c.wait); err != nil {
				if entry.Response != nil {
					return entry.Response, nil
				}
				return nil, err
			}
			continue
		}

		// Either stale, absent, or the peer's lock has expired. Try to claim.
		claim := buildClaim(entry, c.nodeID)
		newVersion, err := c.writeEntry(claim, version)
		if err != nil {
			if cache.IsConflict(err) {
				// Another node beat us to the claim. Re-read and continue.
				if err := sleepCtx(ctx, c.wait); err != nil {
					if entry != nil && entry.Response != nil {
						return entry.Response, nil
					}
					return nil, err
				}
				continue
			}
			c.warn("failed to claim registry cache lock", err)
			if entry != nil && entry.Response != nil {
				return entry.Response, nil
			}
			return c.inner.Current(ctx)
		}

		// We hold the lock — perform the upstream call.
		return c.refreshAndPublish(ctx, entry, newVersion)
	}
}

// sleepCtx is time.Sleep that respects ctx cancellation.
func sleepCtx(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
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
	return entry.Response != nil && time.Since(entry.FetchedAt) < c.ttl
}

func (c *Cached) isLocked(entry *cachedEntry) bool {
	return !entry.LockedAt.IsZero() && time.Since(entry.LockedAt) < c.lockTTL
}

// buildClaim returns the entry that marks "we are refreshing". The previous
// Response is preserved so concurrent readers can still serve stale-but-usable.
func buildClaim(prev *cachedEntry, nodeID string) *cachedEntry {
	now := time.Now()
	c := &cachedEntry{LockedAt: now, LockedBy: nodeID}
	if prev != nil {
		c.Response = prev.Response
		c.FetchedAt = prev.FetchedAt
	}
	return c
}

// refreshAndPublish calls the upstream registry, then writes the new entry
// (releasing the lock). On upstream failure it releases the lock with the
// previous Response so the cache continues to serve stale-but-usable.
func (c *Cached) refreshAndPublish(ctx context.Context, prev *cachedEntry, version string) (*CurrentResponse, error) {
	res, err := c.inner.Current(ctx)
	if err != nil {
		c.releaseLock(prev, version)
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

	final := &cachedEntry{
		Response:  res,
		FetchedAt: time.Now(),
		// LockedAt zero — released.
	}
	if _, werr := c.writeEntry(final, version); werr != nil {
		// We still got a fresh result; just couldn't publish it.
		c.warn("failed to publish refreshed registry cache", werr)
	}
	return res, nil
}

// releaseLock writes back the previous entry without LockedAt. Best effort —
// any failure is logged and ignored. The lockTTL bound ensures a stuck lock
// eventually becomes claimable by another node anyway.
func (c *Cached) releaseLock(prev *cachedEntry, version string) {
	released := &cachedEntry{}
	if prev != nil {
		released.Response = prev.Response
		released.FetchedAt = prev.FetchedAt
	}
	if _, err := c.writeEntry(released, version); err != nil {
		c.warn("failed to release registry cache lock", err)
	}
}

func (c *Cached) warn(msg string, err error) {
	if c.logger == nil {
		return
	}
	c.logger.Warn(msg, slog.String("error", err.Error()))
}
