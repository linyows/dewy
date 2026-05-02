package sysdeps_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/linyows/dewy/internal/sysdeps"
	"github.com/linyows/dewy/internal/sysdeps/fake"
)

func TestRealClockNow(t *testing.T) {
	c := sysdeps.RealClock()
	before := time.Now()
	got := c.Now()
	after := time.Now()
	if got.Before(before) || got.After(after) {
		t.Errorf("RealClock.Now() = %v, expected within [%v, %v]", got, before, after)
	}
}

func TestRealClockTimerFires(t *testing.T) {
	c := sysdeps.RealClock()
	timer := c.NewTimer(5 * time.Millisecond)
	defer timer.Stop()
	select {
	case <-timer.C():
	case <-time.After(time.Second):
		t.Fatal("RealClock timer did not fire within 1s")
	}
}

func TestFakeClockAdvanceFiresTimer(t *testing.T) {
	clk := fake.NewClock(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))
	timer := clk.NewTimer(time.Second)

	select {
	case <-timer.C():
		t.Fatal("timer fired before advance")
	default:
	}

	clk.Advance(time.Second)

	select {
	case <-timer.C():
	case <-time.After(time.Second):
		t.Fatal("timer did not fire after advance")
	}
}

func TestFakeClockStopPreventsFire(t *testing.T) {
	clk := fake.NewClock(time.Now())
	timer := clk.NewTimer(time.Second)
	if !timer.Stop() {
		t.Error("expected Stop to report timer was active")
	}
	clk.Advance(time.Second)
	select {
	case <-timer.C():
		t.Error("stopped timer should not fire")
	case <-time.After(20 * time.Millisecond):
	}
}

func TestFakeEnv(t *testing.T) {
	e := fake.NewEnv().Set("FOO", "bar").SetHostname("h1")
	if got := e.Get("FOO"); got != "bar" {
		t.Errorf("Get(FOO) = %q, want bar", got)
	}
	if got := e.Get("MISSING"); got != "" {
		t.Errorf("Get(MISSING) = %q, want empty", got)
	}
	host, err := e.Hostname()
	if err != nil || host != "h1" {
		t.Errorf("Hostname() = (%q, %v), want (h1, nil)", host, err)
	}

	wantErr := errors.New("boom")
	e.SetHostnameError(wantErr)
	if _, err := e.Hostname(); !errors.Is(err, wantErr) {
		t.Errorf("Hostname() err = %v, want %v", err, wantErr)
	}
}

func TestFakeCommandRunner(t *testing.T) {
	r := fake.NewCommandRunner().SetOutput("docker", []byte("ok")).SetPath("docker", "/usr/bin/docker")

	out, err := r.Output(context.Background(), "docker", "ps")
	if err != nil || string(out) != "ok" {
		t.Errorf("Output() = (%q, %v), want (ok, nil)", out, err)
	}

	path, err := r.LookPath("docker")
	if err != nil || path != "/usr/bin/docker" {
		t.Errorf("LookPath() = (%q, %v), want (/usr/bin/docker, nil)", path, err)
	}

	if _, err := r.LookPath("unknown"); err == nil {
		t.Error("LookPath(unknown) should error")
	}

	if calls := r.Calls(); len(calls) != 1 || calls[0].Name != "docker" || calls[0].Args[0] != "ps" {
		t.Errorf("Calls() = %+v", calls)
	}

	wantErr := errors.New("permission denied")
	r.SetError("rm", wantErr)
	if err := r.Run(context.Background(), "rm", "-rf"); !errors.Is(err, wantErr) {
		t.Errorf("Run(rm) err = %v, want %v", err, wantErr)
	}
}

func TestFakeCommandRunnerCancel(t *testing.T) {
	r := fake.NewCommandRunner().SetOutput("any", []byte("x"))
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := r.Output(ctx, "any"); !errors.Is(err, context.Canceled) {
		t.Errorf("Output() with canceled ctx err = %v, want Canceled", err)
	}
}
