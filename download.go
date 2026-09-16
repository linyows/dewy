package dewy

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/linyows/dewy/artifact"
	"github.com/linyows/dewy/cache"
	"github.com/linyows/dewy/checksum"
	"github.com/linyows/dewy/registry"
)

// errDeferred means "another instance is already doing this; try again next
// tick". It is not a deployment failure: Run turns it into a silent no-op so
// that a fleet waiting on one downloader does not put every follower into
// error backoff and fire a notification per instance.
var errDeferred = errors.New("deferred to another instance")

const (
	// downloadLockTTL bounds how long one instance may hold the artifact lock.
	// It has to cover a slow download of a large artifact, because a lock that
	// expires mid-download lets a second instance start the same transfer.
	downloadLockTTL = 10 * time.Minute

	// blobIndexKeyPrefix namespaces the per-artifact index records that record
	// what the shared cache holds.
	blobIndexKeyPrefix = "blobs/"

	// artifactLockPrefix namespaces the per-artifact download locks.
	artifactLockPrefix = "artifact/"
)

// blobIndex records what an instance published to the shared cache under an
// artifact's cache key. Its job is to let a follower answer two questions
// without going back to the registry: has a peer already published these
// bytes, and are the bytes it reads back the ones that were verified.
type blobIndex struct {
	SHA256      string    `json:"sha256"`
	Size        int64     `json:"size"`
	PublishedAt time.Time `json:"published_at"`
	PublishedBy string    `json:"published_by,omitempty"`
}

func blobIndexKey(cacheKey string) string { return blobIndexKeyPrefix + cacheKey + ".json" }

func artifactLockName(cacheKey string) string { return artifactLockPrefix + cacheKey }

// downloadAndCache makes the artifact bytes available locally, from the
// shared cache when a peer has already fetched them and from upstream
// otherwise.
//
// Without coordination a fleet that ticks together all miss the cache at the
// same moment and all download the same artifact, which is up to
// MaxArtifactSize per instance against the registry. One instance takes the
// lock; the rest defer their tick and find the artifact cached on the next
// poll.
func (d *Dewy) downloadAndCache(ctx context.Context, res *registry.CurrentResponse, st cacheState) error {
	if st.foundInCache {
		return nil
	}

	if d.locker == nil {
		// A backend that cannot coordinate (the file cache) is single-instance
		// anyway, so there is no peer to collide with.
		return d.downloadAndPublish(ctx, res, st, nil)
	}

	lock, err := d.locker.Acquire(ctx, artifactLockName(st.key), cache.LockOptions{
		TTL:     downloadLockTTL,
		TryOnce: true,
		Value:   []byte(d.nodeID),
	})
	switch {
	case errors.Is(err, cache.ErrLockBusy):
		// A peer is downloading. Waiting here would block the scheduler for as
		// long as the transfer takes; deferring costs at most one polling
		// interval, and by then the artifact is in the shared cache and the
		// ordinary cached path picks it up.
		d.logger.Info("Deploy deferred: a peer is downloading this artifact",
			slog.String("cache_key", st.key))
		return errDeferred

	case err != nil:
		// Coordination is unavailable. Downloading uncoordinated is the same
		// behavior Dewy had before the lock existed, and it beats not
		// deploying at all.
		d.logger.Warn("Artifact lock unavailable; downloading without coordination",
			slog.String("cache_key", st.key), slog.String("error", err.Error()))
		return d.downloadAndPublish(ctx, res, st, nil)
	}
	defer func() {
		if err := lock.Unlock(); err != nil {
			d.logger.Warn("Failed to release artifact lock",
				slog.String("cache_key", st.key), slog.String("error", err.Error()))
		}
	}()

	// Re-check now that we hold the lock. resolveCacheState looked before we
	// started waiting, and a peer may have published in between. This is what
	// turns N downloads into one: every follower that queued behind the leader
	// finds the artifact here instead of fetching it again.
	if idx, err := d.readBlobIndex(st.key); err == nil {
		d.logger.Info("Artifact already published by a peer; loading from the shared cache",
			slog.String("cache_key", st.key), slog.String("published_by", idx.PublishedBy))
		return d.stageFromCache(st, idx)
	}

	return d.downloadAndPublish(ctx, res, st, lock.Lost())
}

// downloadAndPublish fetches the artifact from upstream, verifies it, and
// writes it to the cache along with the index that tells peers it is there.
//
// lost is the download lock's liveness channel, or nil when the download is
// uncoordinated.
func (d *Dewy) downloadAndPublish(ctx context.Context, res *registry.CurrentResponse, st cacheState, lost <-chan struct{}) error {
	buf := new(bytes.Buffer)
	if d.artifact == nil {
		a, err := artifact.New(ctx, res.ArtifactURL, d.logger.Slog())
		if err != nil {
			return fmt.Errorf("failed artifact.New: %w", err)
		}
		d.artifact = a
	}
	err := d.artifact.Download(ctx, &limitedWriter{W: buf, N: MaxArtifactSize})
	d.artifact = nil
	if err != nil {
		return fmt.Errorf("failed artifact.Download: %w", err)
	}

	// Verify before anything is written to the cache, so a corrupt or
	// tampered artifact is never staged for extraction and never shared with
	// the other instances pointed at the same cache backend.
	if err := d.verifyChecksum(ctx, res, buf.Bytes()); err != nil {
		return err
	}

	if lockExpired(lost) {
		// The bytes are verified and the cache key names this exact artifact,
		// so publishing anyway is safe — a peer that took the lock over writes
		// the same content. Worth saying out loud, though: it means
		// downloadLockTTL is too short for artifacts this size, and until it is
		// raised the fleet will keep paying for duplicate transfers.
		d.logger.Warn("Artifact download outlived its lock; consider a longer lock TTL",
			slog.String("cache_key", st.key), slog.Duration("lock_ttl", downloadLockTTL))
	}

	if err := d.cache.Write(st.key, buf.Bytes()); err != nil {
		return fmt.Errorf("failed cache.Write cachekeyName: %w", err)
	}
	d.publishBlobIndex(st.key, buf.Bytes())

	if err := d.cache.Write(currentkeyName, []byte(st.key)); err != nil {
		return fmt.Errorf("failed cache.Write currentkeyName: %w", err)
	}
	d.logger.Info("Cached artifact", slog.String("cache_key", st.key))
	return nil
}

// stageFromCache loads an artifact a peer already published into local
// staging, checking it against the index the peer wrote.
func (d *Dewy) stageFromCache(st cacheState, idx *blobIndex) error {
	data, err := d.cache.Read(st.key)
	if err != nil {
		return fmt.Errorf("failed to load cached artifact: %w", err)
	}
	if err := d.verifyAgainstIndex(st.key, data, idx); err != nil {
		return err
	}
	if err := d.cache.Write(currentkeyName, []byte(st.key)); err != nil {
		return fmt.Errorf("failed cache.Write currentkeyName: %w", err)
	}
	return nil
}

// verifyAgainstIndex checks bytes read back from the shared cache against the
// digest recorded when they were published.
//
// A fresh download is checked against the registry's own checksum file, but
// bytes that arrive through the cache never pass that check — they are
// verified once, by whichever instance downloaded them. The index closes that
// gap, so that an object corrupted or replaced in a shared bucket cannot be
// deployed across the fleet.
//
// A missing index is not an error: artifacts staged by a Dewy release older
// than the index simply have nothing to check against.
func (d *Dewy) verifyAgainstIndex(cacheKey string, data []byte, idx *blobIndex) error {
	if idx == nil || idx.SHA256 == "" {
		d.logger.Debug("Cached artifact has no published digest; skipping verification",
			slog.String("cache_key", cacheKey))
		return nil
	}
	if err := checksum.Verify(data, idx.SHA256); err != nil {
		// Drop the local copy: leaving it staged would make the next tick read
		// it straight back from disk and skip this check.
		if derr := d.cache.Delete(cacheKey); derr != nil {
			d.logger.Warn("Failed to drop the corrupt cached artifact",
				slog.String("cache_key", cacheKey), slog.String("error", derr.Error()))
		}
		return fmt.Errorf("cached artifact %s does not match the published digest: %w", cacheKey, err)
	}
	d.logger.Debug("Verified cached artifact against the published digest",
		slog.String("cache_key", cacheKey), slog.String("sha256", idx.SHA256))
	return nil
}

// publishBlobIndex records the digest of what we just cached. It is best
// effort: the artifact is already written, and a peer that finds no index
// falls back to downloading for itself.
func (d *Dewy) publishBlobIndex(cacheKey string, data []byte) {
	sum := sha256.Sum256(data)
	idx := &blobIndex{
		SHA256:      hex.EncodeToString(sum[:]),
		Size:        int64(len(data)),
		PublishedAt: time.Now(),
		PublishedBy: d.nodeID,
	}
	encoded, err := json.Marshal(idx)
	if err != nil {
		d.logger.Warn("Failed to encode the artifact index",
			slog.String("cache_key", cacheKey), slog.String("error", err.Error()))
		return
	}
	if err := d.cache.Write(blobIndexKey(cacheKey), encoded); err != nil {
		d.logger.Warn("Failed to publish the artifact index",
			slog.String("cache_key", cacheKey), slog.String("error", err.Error()))
	}
}

// readBlobIndex returns the index published for cacheKey, or an error when
// none exists.
func (d *Dewy) readBlobIndex(cacheKey string) (*blobIndex, error) {
	data, err := d.cache.Read(blobIndexKey(cacheKey))
	if err != nil {
		return nil, err
	}
	idx := &blobIndex{}
	if err := json.Unmarshal(data, idx); err != nil {
		return nil, fmt.Errorf("decode artifact index for %s: %w", cacheKey, err)
	}
	return idx, nil
}

// lockExpired reports whether a lock's liveness channel has closed. A nil
// channel — an uncoordinated download — never reports a loss.
func lockExpired(lost <-chan struct{}) bool {
	select {
	case <-lost:
		return true
	default:
		return false
	}
}
