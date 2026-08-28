package dewy

import (
	"testing"
	"time"

	"github.com/linyows/dewy/internal/sysdeps/fake"
)

// identityJitter makes delays exact so that window boundaries can be asserted.
func identityJitter(d time.Duration) time.Duration { return d }

func testClock() *fake.Clock {
	return fake.NewClock(time.Date(2026, 8, 25, 0, 0, 0, 0, time.UTC))
}

// testBackoffMax is the bound an operator would pass via --max-backoff-interval.
const testBackoffMax = 5 * time.Minute

func newTestBackoff(base time.Duration, clk *fake.Clock) *backoff {
	b := newBackoff(base, testBackoffMax)
	b.clock = clk
	b.jitter = identityJitter
	return b
}

func TestBackoffRunsBeforeAnyFailure(t *testing.T) {
	b := newTestBackoff(10*time.Second, testClock())

	if b.skip() {
		t.Error("skip() on a fresh backoff = true, want false: the first tick must always run")
	}
}

func TestBackoffFirstFailureDoesNotStretch(t *testing.T) {
	b := newTestBackoff(10*time.Second, testClock())

	if failures, until := b.failure(); !until.IsZero() {
		t.Errorf("failure() #1 (streak %d) opened a window until %v, want none", failures, until)
	}
	if b.skip() {
		t.Error("skip() after one failure = true, want false: a single blip must not delay recovery")
	}
}

func TestBackoffSecondFailureOpensWindow(t *testing.T) {
	clk := testClock()
	b := newTestBackoff(10*time.Second, clk)

	b.failure()
	b.failure()

	if !b.skip() {
		t.Fatal("skip() after two failures = false, want true")
	}

	// One second short of the window: still skipping.
	clk.Advance(19 * time.Second)
	if !b.skip() {
		t.Error("skip() at +19s = false, want true: the 20s window is not over yet")
	}

	clk.Advance(1 * time.Second)
	if b.skip() {
		t.Error("skip() at +20s = true, want false: the window has elapsed")
	}
}

func TestBackoffThirdFailureDoubles(t *testing.T) {
	clk := testClock()
	b := newTestBackoff(10*time.Second, clk)

	b.failure()
	b.failure()
	b.failure()

	clk.Advance(39 * time.Second)
	if !b.skip() {
		t.Error("skip() at +39s = false, want true: the third failure delays for 40s")
	}
	clk.Advance(1 * time.Second)
	if b.skip() {
		t.Error("skip() at +40s = true, want false")
	}
}

func TestBackoffSuccessClearsWindow(t *testing.T) {
	b := newTestBackoff(10*time.Second, testClock())

	b.failure()
	b.failure()
	b.failure()
	if !b.skip() {
		t.Fatal("skip() after three failures = false, want true")
	}

	b.success()

	if b.skip() {
		t.Error("skip() after success() = true, want false: cadence must resume immediately")
	}
	if got := b.failureCount(); got != 0 {
		t.Errorf("failureCount() after success() = %d, want 0", got)
	}
}

func TestBackoffDelayGrowthAndCap(t *testing.T) {
	b := newTestBackoff(10*time.Second, testClock())

	tests := []struct {
		failures int
		want     time.Duration
	}{
		{1, 10 * time.Second},
		{2, 20 * time.Second},
		{3, 40 * time.Second},
		{4, 80 * time.Second},
		{5, 160 * time.Second},
		{6, testBackoffMax},
		{20, testBackoffMax},
		{1000, testBackoffMax},
	}

	for _, tt := range tests {
		if got := b.delayFor(tt.failures); got != tt.want {
			t.Errorf("delayFor(%d) = %v, want %v", tt.failures, got, tt.want)
		}
	}
}

// Without --max-backoff-interval the backoff must never skip a tick, so existing
// deployments keep the cadence they have always had.
func TestBackoffDisabledByDefault(t *testing.T) {
	clk := testClock()
	b := newBackoff(10*time.Second, 0) // as if --max-backoff-interval were never passed
	b.clock = clk
	b.jitter = identityJitter

	for i := 0; i < 10; i++ {
		if failures, until := b.failure(); !until.IsZero() {
			t.Fatalf("failure() #%d opened a window until %v, want none when no max is set", failures, until)
		}
		if b.skip() {
			t.Fatalf("skip() after %d failures = true, want false when no max is set", i+1)
		}
	}

	if got := b.delayFor(5); got != 0 {
		t.Errorf("delayFor(5) with no max = %v, want 0", got)
	}

	// Advancing time must not resurrect a window either.
	clk.Advance(1 * time.Hour)
	if b.skip() {
		t.Error("skip() after an hour = true, want false")
	}
}

func TestBackoffDisabledWhenBaseIsZero(t *testing.T) {
	b := newTestBackoff(0, testClock())

	for i := 0; i < 5; i++ {
		b.failure()
	}

	if b.skip() {
		t.Error("skip() with a zero base = true, want false: the backoff must be inert before Start installs an interval")
	}
}

// An operator already polling more slowly than the cap gets no backoff, rather
// than a delay shorter than their own interval.
func TestBackoffIntervalLongerThanCap(t *testing.T) {
	base := 10 * time.Minute // longer than testBackoffMax
	b := newTestBackoff(base, testClock())

	if got := b.delayFor(5); got != base {
		t.Errorf("delayFor(5) with base %v = %v, want %v", base, got, base)
	}

	b.failure()
	b.failure()
	if b.skip() {
		t.Error("skip() = true, want false: no window should open when the cap cannot exceed base")
	}
}

// The jittered window must never fall below one polling interval, or dewy
// would poll faster while backing off.
func TestBackoffEqualJitterStaysAboveBase(t *testing.T) {
	base := 10 * time.Second

	for i := 0; i < 200; i++ {
		clk := testClock()
		start := clk.Now()
		b := newBackoff(base, testBackoffMax) // keeps the real equalJitter
		b.clock = clk

		b.failure()
		_, until := b.failure()

		got := until.Sub(start)
		// delayFor(2) is 20s, so the window is base + jitter(10s) = [15s, 20s).
		if got < 15*time.Second || got >= 20*time.Second {
			t.Fatalf("window = %v, want within [15s, 20s)", got)
		}
		if got <= base {
			t.Fatalf("window = %v, want longer than the base interval %v", got, base)
		}
	}
}

func TestEqualJitterBounds(t *testing.T) {
	if got := equalJitter(0); got != 0 {
		t.Errorf("equalJitter(0) = %v, want 0", got)
	}
	if got := equalJitter(-time.Second); got != 0 {
		t.Errorf("equalJitter(-1s) = %v, want 0", got)
	}
	if got := equalJitter(time.Nanosecond); got != time.Nanosecond {
		t.Errorf("equalJitter(1ns) = %v, want 1ns: a duration too small to halve is returned as-is", got)
	}

	for i := 0; i < 200; i++ {
		d := 40 * time.Second
		got := equalJitter(d)
		if got < d/2 || got >= d {
			t.Fatalf("equalJitter(%v) = %v, want within [%v, %v)", d, got, d/2, d)
		}
	}
}
