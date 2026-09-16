# Consul KV Cache Backend and Distributed Single-Flight Design Document

This document describes the design for adding a HashiCorp Consul KV cache backend to dewy, and for using Consul sessions as a distributed lock so that a fleet of dewy instances issues one upstream registry request instead of one per instance.

**Status**: Phases 1-2 implemented, phases 3-4 proposed
**Author**: @linyows
**Last Updated**: 2026-09

## Goals

1. **Consul KV as a cache backend**: `--cache 'consul://<addr>/<prefix>'` works alongside the existing `file`, `s3` and `gs` backends.
2. **One registry metadata request per TTL window**: across the whole fleet, not per instance.
3. **One artifact download per release**: when a shared blob store is configured; strictly serialized downloads otherwise.
4. **Correct under failure**: a crashed leader must not block the fleet, and an unreachable Consul must not stop deployments.
5. **No behavior change for existing users**: the file/S3/GCS paths keep working exactly as they do today.

## Non-Goals

1. **Storing artifacts in Consul KV**: Consul's `kv_max_value_size` is 512KB by default while `MaxArtifactSize` is 512MB. Consul is a coordination store, not a blob store.
2. **Service discovery / health checks via Consul**: registering dewy-managed services in the Consul catalog is out of scope.
3. **Consul as the only coordination backend**: etcd, ZooKeeper and Redis are out of scope; the interface introduced here leaves room for them.
4. **Push-based deployment**: dewy stays pull-only. Consul watches/blocking queries are not used to trigger deploys (see Open Questions).
5. **Replacing the existing CAS-based coordination**: the S3/GCS conditional-write path stays as the default for users who do not run Consul.

# Background

## What already exists

Dewy already has a partial answer to the thundering-herd problem:

- `cache.AtomicCache` (`cache/cache.go`) exposes `ReadWithVersion` / `WriteIfMatch`, implemented by the S3 backend (ETag + `If-Match`/`If-None-Match`) and the GCS backend (generation + preconditions).
- `registry.Cached` (`registry/cached.go`) wraps a `Registry` with a shared, TTL-based result cache. The cache entry doubles as a refresh lock: `LockedAt`/`LockedBy` are CAS-written before the upstream call and cleared afterwards. Followers back off and re-read, falling back to the stale-but-usable response.
- It is opted into with `?registry-ttl=30s` on the cache URL, and ignored on the file backend.

So `Current()` — the "is there a new version?" request — is already single-flighted **when the fleet shares an S3 or GCS bucket**.

## What is missing

1. **No coordination without a shared object store.** Fleets that do not use S3/GCS (on-premises, Sakura Cloud, bare-metal VMs) have no way to share the registry result at all. Consul is frequently already deployed in exactly those environments.
2. **The artifact download is not single-flighted.** `downloadAndCache` (`lifecycle.go`) checks `st.foundInCache` and otherwise downloads. When N instances tick at the same moment they all miss, and all N download the artifact — potentially N × 512MB against GitHub Releases or an OCI registry. The S3 backend accidentally mitigates this only for instances that tick *after* a peer has finished uploading; simultaneous ticks all download.
3. **The checksum fetch multiplies the same way.** `verifyChecksum` fetches `ChecksumURL` per instance per download.
4. **The CAS lock has no liveness signal.** `registry.Cached` infers a dead leader from a wall-clock timestamp (`maxLockTTL`, 30s–5m). A leader that is merely slow is indistinguishable from one that crashed, and clock skew between instances directly shifts the decision. A Consul session is a real liveness primitive: it is renewed by the holder and invalidated by the server, so the lock releases automatically when the holder dies.
5. **`current` is shared in the S3 backend.** Because the S3 backend stores every key, including the instance-local `current` and `blocked` pointers, under the same prefix, instances sharing a prefix overwrite each other's deployment state.

   Fixed in phase 2, and it had to be: making downloads single-flight turns this from a race into the normal path. One instance publishes the artifact and moves `current`; every other instance then reads a pointer saying the deploy already happened on its own node and skips it, permanently. `current` and `blocked` now live in a local state store — `Dewy.state()`, a file cache rooted in the directory the artifact cache stages into — for every backend.

## Request-count math

Fleet of N instances, polling interval `i`, registry TTL `t`, one new release published.

| Path | Today (file) | Today (s3 + registry-ttl) | With this design |
| --- | --- | --- | --- |
| `Current()` per TTL window | N | 1 | 1 |
| Artifact download per release | N | 1..N (race-dependent) | 1 (shared blob store) / N serialized (local blobs) |
| Checksum fetch per release | N | 1..N | 1 / N serialized |

For a 300-node fleet polling every 10s against GitHub Releases, the first row alone is the difference between 1,800 req/min and 2 req/min.

# Design

Three layers, each usable on its own:

1. **`cache.Consul`** — a `Cache` + `AtomicCache` backend that keeps coordination state in Consul KV and delegates blobs to a local or object-store backend.
2. **`cache.Locker`** — a new optional capability interface for a real distributed lock, implemented natively by Consul and emulated on top of CAS for S3/GCS.
3. **Download single-flight** — `downloadAndCache` acquires a per-artifact lock, re-checks the shared store after acquiring, and only then downloads.

```
                     ┌──────────────────────────────────────────┐
   dewy instance ──► │ registry.Cached      (Current single-flight)
                     │   ├─ cache.AtomicCache  → shared entry (CAS)
                     │   └─ cache.Locker       → refresh lock
                     ├──────────────────────────────────────────┤
                     │ Dewy.downloadAndCache (download single-flight)
                     │   ├─ cache.Locker       → per-artifact lock
                     │   └─ blob delegate      → artifact bytes
                     └──────────────────────────────────────────┘
                                     │
                     ┌───────────────┴───────────────┐
                     ▼                               ▼
              Consul KV / sessions            local dir | S3 | GCS
              (small, coordination)           (large, blobs)
```

## 1. Consul cache backend

### URL grammar

```
consul://[<host>[:<port>]]/<prefix>[?<params>]
```

`<host>` is optional: when omitted, `api.DefaultConfig()` resolves the agent from `CONSUL_HTTP_ADDR` (default `127.0.0.1:8500`), which is the normal deployment shape — every node runs a local Consul agent. `<prefix>` is the KV prefix, e.g. `dewy/myapp`.

| Parameter | Default | Meaning |
| --- | --- | --- |
| `registry-ttl` | `0` | Same meaning as S3/GCS: freshness window of the shared registry response. `0` disables registry-result caching. |
| `blob` | `file` | Where artifact bytes live: `file`, `file:///path`, `s3://...`, `gs://...`. Determines whether downloads collapse to one. |
| `dc` | agent's DC | Consul datacenter for reads and writes. |
| `namespace`, `partition` | empty | Consul Enterprise namespace / admin partition. |
| `scheme` | from env | `http` or `https`. |
| `lock-ttl` | `30s` | Consul session TTL. Minimum 10s (Consul limit); the holder renews at TTL/2. |
| `lock-wait` | `2m` | How long a follower waits for a lock before giving up the tick. |
| `lock-delay` | `0s` | Consul lock-delay after session invalidation. `0` because every critical section here is idempotent and guarded by a CAS on the published entry. |
| `download-concurrency` | `1` | Lock is a semaphore of this size. `>1` trades registry load for fleet-wide rollout speed. |

The ACL token is never taken from the URL (it would end up in logs and in `ps`). It comes from `CONSUL_HTTP_TOKEN`, or from a file via `CONSUL_HTTP_TOKEN_FILE`, both handled by `api.DefaultConfig()`. TLS likewise comes from `CONSUL_CACERT` / `CONSUL_CLIENT_CERT` / `CONSUL_CLIENT_KEY`.

Example:

```
dewy server \
  --registry 'ghr://linyows/myapp' \
  --cache 'consul://127.0.0.1:8500/dewy/myapp?registry-ttl=30s&blob=s3://ap-northeast-1/mybucket/myapp' \
  -- /opt/myapp/current/myapp
```

### Key layout

```
<prefix>/registry-cache/<scope-hash>.json   shared registry response (CAS)
<prefix>/blobs/<cache-key>.json             blob index: sha256, size, location
<prefix>/locks/registry/<scope-hash>        session lock
<prefix>/locks/artifact/<cache-key>         session lock (or semaphore prefix)
<prefix>/nodes/<node-id>/current            observability mirror of node-local state
```

`current` and `blocked` do not appear here because they are no longer in the cache backend at all; see the routing table below.

`<scope-hash>` reuses `registry.cacheKeyForScope`, which already folds in the registry URL and `GOOS`/`GOARCH`.

### Key routing

The Consul backend is a router, not a store. `Read`/`Write`/`Delete`/`List` classify the key:

| Key class | Destination | Rationale |
| --- | --- | --- |
| `registry-cache/*` | Consul KV | Small, must be shared, needs CAS. |
| `blobs/*` | Consul KV | Small index of what the blob delegate holds. |
| artifact keys (`<tag>--<file>`) | blob delegate | Up to 512MB. Never fits in Consul KV. |
| `current`, `blocked` | local state store, always | Per-instance deployment state, moved out of the cache backend in phase 2. Sharing it makes one instance's progress look like every instance's. Optionally mirrored write-only to `nodes/<node-id>/` for `consul kv get`-based debugging. |

`List()` returns the blob delegate's listing, so `resolveCacheState` keeps its current semantics unchanged.

Any value routed to Consul KV that exceeds `kv-max-value-size` (512KB, configurable to match a tuned agent) is rejected at `Write` with an explicit error rather than being truncated by the agent.

### AtomicCache mapping

Consul's CAS maps onto `AtomicCache` exactly, with no impedance mismatch:

| `AtomicCache` | Consul |
| --- | --- |
| `version` | `KVPair.ModifyIndex` rendered as a decimal string |
| `ReadWithVersion` | `KV().Get()`; nil pair → `ErrNotFound` |
| `WriteIfMatch(key, "", data)` | `KV().CAS()` with `ModifyIndex: 0` (create-only) |
| `WriteIfMatch(key, v, data)` | `KV().CAS()` with `ModifyIndex: v` |
| conflict | `CAS` returns `ok == false` → `ErrConflict` |

The new `ModifyIndex` is not returned by `CAS`, so the backend re-reads the key after a successful CAS to obtain it. That is one extra read per write on the coordination path only; it is not on the artifact path.

## 2. `cache.Locker`

A new optional capability, alongside `AtomicCache`:

```go
// cache/lock.go

// Locker is an optional capability for cache backends that can coordinate
// mutual exclusion across instances. Backends that expose a real session or
// lease primitive implement it directly; CAS-capable backends get an
// emulation via NewCASLocker.
type Locker interface {
	// Acquire blocks until the named lock is held, opts.Wait elapses, or ctx
	// is done. It returns ErrLockBusy when the lock could not be taken in time.
	// ctx is observed between attempts; see the note on cancellation below.
	Acquire(ctx context.Context, name string, opts LockOptions) (Lock, error)
}

type LockOptions struct {
	TTL     time.Duration // holder liveness; the lock releases if the holder dies
	Wait    time.Duration // 0 means "wait until ctx is done"
	TryOnce bool          // return ErrLockBusy instead of waiting at all
	Limit   int           // >1 makes this a semaphore with Limit holders
	Value   []byte        // informational holder metadata (node id, tag)
}

type Lock interface {
	// Lost is closed when the lock is no longer held: the session expired or
	// the backend became unreachable. Long critical sections must abort on it.
	Lost() <-chan struct{}
	Unlock() error
}

var ErrLockBusy = errors.New("lock held by another instance")

// LockerFor returns a Locker for c: the backend's native implementation when
// it has one, a CAS-based emulation when the backend is an AtomicCache, and
// (nil, false) otherwise.
func LockerFor(c Cache) (Locker, bool)
```

### Consul implementation

`client.LockOpts(&api.LockOptions{Key, Value, SessionTTL, LockWaitTime, LockTryOnce, MonitorRetries, MonitorRetryTime})` for `Limit == 1`, `client.SemaphoreOpts` for `Limit > 1`. The consul api client renews the session automatically and closes the returned `lockLost` channel when the session is invalidated — that channel becomes `Lock.Lost()`. `Acquire` bridges `ctx` to the `stopCh` argument of `lock.Lock(stopCh)`, and maps a `nil` lock (wait timeout) to `ErrLockBusy`.

`api.ErrLockHeld`, `api.ErrLockNotHeld` and `api.ErrLockConflict` are translated to package-level errors so callers never import `consul/api`.

### CAS emulation

`NewCASLocker(ac AtomicCache)` reproduces today's `registry.Cached` algorithm — a lock record at `locks/<name>` holding `{holder, acquired_at, expires_at}`, claimed with `WriteIfMatch` and considered abandoned once `expires_at` passes — behind the same interface. This keeps the S3/GCS behavior equivalent to today while letting the call sites be written against one interface.

There is deliberately no background renewal. Liveness is the expiry the holder wrote, not a lease it keeps proving, so `Lost()` closes when that expiry is reached (or on `Unlock`) and a holder cannot extend its claim by staying alive. Two consequences follow, and callers need both:

- A holder that is merely slow loses its lock exactly like one that crashed. Work that can outrun the TTL must select on `Lost()` and decline to publish, which is what `Cached.refreshAndPublish` does.
- Instances whose clocks disagree disagree about when a lock expired. This is the same exposure the pre-Locker timestamp scheme had; only a backend with a server-side session removes it.

Adding renewal would mean a periodic conditional write per held lock against S3 or GCS. That is a real cost on the object-store path for a benefit only slow-critical-section callers see, so it is left to backends that get sessions for free.

**Cancellation.** `AtomicCache.ReadWithVersion` and `WriteIfMatch` take no context — the S3 and GCS backends use the context their constructor was given — so `Acquire` observes `ctx` between attempts but cannot interrupt a backend call already in flight. A hung object-store request therefore outlives a canceled `Acquire`. This is a property of the `AtomicCache` interface rather than of locking, and fixing it means threading a context through those methods and their call sites; it is tracked as a follow-up rather than done here.

## 3. Registry single-flight (refactor of `registry.Cached`)

`Cached` keeps its entry format and its stale-while-revalidate behavior. The only change is that the "claim the refresh" step becomes `Locker.Acquire(ctx, "registry/"+scope, LockOptions{TTL: lockTTL, Wait: 0, TryOnce})` instead of a CAS write of `LockedAt`/`LockedBy`:

```
read entry
  fresh?                  → return entry.Response            (0 upstream requests)
  stale or absent:
    Acquire(TryOnce)
      got it   → upstream Current() → CAS-publish → Unlock   (1 upstream request)
      ErrLockBusy → wait `wait`, re-read
                    fresh now? → return it
                    still stale after lock-wait?
                      entry.Response != nil → return stale
                      else                  → direct upstream (cold-start only)
```

The "still stale after lock-wait" arm is reached only under a locker that renews its lease while the holder is alive, which is the Consul case: a slow-but-healthy leader keeps the lock for the follower's entire wait. Under `CASLocker` the lock expires first — the wait is `lockTTL + wait` against a lock of `lockTTL` — so the follower takes the lock over and refreshes, which is the pre-Locker behavior.

`LockedAt`/`LockedBy` stay in the JSON, but the direction of compatibility is the reverse of what it first appears. Dewy no longer *writes* them: a claim that nothing clears would make every pre-Locker peer wait out its whole lock TTL on each poll. Dewy does still *read* them, on every pass and regardless of which locker is in use, because during a rolling upgrade a peer running the old code claims the entry that way and is entitled to be waited for.

The CAS on publish remains, so a leader whose lease expired mid-refresh cannot clobber a newer entry written by its successor. It declines to publish at all in that case; see the CAS emulation notes above.

Notably, the cold-start fallback ("nothing cached, lock busy, upstream directly") is the one case where more than one instance can hit the registry. It is bounded to the first TTL window after a fleet-wide cold start, and it exists so that a Consul outage cannot deadlock a fresh deploy.

## 4. Download single-flight

`Dewy.downloadAndCache` acquires a per-artifact lock before fetching:

```go
func (d *Dewy) downloadAndCache(ctx context.Context, res *registry.CurrentResponse, st cacheState) error {
	if st.foundInCache {
		return nil
	}
	if d.locker == nil {
		return d.downloadAndPublish(ctx, res, st, nil) // file backend: single-instance anyway
	}

	lock, err := d.locker.Acquire(ctx, artifactLockName(st.key), cache.LockOptions{
		TTL: downloadLockTTL, TryOnce: true, Value: []byte(d.nodeID),
	})
	switch {
	case errors.Is(err, cache.ErrLockBusy):
		return errDeferred // a peer is downloading; retry next tick
	case err != nil:
		return d.downloadAndPublish(ctx, res, st, nil) // coordination down; degrade
	}
	defer lock.Unlock()

	// Re-check now that we hold the lock: a peer may have published in between.
	if idx, err := d.readBlobIndex(st.key); err == nil {
		return d.stageFromCache(st, idx) // verifies sha256 against the index
	}
	return d.downloadAndPublish(ctx, res, st, lock.Lost())
}
```

Three points deserve emphasis.

**The re-check after acquiring is what collapses N downloads into 1.** Without it the lock only serializes; with it, every follower that comes back after deferring finds the artifact already in the shared store and never contacts the registry.

This only works when the blob store is one the fleet shares (S3/GCS). Under the phase-3 Consul backend with `blob=file` the lock is Consul's but the blobs are local, so no follower ever finds a peer's copy and they each download in turn: the registry sees a concurrency of 1 instead of N, which is the meaningful protection, but the request count is still N.

That is not the same as the plain `file` cache backend, which has no locker at all. There, `downloadAndCache` skips coordination entirely and downloads run exactly as they did before this work — correctly, because that backend is single-instance.

**Followers defer the tick rather than waiting on the lock.** Blocking until the leader finishes would hold the scheduler for the length of a transfer that can run to `MaxArtifactSize`. Deferring costs at most one polling interval, and by the next tick `resolveCacheState` finds the artifact in the shared cache and takes the ordinary cached path without touching the lock at all. So `Acquire` is `TryOnce` and there is no `Wait` to configure.

**`errDeferred` is not a deployment failure.** `Run` turns it into a silent no-op tick, like the slot-mismatch and grace-period skips, so it neither feeds `d.backoff` nor fires the error notifier and is not counted as a deployment. A fleet rolling out a large artifact would otherwise put every follower into exponential backoff and send one alert per instance.

### What `Lost()` is and is not used for here

The registry refresh declines to publish when its lock expired, because the thing it publishes is a claim about freshness. The download is not like that: the cache key names one tag and one file name, and the bytes are checksum-verified before they are written, so a second instance that takes the lock over writes exactly the same content. Publishing after losing the lock is therefore harmless, and aborting a nearly-complete transfer would throw away the bytes we came for.

So `downloadAndPublish` takes `Lost()` but uses it only to log: a download that outlived its lock means `downloadLockTTL` is too short for artifacts this size, and until it is raised the fleet keeps paying for duplicate transfers. That is a tuning signal for the operator, not a reason to discard work.

### Integrity across the shared store

Followers that read the artifact from the shared blob store would skip `verifyChecksum` entirely, because verification happens only on the download path. The blob index closes that gap: the downloader records `sha256` and `size` in `blobs/<cache-key>.json`, and every read that comes from the cache rather than from a fresh download is checked against it. There are two such reads — the re-check inside `downloadAndCache`, and the staging that `resolveCacheState` does when it finds the key already present — and both go through one helper so neither can drift.

A mismatch deletes the local staging copy and fails the tick. Dropping the copy matters: leaving it would let the next tick read it straight back from disk and skip the check.

A missing index is not a mismatch. Artifacts staged by a Dewy release older than the index have nothing to check against, and failing them would break the upgrade for every instance whose cache was already warm. Where there is a choice — the re-check path, where we hold the lock — unverifiable bytes are not trusted and the artifact is downloaded instead. That covers an index that is present but carries no digest, an index whose bytes are gone, and an index that cannot be read.

An index that cannot be read is never treated as one that was never written. A malformed record or a backend hiccup would otherwise be a way around the check. Only `cache.ErrNotFound` takes the compatibility path, which is why the file backend now reports that sentinel instead of a bare "File not found" error.

`blobIndex.Size` is recorded for operators reading the record and is not part of verification; a matching SHA-256 already implies a matching length.

A digest failure removes the instance's own staged copy and nothing else. `Cache.Delete` is not local-only on the cloud backends — it removes the object from the bucket — so one instance acting on its own verdict would cost every peer a re-download, and a genuinely tampered object would be quietly replaced rather than investigated. `cache.RemoveLocal` drops the local copy so the next tick re-reads from the shared store, and the error is surfaced.

### Not covered

The container command pulls images through the runtime rather than through the cache, so `RunContainer` is untouched. An OCI layer cache is per-node and the pull is what populates it, which makes the shared-store re-check inapplicable; serializing pulls across a fleet would need its own design.

## Failure modes

| Failure | Behavior |
| --- | --- |
| Consul unreachable at startup | `cache.New` fails; dewy exits with a clear configuration error. A cache backend that cannot be reached is a misconfiguration, not a transient. |
| Consul unreachable at runtime, read path | `registry.Cached` already falls back to a direct upstream call on read error. Logged at warn. |
| Consul unreachable at runtime, lock path | `Acquire` error → warn + proceed uncoordinated. The fleet degrades to today's behavior (N requests) rather than stopping deploys. `on-lock-error=skip` is available for operators who prefer the opposite trade. |
| Leader crashes mid-refresh | Session TTL expires, Consul invalidates the session, the lock is released automatically. No wall-clock heuristic involved. |
| Leader is slow, not dead | Session keeps being renewed, the lock is held, followers wait up to `lock-wait` then defer the tick. |
| Leader's session expires mid-download | `Lost()` fires, the download aborts before publishing. Another instance takes the lock and retries. |
| Clock skew between instances | Irrelevant on the Consul path: liveness is server-side. Still relevant for `registry-ttl` freshness, which compares `FetchedAt` against the local clock — unchanged from today. |
| Consul KV value too large | Rejected at `Write` with the key and size in the message. Only reachable if a registry response were to grow past 512KB. |
| ACL token lacks `key:write` on the prefix | Startup probe (a `KV().Get` on `<prefix>/`) surfaces the permission error at boot rather than on the first deploy. |

## Configuration and documentation surface

- `cli.go` / `config.go`: no new flags. Everything rides on the existing `--cache` URL, matching how S3/GCS options are configured today.
- `docs/pages/cache.md` and `docs/pages/ja/cache.md`: new "Consul KV backend" section.
- `docs/pages/reference.md` and its `ja` mirror: the `consul://` grammar and the parameter table.
- `docs/pages/architecture.md`: the coordination layer in the component diagram.

## Telemetry

Following the existing dot-separated OTel naming in `telemetry/telemetry.go`:

| Metric | Type | Purpose |
| --- | --- | --- |
| `dewy.registry.requests.total` | counter, attr `source=upstream\|cache` | Proves the fan-in actually happened. |
| `dewy.coordination.lock.acquisitions.total` | counter, attr `result=leader\|busy\|error` | Leader/follower ratio; should be 1:N-1. |
| `dewy.coordination.lock.wait.duration` | histogram | Sizing `lock-wait` against real artifact sizes. |
| `dewy.artifact.downloads.total` | counter, attr `source=upstream\|shared` | The number this whole design exists to reduce. |

## Testing

**Unit.** `cache.Locker` and `cache.Consul` are tested against an interface-level fake of the Consul KV/session API, the same way `S3Client` is faked in `cache/s3_test.go`. Compile-time assertions (`var _ AtomicCache = (*Consul)(nil)`, `var _ Locker = (*Consul)(nil)`) mirror the existing ones.

**Concurrency.** A table test drives M goroutines through `registry.Cached.Current` and `downloadAndCache` against the fake, asserting the upstream call count is exactly 1 and that every goroutine receives the same response. `sysdeps.Clock` is already injectable via `WithClock`, so TTL expiry is tested deterministically.

**Integration.** `consul agent -dev` in a `TestMain` guard (skipped when the binary is absent) exercises real sessions, TTL expiry and lock-delay — the parts a fake cannot honestly reproduce.

**E2E.** A `ghr-consulcache-a` / `-b` variant pair, modeled on the existing `ghr-s3cache-a`/`-b` variants in `e2e/test.yml`, with a Consul container in the job. The assertion is on the leader/follower log lines and on both instances converging to the same tag. Note that e2e runs in a single serial concurrency group, so this adds wall-clock time to every e2e run; the pair should be kept to one release cycle.

## Dependency cost

`github.com/hashicorp/consul/api` is a standalone module — it does not pull in the Consul server, Raft or Serf. Its transitive set is small (`hashicorp/go-cleanhttp`, `go-rootcerts`, `go-hclog`, `serf/coordinate`, `golang-lru`) and partially already present in the module graph. It is a direct dependency regardless of whether the operator uses Consul, since Go has no build-tag-free way to make a backend optional without an interface registry. If the added binary size proves material, a `consul` build tag that registers the backend is the fallback, at the cost of a non-obvious "unsupported cache scheme" error for users of the default build.

## Implementation phases

1. **`cache.Locker` + `NewCASLocker` + refactor `registry.Cached` onto it.** Implemented. No new dependency; the S3/GCS path keeps its current semantics and gains test coverage through the new interface. Two things changed beyond the mechanical move:
   - The leader re-reads the entry after acquiring the lock. Previously the claim was itself a conditional write on the entry, so a peer that published in the meantime caused the claim to conflict. With the lock held separately that conflict no longer happens, and without the re-read the leader would repeat a request the peer had already made.
   - `LockedAt`/`LockedBy` are no longer written, but are still honored on read, so a fleet running mixed versions during a rolling upgrade does not double up on upstream.
2. **Download single-flight** (`errDeferred`, blob index, `stageFromCache`, checksum verification on the shared path, and moving `current`/`blocked` to a local state store). Implemented. Immediately valuable to existing S3/GCS users — it closes the N-simultaneous-downloads hole. Two decisions differ from the original sketch: followers defer their tick instead of waiting on the lock, and a download that outlives its lock still publishes. Both are explained in the section above.
3. **`cache.Consul` backend** (KV routing, `AtomicCache`, native `Locker`, blob delegation) plus docs.
4. **Telemetry and e2e.**

Phases 1 and 2 are the agreed first delivery and carry no new dependency. Phase 3 introduces `consul/api` and follows once the coordination semantics are settled.

Note what phase 2 already delivers without Consul: an S3 or GCS fleet gets both single-flight refresh and single-flight download today. Consul's contribution in phase 3 is to extend that to fleets with no shared object store, and to replace the wall-clock expiry with a server-side session.

## Decisions

**Coordination and blob storage are composed inside `--cache`, not split across flags.** `consul://<addr>/<prefix>?blob=s3://...` carries both. The alternative — keeping `--cache s3://...` and adding `--lock consul://...` — separates responsibilities more cleanly, but it moves coordination configuration away from the component that performs the coordination, and it introduces a second URL grammar for what is one decision. The cost accepted here is that a single `--cache` value can reference two credential sets and two failure domains; the startup probe (below) validates both before the first tick, and errors name which of the two failed.

**Phases 1 and 2 ship first, without a Consul dependency.** Extracting `cache.Locker` and closing the simultaneous-download hole are valuable on their own to existing S3/GCS users, and they let the coordination semantics be reviewed and tested before the Consul backend is introduced on top.

## Upgrade note

Moving `current` out of the shared cache means an S3 or GCS instance starts with no local record of what it has deployed. On the first tick after the upgrade it treats the running version as undeployed and deploys it once more, which for `dewy server` is one restart per instance. Later ticks are unaffected.

Seeding the local record from the shared one was considered and rejected. If the fleet upgrades while a rollout is in flight, the seeded value can name a version the instance has not actually deployed, and it would skip that version exactly once — reintroducing the bug being fixed, in the case where it is hardest to notice. One extra deploy is the cheaper mistake.

The stale `current` object left in the bucket is never read again and can be removed at leisure.

## Open Questions

1. **Should `blob=` default to `file` or be required?** Defaulting to `file` makes `consul://host/prefix` work out of the box but delivers serialized-not-single downloads, which may surprise operators who read only the headline. Requiring `blob=` is louder but more honest.
2. **Blocking queries instead of polling `lock-wait`?** Consul's blocking-query support would let followers be woken the moment the leader publishes, rather than polling. It is a strict improvement in latency and Consul load, but it introduces a long-lived connection per instance to the local agent, which deserves its own measurement before adoption.
3. **Should `nodes/<node-id>/current` be authoritative rather than a mirror?** Making it authoritative would give `dewy` fleet-wide visibility of which node runs which version — useful, but it turns a debugging aid into a correctness dependency on Consul.
