package supervisor

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

// The test binary doubles as the worker. When helperEnv is set, TestMain runs
// the worker instead of the tests. The behavior is read from the file named
// by modeFileEnv at each start, so a test can change what the next worker
// does, as a new release would.
const (
	helperEnv   = "SUPERVISOR_TEST_HELPER"
	modeFileEnv = "SUPERVISOR_TEST_MODE_FILE"
)

func TestMain(m *testing.M) {
	if os.Getenv(helperEnv) == "1" {
		os.Exit(runHelper())
	}
	os.Exit(m.Run())
}

// runHelper is the worker. Modes:
//
//	serve        answer HTTP on every inherited listener with
//	             "<mode> <pid>"; GET /exit makes it exit 1; SIGTERM makes it
//	             exit 0
//	crash        exit 1 at once
//	ignore-term  like serve, but ignore SIGTERM
func runHelper() int {
	mode := "serve"
	if b, err := os.ReadFile(os.Getenv(modeFileEnv)); err == nil { //nolint:gosec // G703: the path is a temp file the test itself names
		mode = strings.TrimSpace(string(b))
	}
	if mode == "crash" {
		return 1
	}

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGTERM)

	mux := http.NewServeMux()
	mux.HandleFunc("/exit", func(http.ResponseWriter, *http.Request) { os.Exit(1) })
	mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprintf(w, "%s %d", mode, os.Getpid())
	})

	spec := os.Getenv(EnvListeners)
	if compat := os.Getenv(EnvListenersCompat); compat != spec {
		return 3
	}
	if spec != "" {
		for entry := range strings.SplitSeq(spec, ";") {
			i := strings.LastIndex(entry, "=")
			fd, err := strconv.Atoi(entry[i+1:])
			if err != nil {
				return 2
			}
			ln, err := net.FileListener(os.NewFile(uintptr(fd), entry[:i]))
			if err != nil {
				return 2
			}
			go serve(ln, mux)
		}
	}

	for range sigs {
		if mode != "ignore-term" {
			return 0
		}
	}
	return 0
}

type harness struct {
	t        *testing.T
	modeFile string
}

// serve serves h on ln for the helper worker until the process exits.
func serve(ln net.Listener, h http.Handler) {
	srv := &http.Server{Handler: h, ReadHeaderTimeout: 5 * time.Second}
	_ = srv.Serve(ln)
}

func newHarness(t *testing.T, mode string) *harness {
	t.Helper()
	h := &harness{t: t, modeFile: filepath.Join(t.TempDir(), "mode")}
	h.setMode(mode)
	return h
}

func (h *harness) setMode(mode string) {
	h.t.Helper()
	if err := os.WriteFile(h.modeFile, []byte(mode), 0600); err != nil {
		h.t.Fatal(err)
	}
}

func (h *harness) config(addrs ...string) Config {
	return Config{
		Command:       os.Args[0],
		Env:           []string{helperEnv + "=1", modeFileEnv + "=" + h.modeFile},
		Addrs:         addrs,
		StartupWindow: 200 * time.Millisecond,
		Stdout:        io.Discard,
		Stderr:        io.Discard,
	}
}

func (h *harness) start(cfg Config) *Supervisor {
	h.t.Helper()
	s, err := New(cfg)
	if err != nil {
		h.t.Fatalf("New() error = %v", err)
	}
	h.t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = s.Shutdown(ctx)
	})
	if err := s.Start(context.Background()); err != nil {
		h.t.Fatalf("Start() error = %v", err)
	}
	return s
}

// get returns the body served at addr.
func get(t *testing.T, addr net.Addr) string {
	t.Helper()
	c := http.Client{Timeout: 2 * time.Second}
	res, err := c.Get("http://" + addr.String() + "/")
	if err != nil {
		t.Fatalf("GET %s: %v", addr, err)
	}
	defer func() { _ = res.Body.Close() }()
	b, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func eventually(t *testing.T, timeout time.Duration, cond func() bool) bool {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if cond() {
			return true
		}
		time.Sleep(50 * time.Millisecond)
	}
	return cond()
}

func ipv6Available() bool {
	ln, err := net.Listen("tcp6", "[::1]:0")
	if err != nil {
		return false
	}
	_ = ln.Close()
	return true
}

func TestResolveAddr(t *testing.T) {
	tests := []struct {
		addr        string
		wantNetwork string
		wantAddress string
		wantErr     bool
	}{
		{"8080", "tcp", ":8080", false},
		{"0", "tcp", ":0", false},
		{"127.0.0.1:8080", "tcp4", "127.0.0.1:8080", false},
		{"0.0.0.0:8080", "tcp4", "0.0.0.0:8080", false},
		{"[::1]:8080", "tcp6", "[::1]:8080", false},
		{"[::]:8080", "tcp6", "[::]:8080", false},
		{"[fe80::1%eth0]:8080", "tcp6", "[fe80::1%eth0]:8080", false},
		{"[::ffff:127.0.0.1]:8080", "tcp6", "[::ffff:127.0.0.1]:8080", false},
		{"65536", "", "", true},
		{"-1", "", "", true},
		{"", "", "", true},
		{"localhost:8080", "", "", true},
		{"127.0.0.1", "", "", true},
		{"::1:8080", "", "", true},
		{"127.0.0.1:http", "", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.addr, func(t *testing.T) {
			network, address, err := resolveAddr(tt.addr)
			if (err != nil) != tt.wantErr {
				t.Fatalf("resolveAddr(%q) error = %v, wantErr %v", tt.addr, err, tt.wantErr)
			}
			if network != tt.wantNetwork || address != tt.wantAddress {
				t.Errorf("resolveAddr(%q) = %q, %q, want %q, %q", tt.addr, network, address, tt.wantNetwork, tt.wantAddress)
			}
		})
	}
}

func TestListenersEnv(t *testing.T) {
	lns := []*listener{
		{addr: "8080"}, {addr: "127.0.0.1:8081"}, {addr: "[::1]:8082"},
		{addr: "0.0.0.0:8083"}, {addr: "[::]:8084"},
	}
	want := "8080=3;127.0.0.1:8081=4;[::1]:8082=5;8083=6;8084=7"
	if got := listenersEnv(lns); got != want {
		t.Errorf("listenersEnv() = %q, want %q", got, want)
	}
}

func TestUnspecifiedHostNarrowsFamily(t *testing.T) {
	s, err := New(Config{Command: "true", Addrs: []string{"0.0.0.0:0", "0"}})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer func() { _ = s.Shutdown(context.Background()) }()

	v4, ok := s.Addrs()[0].(*net.TCPAddr)
	if !ok || v4.IP.To4() == nil {
		t.Errorf("0.0.0.0:0 bound %v, want an IPv4 socket", v4)
	}
	probe, err := net.Listen("tcp6", "[::1]:0")
	if err != nil {
		t.Skip("IPv6 is not available")
	}
	_ = probe.Close()
	if both, ok := s.Addrs()[1].(*net.TCPAddr); !ok || both.IP.To4() != nil {
		t.Errorf("0 bound %v, want a dual-stack IPv6 socket", both)
	}
}

func TestNew(t *testing.T) {
	t.Run("command is required", func(t *testing.T) {
		if _, err := New(Config{Addrs: []string{"127.0.0.1:0"}}); err == nil {
			t.Error("New() error = nil, want an error without a command")
		}
	})

	t.Run("an address in use fails and releases the others", func(t *testing.T) {
		busy, err := net.Listen("tcp4", "127.0.0.1:0")
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = busy.Close() }()

		free, err := net.Listen("tcp4", "127.0.0.1:0")
		if err != nil {
			t.Fatal(err)
		}
		freeAddr := free.Addr().String()
		_ = free.Close()

		if _, err := New(Config{Command: "true", Addrs: []string{freeAddr, busy.Addr().String()}}); err == nil {
			t.Fatal("New() error = nil, want an error for an address in use")
		}
		ln, err := net.Listen("tcp4", freeAddr)
		if err != nil {
			t.Fatalf("the first address was left bound after the failure: %v", err)
		}
		_ = ln.Close()
	})

	t.Run("duplicate address", func(t *testing.T) {
		if _, err := New(Config{Command: "true", Addrs: []string{"127.0.0.1:0", "127.0.0.1:0"}}); err == nil {
			t.Error("New() error = nil, want an error for a duplicate address")
		}
	})
}

func TestStartServesOnInheritedListeners(t *testing.T) {
	addrs := []string{"127.0.0.1:0", "0"}
	if ipv6Available() {
		addrs = append(addrs, "[::1]:0")
	}
	h := newHarness(t, "serve")
	s := h.start(h.config(addrs...))

	for _, addr := range s.Addrs() {
		if a, ok := addr.(*net.TCPAddr); ok && a.IP.IsUnspecified() {
			addr = &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1), Port: a.Port}
		}
		if got := get(t, addr); !strings.HasPrefix(got, "serve ") {
			t.Errorf("GET %s = %q, want a response from the worker", addr, got)
		}
	}

	st := s.Status()
	if st.Current != 1 || len(st.Workers) != 1 || st.Workers[0].Generation != 1 {
		t.Errorf("Status() = %+v, want one worker of generation 1", st)
	}
}

func TestStartFailsWhenWorkerExits(t *testing.T) {
	h := newHarness(t, "crash")
	s, err := New(h.config("127.0.0.1:0"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = s.Shutdown(context.Background()) }()

	if err := s.Start(context.Background()); err == nil {
		t.Error("Start() error = nil, want an error when the worker exits during the startup window")
	}
}

func TestStartFailsWhenCommandIsMissing(t *testing.T) {
	s, err := New(Config{Command: filepath.Join(t.TempDir(), "absent")})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = s.Shutdown(context.Background()) }()

	if err := s.Start(context.Background()); err == nil {
		t.Error("Start() error = nil, want an error for a missing command")
	}
}

func TestRestartReplacesWorker(t *testing.T) {
	h := newHarness(t, "serve")
	s := h.start(h.config("127.0.0.1:0"))
	addr := s.Addrs()[0]
	before := get(t, addr)

	h.setMode("serve-v2")
	if err := s.Restart(context.Background()); err != nil {
		t.Fatalf("Restart() error = %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := s.WaitRetired(ctx); err != nil {
		t.Fatalf("WaitRetired() error = %v", err)
	}

	after := get(t, addr)
	if !strings.HasPrefix(after, "serve-v2 ") || after == before {
		t.Errorf("after Restart got %q (before %q), want the new worker", after, before)
	}
	st := s.Status()
	if st.Current != 2 || len(st.Workers) != 1 || st.Workers[0].Generation != 2 {
		t.Errorf("Status() = %+v, want only generation 2", st)
	}
}

func TestRestartKeepsCurrentWorkerWhenNewOneExits(t *testing.T) {
	h := newHarness(t, "serve")
	s := h.start(h.config("127.0.0.1:0"))
	addr := s.Addrs()[0]
	before := get(t, addr)

	h.setMode("crash")
	if err := s.Restart(context.Background()); err == nil {
		t.Fatal("Restart() error = nil, want an error when the new worker exits")
	}

	if got := get(t, addr); got != before {
		t.Errorf("after a failed Restart got %q, want the previous worker %q", got, before)
	}
	if st := s.Status(); st.Current != 1 || len(st.Workers) != 1 {
		t.Errorf("Status() = %+v, want only generation 1", st)
	}
}

func TestRestartCanceled(t *testing.T) {
	h := newHarness(t, "serve")
	s := h.start(h.config("127.0.0.1:0"))

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := s.Restart(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("Restart() error = %v, want context.Canceled", err)
	}
	if st := s.Status(); st.Current != 1 || len(st.Workers) != 1 {
		t.Errorf("Status() = %+v, want only generation 1", st)
	}
}

func TestCrashedWorkerIsRestarted(t *testing.T) {
	h := newHarness(t, "serve")
	s := h.start(h.config("127.0.0.1:0"))
	addr := s.Addrs()[0]

	c := http.Client{Timeout: time.Second}
	if res, err := c.Get("http://" + addr.String() + "/exit"); err == nil {
		_ = res.Body.Close()
	}

	if !eventually(t, 5*time.Second, func() bool {
		st := s.Status()
		return st.Current == 2 && len(st.Workers) == 1
	}) {
		t.Fatalf("Status() = %+v, want the crashed worker replaced by generation 2", s.Status())
	}
	if got := get(t, addr); !strings.HasPrefix(got, "serve ") {
		t.Errorf("GET after recovery = %q, want a response from the new worker", got)
	}
}

func TestShutdown(t *testing.T) {
	t.Run("stops workers and closes listeners", func(t *testing.T) {
		h := newHarness(t, "serve")
		s := h.start(h.config("127.0.0.1:0"))
		addr := s.Addrs()[0].String()

		if err := s.Shutdown(context.Background()); err != nil {
			t.Fatalf("Shutdown() error = %v", err)
		}
		if st := s.Status(); len(st.Workers) != 0 {
			t.Errorf("Status() = %+v, want no workers", st)
		}
		ln, err := net.Listen("tcp4", addr)
		if err != nil {
			t.Fatalf("the listener is still bound after Shutdown: %v", err)
		}
		_ = ln.Close()

		if err := s.Restart(context.Background()); err == nil {
			t.Error("Restart() after Shutdown error = nil, want an error")
		}
		if err := s.Shutdown(context.Background()); err != nil {
			t.Errorf("second Shutdown() error = %v, want nil", err)
		}
	})

	t.Run("kills a worker that ignores the stop signal", func(t *testing.T) {
		h := newHarness(t, "ignore-term")
		s := h.start(h.config("127.0.0.1:0"))

		ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
		defer cancel()
		if err := s.Shutdown(ctx); !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("Shutdown() error = %v, want context.DeadlineExceeded", err)
		}
		if st := s.Status(); len(st.Workers) != 0 {
			t.Errorf("Status() = %+v, want no workers", st)
		}
	})
}

func TestWithoutListeners(t *testing.T) {
	h := newHarness(t, "serve")
	s := h.start(h.config())
	if len(s.Addrs()) != 0 {
		t.Errorf("Addrs() = %v, want none", s.Addrs())
	}
	if err := s.Restart(context.Background()); err != nil {
		t.Fatalf("Restart() error = %v", err)
	}
}

func TestBackoff(t *testing.T) {
	tests := []struct {
		n    int
		want time.Duration
	}{
		{1, time.Second},
		{2, 2 * time.Second},
		{3, 4 * time.Second},
		{5, 16 * time.Second},
		{6, 30 * time.Second},
		{100, 30 * time.Second},
	}
	for _, tt := range tests {
		if got := backoff(tt.n); got != tt.want {
			t.Errorf("backoff(%d) = %s, want %s", tt.n, got, tt.want)
		}
	}
}
