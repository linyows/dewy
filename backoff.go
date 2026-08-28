package dewy

import (
	"math/rand"
	"sync"
	"time"

	"github.com/linyows/dewy/internal/sysdeps"
)

// backoff stretches the effective polling interval after consecutive
// failures. The scheduler keeps firing at the configured interval; this only
// decides whether a tick does any work.
//
// The first failure never opens a window: one failed tick is more often a blip
// than an outage, and delaying recovery for it costs more than it saves.
type backoff struct {
	mu       sync.Mutex
	base     time.Duration
	maxDelay time.Duration
	clock    sysdeps.Clock
	// jitter spreads the delay above base; failure applies it. Tests replace
	// it to make windows exact.
	jitter   func(time.Duration) time.Duration
	failures int
	until    time.Time
}

// newBackoff grows delays from base, bounded by maxDelay. A maxDelay of
// zero -- the default, meaning no --max-backoff-interval -- is inert and never
// skips a tick; so is a zero base. maxDelay is raised to base so a delay can
// never come out shorter than one interval.
func newBackoff(base, maxDelay time.Duration) *backoff {
	if maxDelay > 0 && base > maxDelay {
		maxDelay = base
	}
	return &backoff{
		base:     base,
		maxDelay: maxDelay,
		clock:    sysdeps.RealClock(),
		jitter:   equalJitter,
	}
}

// equalJitter returns a duration in [d/2, d). Applied only above base, so the
// total stays over one interval while still spreading instances that failed
// together.
func equalJitter(d time.Duration) time.Duration {
	if d <= 0 {
		return 0
	}
	half := d / 2
	if half <= 0 {
		return d
	}
	return half + time.Duration(rand.Int63n(int64(half))) //nolint:gosec // G404: jitter needs no cryptographic randomness
}

// skip reports whether a backoff window is still open.
func (b *backoff) skip() bool {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.until.IsZero() {
		return false
	}
	return b.clock.Now().Before(b.until)
}

// success clears the failure streak and returns the length it cleared.
func (b *backoff) success() int {
	b.mu.Lock()
	defer b.mu.Unlock()

	cleared := b.failures
	b.failures = 0
	b.until = time.Time{}
	return cleared
}

// failure records a failed tick and returns the new streak along with the time
// polling resumes, or the zero time when no window opened.
func (b *backoff) failure() (failures int, until time.Time) {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.failures++
	d := b.delayForLocked(b.failures)
	if d <= b.base {
		b.until = time.Time{}
		return b.failures, time.Time{}
	}

	b.until = b.clock.Now().Add(b.base + b.jitter(d-b.base))
	return b.failures, b.until
}

// failureCount returns the consecutive-failure streak.
func (b *backoff) failureCount() int {
	b.mu.Lock()
	defer b.mu.Unlock()

	return b.failures
}

// delayFor returns the delay for n consecutive failures, before jitter.
func (b *backoff) delayFor(n int) time.Duration {
	b.mu.Lock()
	defer b.mu.Unlock()

	return b.delayForLocked(n)
}

// delayForLocked doubles in a loop rather than shifting by n, so a long streak
// cannot overflow the duration.
func (b *backoff) delayForLocked(n int) time.Duration {
	if b.base <= 0 || b.maxDelay <= 0 {
		return 0
	}
	if n <= 1 {
		return b.base
	}

	d := b.base
	for i := 1; i < n; i++ {
		d *= 2
		if d >= b.maxDelay {
			return b.maxDelay
		}
	}
	return d
}
