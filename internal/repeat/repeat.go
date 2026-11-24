// Package repeat runs a function repeatedly at a fixed interval.
package repeat

import (
	"errors"
	"sync"
	"time"
)

// ErrInvalidInterval is returned by Every for an interval that is not positive.
var ErrInvalidInterval = errors.New("repeat: interval must be positive")

// Loop is a running repetition started by Every.
type Loop struct {
	stop     chan struct{}
	done     chan struct{}
	stopOnce sync.Once
}

// Every calls job once right away and then once at every interval, until
// Stop is called.
//
// The calls are made one at a time from a single goroutine, so job never runs
// concurrently with itself. The call times are fixed at start, start+interval,
// start+2*interval, and so on. When a call runs past one or more of those
// times, the times it ran past are dropped, and the next call is made at the
// first time still ahead. A long call therefore does not cause a burst of
// calls after it.
func Every(interval time.Duration, job func()) (*Loop, error) {
	if interval <= 0 {
		return nil, ErrInvalidInterval
	}
	if job == nil {
		return nil, errors.New("repeat: job must not be nil")
	}

	l := &Loop{
		stop: make(chan struct{}),
		done: make(chan struct{}),
	}
	go l.run(interval, job)
	return l, nil
}

// Stop ends the repetition. No call of job starts after Stop returns, and a
// call in progress has finished by then. Stop may be called more than once.
// It must not be called from within job, because it would wait for job to
// return.
func (l *Loop) Stop() {
	l.stopOnce.Do(func() { close(l.stop) })
	<-l.done
}

func (l *Loop) run(interval time.Duration, job func()) {
	defer close(l.done)

	next := time.Now()
	for {
		// Stop takes priority over a call that is due at the same time.
		select {
		case <-l.stop:
			return
		default:
		}

		job()

		next = nextAfter(next, interval, time.Now())
		timer := time.NewTimer(time.Until(next))
		select {
		case <-l.stop:
			timer.Stop()
			return
		case <-timer.C:
		}
	}
}

// nextAfter returns the first time in the sequence prev+interval,
// prev+2*interval, ... that is after now.
func nextAfter(prev time.Time, interval time.Duration, now time.Time) time.Time {
	next := prev.Add(interval)
	if next.After(now) {
		return next
	}
	missed := now.Sub(next)/interval + 1
	return next.Add(missed * interval)
}
