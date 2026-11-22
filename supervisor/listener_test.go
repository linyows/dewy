package supervisor

import (
	"context"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

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
		{"53/udp", "udp", ":53", false},
		{"127.0.0.1:53/udp", "udp4", "127.0.0.1:53", false},
		{"[::1]:53/udp", "udp6", "[::1]:53", false},
		{"53/sctp", "", "", true},
		{"localhost:53/udp", "", "", true},
		{"/run/app.sock", "unix", "/run/app.sock", false},
		{"/srv/udp", "unix", "/srv/udp", false},
		{"/run/app;1.sock", "", "", true},
		{"/run/a=b.sock", "", "", true},
		{"/run/../app.sock", "", "", true},
		{"/run/", "", "", true},
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
		{addr: "8080"}, {addr: "127.0.0.1:8081"}, {addr: "[::1]:8082"}, {addr: "53/udp"},
		{addr: "0.0.0.0:8083"}, {addr: "[::]:8084"}, {addr: "0.0.0.0:54/udp"},
	}
	lns = append(lns, &listener{addr: "/run/app.sock"}, &listener{addr: "/srv/udp"})
	want := "8080=3;127.0.0.1:8081=4;[::1]:8082=5;53/udp=6;8083=7;8084=8;54/udp=9;/run/app.sock=10;/srv/udp=11"
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

func TestResolveAbstractAddr(t *testing.T) {
	network, address, err := resolveAddr("@dewy-test")
	if runtime.GOOS != "linux" {
		if err == nil {
			t.Errorf("resolveAddr(@dewy-test) = %q, %q, want an error outside Linux", network, address)
		}
		return
	}
	if err != nil || network != "unix" || address != "@dewy-test" {
		t.Errorf("resolveAddr(@dewy-test) = %q, %q, %v, want unix, @dewy-test", network, address, err)
	}
	if _, _, err := resolveAddr("@"); err == nil {
		t.Error("resolveAddr(@) error = nil, want an error for an empty abstract name")
	}
}

func TestUnixSocketServesAcrossRestart(t *testing.T) {
	path := filepath.Join(shortTempDir(t), "app.sock")
	h := newHarness(t, "serve")
	s := h.start(h.config("127.0.0.1:0", path))

	ua, ok := s.Addrs()[1].(*net.UnixAddr)
	if !ok || ua.Name != path {
		t.Fatalf("Addrs()[1] = %#v, want the Unix socket %s", s.Addrs()[1], path)
	}
	if got := getPath(t, ua, "/env/"+EnvListeners); !strings.HasSuffix(got, ";"+path+"=4") {
		t.Errorf("%s = %q, want the socket listed as %s=4", EnvListeners, got, path)
	}
	before := get(t, ua)
	if !strings.HasPrefix(before, "serve ") {
		t.Fatalf("GET over %s = %q, want a response from the worker", path, before)
	}

	h.setMode("serve-v2")
	if err := s.Restart(context.Background()); err != nil {
		t.Fatalf("Restart() error = %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := s.WaitRetired(ctx); err != nil {
		t.Fatalf("WaitRetired() error = %v", err)
	}
	// The retired worker closed its copy of the listener; the path must
	// still lead to the socket.
	if after := get(t, ua); !strings.HasPrefix(after, "serve-v2 ") {
		t.Errorf("after Restart GET over %s = %q, want the new worker", path, after)
	}

	if err := s.Shutdown(context.Background()); err != nil {
		t.Fatalf("Shutdown() error = %v", err)
	}
	if _, err := os.Lstat(path); !os.IsNotExist(err) {
		t.Errorf("socket file after Shutdown: %v, want it removed", err)
	}
}

func TestUnixSocketPathInUse(t *testing.T) {
	dir := shortTempDir(t)

	t.Run("a stale socket file is replaced", func(t *testing.T) {
		path := filepath.Join(dir, "stale.sock")
		ln, err := net.Listen("unix", path)
		if err != nil {
			t.Fatal(err)
		}
		ul, ok := ln.(*net.UnixListener)
		if !ok {
			t.Fatalf("listener is %T, not *net.UnixListener", ln)
		}
		ul.SetUnlinkOnClose(false)
		_ = ul.Close()

		s, err := New(Config{Command: "true", Addrs: []string{path}})
		if err != nil {
			t.Fatalf("New() error = %v, want the stale socket replaced", err)
		}
		_ = s.Shutdown(context.Background())
	})

	t.Run("a live socket is an error", func(t *testing.T) {
		path := filepath.Join(dir, "live.sock")
		ln, err := net.Listen("unix", path)
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = ln.Close() }()

		if s, err := New(Config{Command: "true", Addrs: []string{path}}); err == nil {
			_ = s.Shutdown(context.Background())
			t.Fatal("New() error = nil, want an error for a socket in use")
		}
		if c, err := net.Dial("unix", path); err != nil {
			t.Errorf("the live socket no longer accepts connections: %v", err)
		} else {
			_ = c.Close()
		}
	})

	t.Run("a regular file is an error and is kept", func(t *testing.T) {
		path := filepath.Join(dir, "file.sock")
		if err := os.WriteFile(path, []byte("x"), 0600); err != nil {
			t.Fatal(err)
		}
		if s, err := New(Config{Command: "true", Addrs: []string{path}}); err == nil {
			_ = s.Shutdown(context.Background())
			t.Fatal("New() error = nil, want an error for a regular file at the path")
		}
		if _, err := os.Stat(path); err != nil {
			t.Errorf("the regular file was removed: %v", err)
		}
	})
}

func TestShutdownKeepsReplacedSocketFile(t *testing.T) {
	path := filepath.Join(shortTempDir(t), "app.sock")
	s, err := New(Config{Command: "true", Addrs: []string{path}})
	if err != nil {
		t.Fatal(err)
	}

	// Another process takes the path over while the supervisor runs.
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("someone else"), 0600); err != nil {
		t.Fatal(err)
	}

	_ = s.Shutdown(context.Background())
	if b, err := os.ReadFile(path); err != nil || string(b) != "someone else" {
		t.Errorf("after Shutdown the file at the path is %q, %v, want it untouched", b, err)
	}
}

// shortTempDir returns a directory for Unix domain sockets. t.TempDir is
// too long on macOS for the 104-byte limit on a socket path.
func shortTempDir(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp("/tmp", "dewy-sv")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	return dir
}
