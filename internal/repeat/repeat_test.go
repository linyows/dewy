package repeat

import (
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

func TestEveryRejectsInvalidInterval(t *testing.T) {
	for _, d := range []time.Duration{0, -time.Second} {
		if _, err := Every(d, func() {}); !errors.Is(err, ErrInvalidInterval) {
			t.Errorf("Every(%v) error = %v, want ErrInvalidInterval", d, err)
		}
	}
}

func TestEveryRejectsNilJob(t *testing.T) {
	if _, err := Every(time.Second, nil); err == nil {
		t.Error("Every with a nil job returned no error")
	}
}

func TestEveryRunsImmediately(t *testing.T) {
	called := make(chan struct{}, 1)
	l, err := Every(time.Hour, func() {
		select {
		case called <- struct{}{}:
		default:
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	defer l.Stop()

	select {
	case <-called:
	case <-time.After(5 * time.Second):
		t.Fatal("job was not called right after Every")
	}
}

func TestEveryRepeats(t *testing.T) {
	var calls atomic.Int32
	reached := make(chan struct{})
	l, err := Every(10*time.Millisecond, func() {
		if calls.Add(1) == 3 {
			close(reached)
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	defer l.Stop()

	select {
	case <-reached:
	case <-time.After(5 * time.Second):
		t.Fatalf("job was called %d times, want at least 3", calls.Load())
	}
}

func TestEveryDoesNotOverlap(t *testing.T) {
	var running, overlaps, calls atomic.Int32
	l, err := Every(time.Millisecond, func() {
		if running.Add(1) > 1 {
			overlaps.Add(1)
		}
		time.Sleep(5 * time.Millisecond)
		running.Add(-1)
		calls.Add(1)
	})
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(50 * time.Millisecond)
	l.Stop()

	if got := overlaps.Load(); got != 0 {
		t.Errorf("job overlapped itself %d times, want 0", got)
	}
	if calls.Load() == 0 {
		t.Error("job was never called")
	}
}

func TestStopWaitsForRunningJob(t *testing.T) {
	entered := make(chan struct{})
	release := make(chan struct{})
	var finished atomic.Bool
	l, err := Every(time.Hour, func() {
		close(entered)
		<-release
		finished.Store(true)
	})
	if err != nil {
		t.Fatal(err)
	}
	<-entered

	stopped := make(chan struct{})
	go func() {
		l.Stop()
		close(stopped)
	}()

	select {
	case <-stopped:
		t.Fatal("Stop returned while the job was running")
	case <-time.After(50 * time.Millisecond):
	}

	close(release)
	select {
	case <-stopped:
	case <-time.After(5 * time.Second):
		t.Fatal("Stop did not return after the job finished")
	}
	if !finished.Load() {
		t.Error("Stop returned before the job finished")
	}
}

func TestNoCallAfterStop(t *testing.T) {
	var calls atomic.Int32
	l, err := Every(time.Millisecond, func() { calls.Add(1) })
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(10 * time.Millisecond)
	l.Stop()
	after := calls.Load()

	time.Sleep(20 * time.Millisecond)
	if got := calls.Load(); got != after {
		t.Errorf("job was called %d times after Stop, want 0", got-after)
	}
}

func TestStopIsIdempotent(t *testing.T) {
	l, err := Every(time.Hour, func() {})
	if err != nil {
		t.Fatal(err)
	}
	l.Stop()
	l.Stop()
}

func TestNextAfter(t *testing.T) {
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	interval := 10 * time.Second
	tests := []struct {
		name string
		now  time.Time
		want time.Time
	}{
		{"job finished early", base.Add(3 * time.Second), base.Add(10 * time.Second)},
		{"job finished exactly on the next time", base.Add(10 * time.Second), base.Add(20 * time.Second)},
		{"job ran past one time", base.Add(15 * time.Second), base.Add(20 * time.Second)},
		{"job ran past several times", base.Add(47 * time.Second), base.Add(50 * time.Second)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := nextAfter(base, interval, tt.now); !got.Equal(tt.want) {
				t.Errorf("nextAfter = %v, want %v", got, tt.want)
			}
		})
	}
}
