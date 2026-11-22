// Package supervisor runs an application as a worker process that serves on
// listening sockets owned by the supervisor, and replaces the worker without
// closing those sockets.
//
// The supervisor binds every listen address once, in New, and keeps the
// sockets open until Shutdown. Each worker inherits them as file descriptors
// 3, 4, ... and learns which descriptor belongs to which address from the
// EnvListeners environment variable (also set as EnvListenersCompat).
// Because the sockets outlive the workers, a connection that arrives while
// one worker is being replaced by the next is queued by the kernel instead
// of being refused.
//
// A replacement (Restart) starts the new worker next to the current one and
// stops the current one only after the new worker has stayed up for the
// startup window and, when a health check is configured, has passed it on a
// port of its own. A new worker that exits within that window or fails the
// health check is reported as an error and the current worker keeps serving.
// A replaced worker that has not exited after the drain timeout is killed. A current worker that exits
// on its own is started again after a backoff.
package supervisor

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"os"
	"os/exec"
	"slices"
	"sort"
	"sync"
	"syscall"
	"time"
)

const (
	// DefaultStartupWindow is how long a new worker must stay up before it
	// is considered started.
	DefaultStartupWindow = time.Second

	// respawnBaseDelay and respawnMaxDelay bound the backoff before a
	// crashed worker is started again. The delay doubles with each crash in
	// a row, up to the maximum.
	respawnBaseDelay = time.Second
	respawnMaxDelay  = 30 * time.Second

	// respawnResetAfter is how long a worker must have run for its exit to
	// count as a first crash again, resetting the backoff.
	respawnResetAfter = 30 * time.Second
)

// ErrShutdown is returned by Start and Restart once Shutdown has begun.
var ErrShutdown = errors.New("supervisor is shut down")

// ErrUnhealthy is wrapped by the error Start and Restart return when the new
// worker fails the health check.
var ErrUnhealthy = errors.New("worker failed the health check")

// HealthCheckFunc probes one worker through addr, the loopback address of the
// health socket only that worker serves. It returns nil once the worker is
// healthy, and an error when it gives up or ctx ends.
type HealthCheckFunc func(ctx context.Context, addr net.Addr) error

// Config describes the managed application.
type Config struct {
	// Command is the executable to run. A name without a slash is looked up
	// in PATH; a relative path is resolved against Dir. It is resolved again
	// for every worker, so a symlink such as "current/app" picks up the
	// release it points to at the time of each start.
	Command string
	// Args are the arguments passed to Command.
	Args []string
	// Dir is the working directory of each worker. Empty means the
	// supervisor's working directory.
	Dir string
	// Env is appended to the supervisor's environment for each worker.
	Env []string
	// Addrs are the listen addresses: "8080", "127.0.0.1:8080" or
	// "[::1]:8080" for TCP, and the same with a "/udp" suffix for UDP
	// ("53/udp", "[::1]:53/udp"). An absolute path ("/run/app.sock") is a
	// Unix domain socket, and on Linux so is a name starting with "@"
	// ("@app"), which lives in the abstract namespace. Empty runs the
	// application without sockets.
	//
	// A socket file left at the path by a process that exited without
	// removing it is replaced; any other file there is an error. The file
	// is removed on Shutdown.
	Addrs []string
	// StartupWindow is how long a new worker must stay up before it is
	// considered started. Zero means DefaultStartupWindow.
	StartupWindow time.Duration
	// HealthCheck, when set, gates every new worker: after the startup
	// window the worker must pass it before it becomes current. Each worker
	// then gets a TCP socket of its own on the loopback interface, announced
	// in EnvHealthPort, and the check probes that socket. Because no other
	// worker shares it, the result is the new worker's own, whereas a probe
	// through a shared listener may be answered by the worker being
	// replaced.
	HealthCheck HealthCheckFunc
	// DrainTimeout bounds how long a replaced worker may take to exit after
	// the stop signal before it is killed. Zero waits for it indefinitely.
	DrainTimeout time.Duration
	// StopSignal is sent to a worker that is being replaced or shut down.
	// Zero means SIGTERM.
	StopSignal syscall.Signal
	// Stdout and Stderr receive the workers' output. Nil means the
	// supervisor's own stdout and stderr.
	Stdout io.Writer
	Stderr io.Writer
	// Logger records worker lifecycle events. Nil discards them.
	Logger *slog.Logger
	// OnCrash, when set, is called each time the current worker exits
	// without being told to stop, before the supervisor starts a
	// replacement. err is the exit status. It runs on the goroutine that
	// waited for the process, so it must return quickly.
	OnCrash func(w Worker, err error)
}

// Worker describes one live worker process.
type Worker struct {
	Generation int
	PID        int
	StartedAt  time.Time
}

// Status is a snapshot of the live workers.
type Status struct {
	// Current is the generation of the worker that serves traffic, or 0
	// when there is none.
	Current int
	// Workers lists every live worker, oldest first. During a replacement
	// it includes the previous worker until that one exits.
	Workers []Worker
}

// worker is one run of the application.
type worker struct {
	gen     int
	cmd     *exec.Cmd
	started time.Time
	done    chan struct{} // closed once the process has exited
	err     error         // exit status; valid after done is closed
	health  net.Addr      // address of the worker's health socket, or nil

	// stopping is set, under Supervisor.mu, when the supervisor asks the
	// worker to exit, so its exit is not mistaken for a crash.
	stopping bool
}

// Supervisor runs and replaces the workers of one application.
type Supervisor struct {
	cfg       Config
	log       *slog.Logger
	listeners []*listener
	env       []string

	// opMu serializes operations that start workers (Start, Restart and
	// crash recovery), so two of them never race to become current.
	opMu sync.Mutex

	mu       sync.Mutex
	started  bool
	closing  bool
	closed   chan struct{} // closed when Shutdown begins
	gen      int
	current  *worker
	workers  map[*worker]struct{} // live workers
	crashes  int                  // consecutive crashes, for the backoff
	shutOnce sync.Once
}

// New validates cfg and binds the listen addresses. No worker runs until
// Start. Binding here, rather than in Start, reports an address that is in
// use before anything is spawned.
func New(cfg Config) (*Supervisor, error) {
	if cfg.Command == "" {
		return nil, errors.New("supervisor: command is required")
	}
	if cfg.StartupWindow <= 0 {
		cfg.StartupWindow = DefaultStartupWindow
	}
	if cfg.StopSignal == 0 {
		cfg.StopSignal = syscall.SIGTERM
	}
	if cfg.Stdout == nil {
		cfg.Stdout = os.Stdout
	}
	if cfg.Stderr == nil {
		cfg.Stderr = os.Stderr
	}
	log := cfg.Logger
	if log == nil {
		log = slog.New(slog.DiscardHandler)
	}

	lns, err := openListeners(cfg.Addrs)
	if err != nil {
		return nil, err
	}

	env := append(os.Environ(), cfg.Env...)
	if len(lns) > 0 {
		v := listenersEnv(lns)
		env = append(env, EnvListeners+"="+v, EnvListenersCompat+"="+v)
	}

	return &Supervisor{
		cfg:       cfg,
		log:       log,
		listeners: lns,
		env:       env,
		closed:    make(chan struct{}),
		workers:   make(map[*worker]struct{}),
	}, nil
}

// Addrs returns the bound address of each socket, in configuration order: a
// *net.TCPAddr for a TCP address, a *net.UDPAddr for a UDP one and a
// *net.UnixAddr for a Unix domain socket.
// It differs from Config.Addrs when a port of 0 was configured.
func (s *Supervisor) Addrs() []net.Addr {
	addrs := make([]net.Addr, len(s.listeners))
	for i, l := range s.listeners {
		addrs[i] = l.local
	}
	return addrs
}

// Start runs the first worker and waits for it to stay up for the startup
// window and to pass the health check, if one is configured. It returns an
// error if the worker cannot be spawned, exits within the window or fails the
// check; the caller should then call Shutdown to release the listeners.
func (s *Supervisor) Start(ctx context.Context) error {
	s.opMu.Lock()
	defer s.opMu.Unlock()

	s.mu.Lock()
	if s.started {
		s.mu.Unlock()
		return errors.New("supervisor: already started")
	}
	s.started = true
	s.mu.Unlock()

	w, err := s.spawn()
	if err != nil {
		return err
	}
	if err := s.admit(ctx, w); err != nil {
		return err
	}

	s.mu.Lock()
	s.current = w
	s.mu.Unlock()
	s.log.Info("Worker started", slog.Int("generation", w.gen), slog.Int("pid", w.cmd.Process.Pid))
	return nil
}

// Restart replaces the current worker. It starts a new worker, waits for it
// to stay up for the startup window and to pass the health check, if one is
// configured, makes it current and sends the stop signal to every older
// worker. It returns once the older workers have been signaled, without
// waiting for them to exit; see WaitRetired. An older worker still running
// after the drain timeout is killed.
//
// If the new worker exits within the window, fails the health check, or ctx
// ends first, the new worker is stopped, the current worker keeps serving
// and an error is returned.
func (s *Supervisor) Restart(ctx context.Context) error {
	s.opMu.Lock()
	defer s.opMu.Unlock()

	s.mu.Lock()
	if !s.started {
		s.mu.Unlock()
		return errors.New("supervisor: not started")
	}
	s.mu.Unlock()

	w, err := s.spawn()
	if err != nil {
		return err
	}
	if err := s.admit(ctx, w); err != nil {
		return err
	}

	s.mu.Lock()
	if s.closing {
		s.mu.Unlock()
		return ErrShutdown
	}
	prev := s.current
	s.current = w
	s.crashes = 0
	retiring := s.othersLocked(w)
	for _, o := range retiring {
		o.stopping = true
	}
	s.mu.Unlock()

	attrs := []any{slog.Int("generation", w.gen), slog.Int("pid", w.cmd.Process.Pid)}
	if prev != nil {
		attrs = append(attrs, slog.Int("previous_generation", prev.gen))
	}
	s.log.Info("Worker replaced", attrs...)

	for _, o := range retiring {
		s.signal(o, s.cfg.StopSignal)
		if s.cfg.DrainTimeout > 0 {
			go s.drain(o)
		}
	}
	return nil
}

// drain kills w if it is still running after the drain timeout.
func (s *Supervisor) drain(w *worker) {
	t := time.NewTimer(s.cfg.DrainTimeout)
	defer t.Stop()
	select {
	case <-w.done:
	case <-t.C:
		s.log.Warn("Killing worker that did not exit within the drain time",
			slog.Int("generation", w.gen),
			slog.Int("pid", w.cmd.Process.Pid),
			slog.Duration("drain_time", s.cfg.DrainTimeout))
		s.signal(w, syscall.SIGKILL)
	}
}

// WaitRetired waits until every worker other than the current one has
// exited. After a Restart this is the point from which every accepted
// connection is handled by the new worker.
func (s *Supervisor) WaitRetired(ctx context.Context) error {
	s.mu.Lock()
	others := s.othersLocked(s.current)
	s.mu.Unlock()

	for _, w := range others {
		select {
		case <-w.done:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return nil
}

// Status returns a snapshot of the live workers.
func (s *Supervisor) Status() Status {
	s.mu.Lock()
	defer s.mu.Unlock()

	var st Status
	if s.current != nil {
		st.Current = s.current.gen
	}
	for w := range s.workers {
		st.Workers = append(st.Workers, Worker{
			Generation: w.gen,
			PID:        w.cmd.Process.Pid,
			StartedAt:  w.started,
		})
	}
	sort.Slice(st.Workers, func(i, j int) bool {
		return st.Workers[i].Generation < st.Workers[j].Generation
	})
	return st
}

// Shutdown sends the stop signal to every worker and waits for them to exit.
// Workers still running when ctx ends are killed. The listeners are closed
// last, so the port stays bound until no worker can use it. Calling it again
// is a no-op that returns nil.
func (s *Supervisor) Shutdown(ctx context.Context) error {
	first := false
	s.shutOnce.Do(func() { first = true })
	if !first {
		return nil
	}

	s.mu.Lock()
	s.closing = true
	close(s.closed)
	live := s.othersLocked(nil)
	for _, w := range live {
		w.stopping = true
	}
	s.mu.Unlock()

	for _, w := range live {
		s.signal(w, s.cfg.StopSignal)
	}

	var err error
	for _, w := range live {
		select {
		case <-w.done:
			continue
		case <-ctx.Done():
		}
		err = ctx.Err()
		s.log.Warn("Killing worker that did not stop in time",
			slog.Int("generation", w.gen), slog.Int("pid", w.cmd.Process.Pid))
		s.signal(w, syscall.SIGKILL)
		<-w.done
	}

	for _, l := range s.listeners {
		l.close()
	}
	return err
}

// spawn starts a new worker process. The worker is registered as live but
// is not made current; the caller decides that.
//
// With a health check configured, the worker also inherits a health socket
// of its own, placed after the shared sockets. The supervisor closes its copy
// once the worker has started, so the socket lives exactly as long as the
// worker holds it.
func (s *Supervisor) spawn() (*worker, error) {
	cmd := exec.Command(s.cfg.Command, s.cfg.Args...) //nolint:gosec // G204: the command is the operator's own application
	cmd.Dir = s.cfg.Dir
	cmd.Env = s.env
	cmd.Stdout = s.cfg.Stdout
	cmd.Stderr = s.cfg.Stderr
	for _, l := range s.listeners {
		cmd.ExtraFiles = append(cmd.ExtraFiles, l.file)
	}

	var health *listener
	if s.cfg.HealthCheck != nil {
		var err error
		if health, err = openHealthListener(); err != nil {
			return nil, fmt.Errorf("open health socket: %w", err)
		}
		defer health.close()
		fd := firstListenerFD + len(cmd.ExtraFiles)
		cmd.ExtraFiles = append(cmd.ExtraFiles, health.file)
		cmd.Env = append(slices.Clip(cmd.Env), EnvHealthPort+"="+healthEnv(health.local, fd))
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closing {
		return nil, ErrShutdown
	}
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start worker: %w", err)
	}
	s.gen++
	w := &worker{
		gen:     s.gen,
		cmd:     cmd,
		started: time.Now(),
		done:    make(chan struct{}),
	}
	if health != nil {
		w.health = health.local
	}
	s.workers[w] = struct{}{}
	go s.wait(w)
	return w, nil
}

// admit decides whether the new worker w may become current: it must stay up
// for the startup window and then pass the health check, if one is
// configured. A worker that is not admitted has been stopped by the time
// admit returns.
func (s *Supervisor) admit(ctx context.Context, w *worker) error {
	if err := s.settle(ctx, w); err != nil {
		return err
	}
	if s.cfg.HealthCheck == nil {
		return nil
	}

	// End the probe as soon as the worker exits or Shutdown begins, rather
	// than letting it run out its budget against a dead socket.
	hctx, cancel := context.WithCancel(ctx)
	defer cancel()
	go func() {
		select {
		case <-w.done:
		case <-s.closed:
		case <-hctx.Done():
		}
		cancel()
	}()

	err := s.cfg.HealthCheck(hctx, w.health)
	if err == nil {
		s.log.Debug("Worker passed the health check", slog.Int("generation", w.gen))
		return nil
	}

	select {
	case <-w.done:
		return fmt.Errorf("worker exited during the health check: %w", exitError(w.err))
	default:
	}
	s.kill(w)
	select {
	case <-s.closed:
		return ErrShutdown
	default:
	}
	if ctx.Err() != nil {
		return ctx.Err()
	}
	return fmt.Errorf("%w: %w", ErrUnhealthy, err)
}

// kill stops a worker that was never made current and waits for it to exit.
func (s *Supervisor) kill(w *worker) {
	s.mu.Lock()
	w.stopping = true
	s.mu.Unlock()
	s.signal(w, syscall.SIGKILL)
	<-w.done
}

// settle waits for w to stay up for the startup window. If w exits first,
// or ctx ends first, w is stopped and an error is returned.
func (s *Supervisor) settle(ctx context.Context, w *worker) error {
	t := time.NewTimer(s.cfg.StartupWindow)
	defer t.Stop()

	select {
	case <-t.C:
		return nil
	case <-w.done:
		return fmt.Errorf("worker exited during the %s startup window: %w", s.cfg.StartupWindow, exitError(w.err))
	case <-ctx.Done():
	case <-s.closed:
	}

	s.kill(w)
	if ctx.Err() != nil {
		return ctx.Err()
	}
	return ErrShutdown
}

// wait reaps w and decides whether its exit was a crash of the current
// worker that must be recovered.
func (s *Supervisor) wait(w *worker) {
	err := w.cmd.Wait()

	s.mu.Lock()
	w.err = err
	delete(s.workers, w)
	crashed := s.current == w && !w.stopping && !s.closing
	stopping := w.stopping
	s.mu.Unlock()
	close(w.done)

	attrs := []any{
		slog.Int("generation", w.gen),
		slog.Int("pid", w.cmd.Process.Pid),
		slog.String("status", exitError(err).Error()),
	}
	switch {
	case crashed:
		s.log.Warn("Worker exited unexpectedly", attrs...)
		if s.cfg.OnCrash != nil {
			s.cfg.OnCrash(Worker{Generation: w.gen, PID: w.cmd.Process.Pid, StartedAt: w.started}, exitError(err))
		}
		go s.recover(w)
	case stopping:
		s.log.Info("Worker stopped", attrs...)
	}
	// A worker that is neither current nor stopping is still settling;
	// settle reports its exit.
}

// recover starts a replacement for the crashed current worker after a
// backoff. It gives up when Shutdown begins or when another operation has
// already replaced the crashed worker.
func (s *Supervisor) recover(dead *worker) {
	if time.Since(dead.started) >= respawnResetAfter {
		s.mu.Lock()
		s.crashes = 0
		s.mu.Unlock()
	}

	for {
		s.mu.Lock()
		s.crashes++
		delay := backoff(s.crashes)
		s.mu.Unlock()

		s.log.Info("Restarting worker after backoff",
			slog.Int("crashed_generation", dead.gen), slog.Duration("delay", delay))

		t := time.NewTimer(delay)
		select {
		case <-t.C:
		case <-s.closed:
			t.Stop()
			return
		}

		s.opMu.Lock()
		s.mu.Lock()
		replaced := s.current != dead || s.closing
		s.mu.Unlock()
		if replaced {
			s.opMu.Unlock()
			return
		}

		w, err := s.spawn()
		if err != nil {
			s.opMu.Unlock()
			if errors.Is(err, ErrShutdown) {
				return
			}
			s.log.Error("Worker restart failure", slog.String("error", err.Error()))
			continue
		}

		// The replacement becomes current at once, without a startup
		// window: nothing else is serving. If it crashes too, wait starts
		// another recovery with a longer backoff.
		s.mu.Lock()
		s.current = w
		s.mu.Unlock()
		s.opMu.Unlock()
		s.log.Info("Worker started", slog.Int("generation", w.gen), slog.Int("pid", w.cmd.Process.Pid))
		return
	}
}

// othersLocked returns the live workers other than except. s.mu must be held.
func (s *Supervisor) othersLocked(except *worker) []*worker {
	var ws []*worker
	for w := range s.workers {
		if w != except {
			ws = append(ws, w)
		}
	}
	return ws
}

// signal sends sig to w, ignoring a process that has already exited.
func (s *Supervisor) signal(w *worker, sig os.Signal) {
	if err := w.cmd.Process.Signal(sig); err != nil && !errors.Is(err, os.ErrProcessDone) {
		s.log.Warn("Failed to signal worker",
			slog.Int("generation", w.gen),
			slog.String("signal", sig.String()),
			slog.String("error", err.Error()))
	}
}

// backoff returns the delay before the n-th consecutive restart (n >= 1).
func backoff(n int) time.Duration {
	d := respawnBaseDelay
	for i := 1; i < n && d < respawnMaxDelay; i++ {
		d *= 2
	}
	return min(d, respawnMaxDelay)
}

// exitError turns a nil Wait result into a readable status.
func exitError(err error) error {
	if err == nil {
		return errors.New("exit status 0")
	}
	return err
}
