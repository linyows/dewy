package dewy

import (
	"context"
	"errors"
	"os/exec"
	"testing"
)

func TestServerLifecycle(t *testing.T) {
	sleep, err := exec.LookPath("sleep")
	if err != nil {
		t.Skip("sleep is not available")
	}

	d := newPhaseTestDewy(t)
	d.config.Command = SERVER
	d.config.Server = &ServerConfig{Command: sleep, Args: []string{"30"}, Addrs: []string{"127.0.0.1:0"}}
	t.Cleanup(func() { _ = d.stopServer() })

	ctx := context.Background()
	if err := d.restartServer(ctx); !errors.Is(err, errNoServer) {
		t.Fatalf("restartServer() before start = %v, want errNoServer", err)
	}

	if err := d.startServer(ctx); err != nil {
		t.Fatalf("startServer() = %v", err)
	}
	if !d.isServerRunning {
		t.Error("isServerRunning = false after a successful start")
	}
	if err := d.startServer(ctx); err == nil {
		t.Error("a second startServer() = nil, want an error while the server runs")
	}

	if err := d.restartServer(ctx); err != nil {
		t.Fatalf("restartServer() = %v", err)
	}
	if !d.awaitWorkerSwap(ctx, defaultWorkerSwapTimeout) {
		t.Error("awaitWorkerSwap() = false, want the previous worker gone after the restart")
	}

	if err := d.stopServer(); err != nil {
		t.Fatalf("stopServer() = %v", err)
	}
	if d.isServerRunning || d.server != nil {
		t.Error("the server is still recorded as running after stopServer")
	}
}

func TestStartServerFailureReleasesPorts(t *testing.T) {
	d := newPhaseTestDewy(t)
	d.config.Command = SERVER
	// "false" exits at once, inside the startup window.
	d.config.Server = &ServerConfig{Command: "false", Addrs: []string{"127.0.0.1:0"}}

	if err := d.startServer(context.Background()); err == nil {
		t.Fatal("startServer() = nil, want an error when the application exits at once")
	}
	if d.isServerRunning || d.server != nil {
		t.Error("a failed start left the server recorded as running")
	}
}
