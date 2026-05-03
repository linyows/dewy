// Package sysdeps provides interfaces over process-level side effects so that
// the rest of dewy can be exercised without real wall-clock time, real
// environment variables, or real subprocesses. Production code uses RealClock,
// RealEnv, and RealCommandRunner; tests can swap in fakes from the
// internal/sysdeps/fake package.
package sysdeps

import "time"

// Clock abstracts wall-clock time so that callers depending on Now or on
// elapsed-time decisions can be tested deterministically.
type Clock interface {
	Now() time.Time
	NewTimer(d time.Duration) Timer
}

// Timer mirrors the relevant portion of *time.Timer. C exposes the channel
// rather than a field so that fake implementations can be channel-backed.
type Timer interface {
	C() <-chan time.Time
	Stop() bool
}

// RealClock returns a Clock backed by the time package.
func RealClock() Clock { return realClock{} }

type realClock struct{}

func (realClock) Now() time.Time { return time.Now() }
func (realClock) NewTimer(d time.Duration) Timer {
	return &realTimer{t: time.NewTimer(d)}
}

type realTimer struct{ t *time.Timer }

func (r *realTimer) C() <-chan time.Time { return r.t.C }
func (r *realTimer) Stop() bool          { return r.t.Stop() }
