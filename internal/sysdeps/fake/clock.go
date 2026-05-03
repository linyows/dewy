// Package fake provides deterministic implementations of the sysdeps
// interfaces for use in tests.
package fake

import (
	"sync"
	"time"

	"github.com/linyows/dewy/internal/sysdeps"
)

// Clock is a manually-advanced sysdeps.Clock. Tests advance time with Advance
// or Set; timers created via NewTimer fire when their deadline is reached by
// the clock's current time.
type Clock struct {
	mu     sync.Mutex
	now    time.Time
	timers []*fakeTimer
}

// NewClock returns a Clock fixed at the given time.
func NewClock(start time.Time) *Clock {
	return &Clock{now: start}
}

// Now returns the clock's current time.
func (c *Clock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

// Set jumps the clock to t and fires any timers whose deadline is at or before t.
func (c *Clock) Set(t time.Time) {
	c.mu.Lock()
	c.now = t
	expired := c.collectExpiredLocked()
	c.mu.Unlock()
	fireAll(expired)
}

// Advance moves the clock forward by d and fires any newly-expired timers.
func (c *Clock) Advance(d time.Duration) {
	c.mu.Lock()
	c.now = c.now.Add(d)
	expired := c.collectExpiredLocked()
	c.mu.Unlock()
	fireAll(expired)
}

// NewTimer creates a fake timer that fires when the clock reaches the deadline.
func (c *Clock) NewTimer(d time.Duration) sysdeps.Timer {
	c.mu.Lock()
	defer c.mu.Unlock()
	t := &fakeTimer{
		deadline: c.now.Add(d),
		ch:       make(chan time.Time, 1),
		clock:    c,
	}
	c.timers = append(c.timers, t)
	return t
}

func (c *Clock) collectExpiredLocked() []*fakeTimer {
	var fired []*fakeTimer
	remaining := c.timers[:0]
	for _, t := range c.timers {
		if t.stopped {
			continue
		}
		if !t.deadline.After(c.now) {
			t.stopped = true
			t.fireTime = c.now
			fired = append(fired, t)
			continue
		}
		remaining = append(remaining, t)
	}
	c.timers = remaining
	return fired
}

func fireAll(fired []*fakeTimer) {
	for _, t := range fired {
		select {
		case t.ch <- t.fireTime:
		default:
		}
	}
}

type fakeTimer struct {
	deadline time.Time
	fireTime time.Time
	ch       chan time.Time
	clock    *Clock
	stopped  bool
}

func (t *fakeTimer) C() <-chan time.Time { return t.ch }

func (t *fakeTimer) Stop() bool {
	t.clock.mu.Lock()
	defer t.clock.mu.Unlock()
	if t.stopped {
		return false
	}
	t.stopped = true
	return true
}

// Compile-time check.
var _ sysdeps.Clock = (*Clock)(nil)
