package dewy

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/linyows/dewy/supervisor"
)

// errNoServer is returned when a restart is requested while no managed
// server is running, such as in assets mode or before the first start.
var errNoServer = errors.New("no managed server is running")

// errNotSwitched wraps a failed restart. Whatever the cause - the new worker
// exited within its startup window or failed the health check before the
// switch - the previous worker is still serving the previous release.
var errNotSwitched = errors.New("the new release was not switched in")

// ServerConfig describes the application the server command manages.
type ServerConfig struct {
	// Command and Args are the application and its arguments, given after
	// "--" on the command line.
	Command string
	Args    []string
	// Addrs are the listen addresses, one per --port: a bare port ("8080")
	// listens on every interface; "host:port" listens on that IP address.
	// The list is deduplicated and sorted by port.
	Addrs []string
}

// managedServer is the process supervisor as dewy uses it. It is an
// interface so tests can stand in for real worker processes.
type managedServer interface {
	Restart(ctx context.Context) error
	WaitRetired(ctx context.Context) error
	Shutdown(ctx context.Context) error
}

// supervisorConfig builds the supervisor configuration for the managed
// application.
func (d *Dewy) supervisorConfig() supervisor.Config {
	cfg := supervisor.Config{
		Logger: d.logger.Slog(),
	}
	if c := d.config.Server; c != nil {
		cfg.Command = c.Command
		cfg.Args = c.Args
		cfg.Addrs = c.Addrs
	}
	return cfg
}

// startServer binds the listen addresses, runs the managed application and
// marks d.isServerRunning. It returns an error when an address cannot be
// bound or the first worker exits within its startup window; nothing is left
// running or bound in that case.
func (d *Dewy) startServer(ctx context.Context) error {
	d.RLock()
	ver := d.cVer
	running := d.server != nil
	d.RUnlock()
	if running {
		return errors.New("managed server is already running")
	}

	d.logger.Info("Start server", slog.String("version", ver))

	s, err := supervisor.New(d.supervisorConfig())
	if err != nil {
		return err
	}
	if err := s.Start(ctx); err != nil {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), defaultServerStopTimeout)
		defer cancel()
		_ = s.Shutdown(shutdownCtx)
		return err
	}

	d.Lock()
	d.server = s
	d.isServerRunning = true
	d.Unlock()
	return nil
}

// restartServer replaces the running worker with one that runs the release
// "current" points at now. It returns an error when the new worker exits
// within its startup window; the previous worker then keeps serving.
func (d *Dewy) restartServer(ctx context.Context) error {
	d.RLock()
	s := d.server
	ver := d.cVer
	d.RUnlock()
	if s == nil {
		return errNoServer
	}

	d.logger.Info("Restart server", slog.String("version", ver))
	if err := s.Restart(ctx); err != nil {
		return fmt.Errorf("%w: %w", errNotSwitched, err)
	}
	return nil
}

// stopServer stops the managed application and releases its listen
// addresses. Workers still running after defaultServerStopTimeout are killed. It is a no-op when no server is running.
func (d *Dewy) stopServer() error {
	d.Lock()
	s := d.server
	d.server = nil
	d.isServerRunning = false
	d.Unlock()
	if s == nil {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), defaultServerStopTimeout)
	defer cancel()
	return s.Shutdown(ctx)
}

// awaitWorkerSwap waits until every worker older than the current one has
// exited.
//
// Without it the health check is meaningless on a restart. A restart starts
// the new worker beside the old one, both serving on the same listening
// socket, and only then tells the old one to stop. Until the old worker has
// exited it may still accept connections, so a probe sent in that window is
// as likely to be answered by the release being replaced as by the new one.
//
// It reports whether the swap was observed. A false return means no server
// is running or an old worker is still alive after budget, and the caller
// probes anyway rather than failing a deploy over a slow shutdown.
func (d *Dewy) awaitWorkerSwap(ctx context.Context, budget time.Duration) bool {
	d.RLock()
	s := d.server
	d.RUnlock()
	if s == nil {
		return false
	}

	ctx, cancel := context.WithTimeout(ctx, budget)
	defer cancel()
	if err := s.WaitRetired(ctx); err != nil {
		d.logger.Warn("Timed out waiting for the previous worker to exit; the health check may reach the previous release",
			slog.Duration("waited", budget))
		return false
	}
	return true
}
