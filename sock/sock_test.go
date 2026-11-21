//go:build unix

package sock

import (
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"
	"syscall"
	"testing"
)

// reset forgets the descriptors returned so far, so each test starts clean.
func reset(t *testing.T) {
	t.Helper()
	mu.Lock()
	taken = map[uintptr]bool{}
	mu.Unlock()
}

// inheritTCP binds a TCP socket and returns a duplicate of its descriptor,
// standing in for one dewy passed down. The package takes ownership of the
// duplicate; the original listener is closed by the test cleanup.
func inheritTCP(t *testing.T) (port string, fd int, addr string) {
	t.Helper()
	ln, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ln.Close() })
	addr = ln.Addr().String()
	_, port, _ = net.SplitHostPort(addr)
	return port, dupFD(t, ln.(*net.TCPListener)), addr
}

func inheritUDP(t *testing.T) (port string, fd int, addr string) {
	t.Helper()
	pc, err := net.ListenPacket("udp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = pc.Close() })
	addr = pc.LocalAddr().String()
	_, port, _ = net.SplitHostPort(addr)
	return port, dupFD(t, pc.(*net.UDPConn)), addr
}

type syscallConn interface {
	SyscallConn() (syscall.RawConn, error)
}

// dupFD duplicates the descriptor of c.
func dupFD(t *testing.T, c syscallConn) int {
	t.Helper()
	rc, err := c.SyscallConn()
	if err != nil {
		t.Fatal(err)
	}
	var fd int
	var dupErr error
	if err := rc.Control(func(s uintptr) { fd, dupErr = syscall.Dup(int(s)) }); err != nil {
		t.Fatal(err)
	}
	if dupErr != nil {
		t.Fatal(dupErr)
	}
	return fd
}

func TestParseEntry(t *testing.T) {
	tests := []struct {
		in      string
		want    Entry
		wantErr bool
	}{
		{"8080=3", Entry{Addr: "8080", Network: "tcp", FD: 3}, false},
		{"127.0.0.1:8080=4", Entry{Addr: "127.0.0.1:8080", Network: "tcp", FD: 4}, false},
		{"[::1]:8080=5", Entry{Addr: "[::1]:8080", Network: "tcp", FD: 5}, false},
		{"53/udp=6", Entry{Addr: "53", Network: "udp", FD: 6}, false},
		{"[fe80::1%eth0]:53/udp=7", Entry{Addr: "[fe80::1%eth0]:53", Network: "udp", FD: 7}, false},
		{"8080", Entry{}, true},
		{"=3", Entry{}, true},
		{"8080=x", Entry{}, true},
		{"/udp=3", Entry{}, true},
	}
	for _, tt := range tests {
		got, err := parseEntry(tt.in)
		if (err != nil) != tt.wantErr {
			t.Errorf("parseEntry(%q) error = %v, wantErr %v", tt.in, err, tt.wantErr)
			continue
		}
		if got != tt.want {
			t.Errorf("parseEntry(%q) = %+v, want %+v", tt.in, got, tt.want)
		}
	}
}

func TestEntries(t *testing.T) {
	t.Run("not inherited", func(t *testing.T) {
		unsetenv(t, EnvPort)
		unsetenv(t, EnvPortCompat)
		if Inherited() {
			t.Error("Inherited() = true without either variable")
		}
		got, err := Entries()
		if err != nil || got != nil {
			t.Errorf("Entries() = %v, %v, want nil, nil", got, err)
		}
	})

	t.Run("DEWY_PORT", func(t *testing.T) {
		t.Setenv(EnvPort, "8080=3;53/udp=4")
		t.Setenv(EnvPortCompat, "9999=9")
		got, err := Entries()
		if err != nil {
			t.Fatal(err)
		}
		want := []Entry{{Addr: "8080", Network: "tcp", FD: 3}, {Addr: "53", Network: "udp", FD: 4}}
		if fmt.Sprint(got) != fmt.Sprint(want) {
			t.Errorf("Entries() = %+v, want %+v", got, want)
		}
	})

	t.Run("SERVER_STARTER_PORT when DEWY_PORT is unset", func(t *testing.T) {
		unsetenv(t, EnvPort)
		t.Setenv(EnvPortCompat, "127.0.0.1:8080=3")
		got, err := Entries()
		if err != nil || len(got) != 1 || got[0].Addr != "127.0.0.1:8080" || got[0].Port() != "8080" {
			t.Errorf("Entries() = %+v, %v, want the SERVER_STARTER_PORT entry", got, err)
		}
	})

	t.Run("malformed", func(t *testing.T) {
		t.Setenv(EnvPort, "8080=3;bad")
		if _, err := Entries(); err == nil {
			t.Error("Entries() error = nil, want an error for a malformed entry")
		}
		if _, err := Listen("tcp", ":8080"); err == nil {
			t.Error("Listen() error = nil, want the parse error")
		}
	})
}

// unsetenv removes key for the rest of the test; t.Setenv restores it.
func unsetenv(t *testing.T, key string) {
	t.Helper()
	t.Setenv(key, "")
	if err := syscall.Unsetenv(key); err != nil {
		t.Fatal(err)
	}
}

func TestListenInherited(t *testing.T) {
	reset(t)
	port, fd, addr := inheritTCP(t)
	t.Setenv(EnvPort, port+"="+strconv.Itoa(fd))

	// A different host still selects the socket by port.
	ln, err := Listen("tcp", "0.0.0.0:"+port)
	if err != nil {
		t.Fatalf("Listen() error = %v", err)
	}
	defer func() { _ = ln.Close() }()
	if got := ln.Addr().String(); got != addr {
		t.Errorf("Listen() returned %s, want the inherited socket %s", got, addr)
	}

	go func() {
		c, err := ln.Accept()
		if err == nil {
			_, _ = c.Write([]byte("ok"))
			_ = c.Close()
		}
	}()
	c, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = c.Close() }()
	buf := make([]byte, 2)
	if _, err := c.Read(buf); err != nil || string(buf) != "ok" {
		t.Errorf("read %q, %v from the inherited socket, want ok", buf, err)
	}

	if _, err := Listen("tcp", ":"+port); !errors.Is(err, ErrTaken) {
		t.Errorf("second Listen() error = %v, want ErrTaken", err)
	}
	if lns, err := Listeners(); err != nil || lns != nil {
		t.Errorf("Listeners() after Listen = %v, %v, want nothing left", lns, err)
	}
}

func TestListenFallsBack(t *testing.T) {
	reset(t)
	port, fd, _ := inheritTCP(t)
	t.Setenv(EnvPort, port+"="+strconv.Itoa(fd))

	tests := []struct{ network, address string }{
		{"tcp", "127.0.0.1:0"},  // port 0
		{"tcp4", "127.0.0.1:0"}, // port 0 with a narrowed network
		{"unix", ""},            // another network
	}
	for _, tt := range tests {
		if tt.network == "unix" {
			tt.address = t.TempDir() + "/s"
		}
		ln, err := Listen(tt.network, tt.address)
		if err != nil {
			t.Errorf("Listen(%q, %q) error = %v", tt.network, tt.address, err)
			continue
		}
		if strings.HasSuffix(ln.Addr().String(), ":"+port) {
			t.Errorf("Listen(%q, %q) returned the inherited socket", tt.network, tt.address)
		}
		_ = ln.Close()
	}

	// A port dewy did not open is bound directly.
	free, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	freeAddr := free.Addr().String()
	_ = free.Close()
	ln, err := Listen("tcp4", freeAddr)
	if err != nil {
		t.Fatalf("Listen(%q) error = %v", freeAddr, err)
	}
	_ = ln.Close()
}

func TestListenWithoutDewy(t *testing.T) {
	reset(t)
	unsetenv(t, EnvPort)
	unsetenv(t, EnvPortCompat)
	ln, err := Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Listen() error = %v", err)
	}
	_ = ln.Close()
	pc, err := ListenPacket("udp4", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("ListenPacket() error = %v", err)
	}
	_ = pc.Close()
	if lns, err := Listeners(); err != nil || lns != nil {
		t.Errorf("Listeners() = %v, %v, want nil, nil", lns, err)
	}
}

func TestListenPacketInherited(t *testing.T) {
	reset(t)
	tport, tfd, _ := inheritTCP(t)
	uport, ufd, uaddr := inheritUDP(t)
	t.Setenv(EnvPort, fmt.Sprintf("%s=%d;%s/udp=%d", tport, tfd, uport, ufd))

	pc, err := ListenPacket("udp", ":"+uport)
	if err != nil {
		t.Fatalf("ListenPacket() error = %v", err)
	}
	defer func() { _ = pc.Close() }()
	if got := pc.LocalAddr().String(); got != uaddr {
		t.Errorf("ListenPacket() returned %s, want the inherited socket %s", got, uaddr)
	}

	// The TCP entry of another port is not a UDP socket and is not matched.
	if pcs, err := PacketConns(); err != nil || pcs != nil {
		t.Errorf("PacketConns() = %v, %v, want nothing left", pcs, err)
	}
	lns, err := Listeners()
	if err != nil || len(lns) != 1 {
		t.Fatalf("Listeners() = %v, %v, want the TCP socket", lns, err)
	}
	_ = lns[0].Close()
}

func TestListenersAndPacketConns(t *testing.T) {
	reset(t)
	p1, fd1, a1 := inheritTCP(t)
	p2, fd2, a2 := inheritTCP(t)
	p3, fd3, a3 := inheritUDP(t)
	t.Setenv(EnvPort, fmt.Sprintf("%s=%d;%s=%d;%s/udp=%d", p1, fd1, p2, fd2, p3, fd3))

	lns, err := Listeners()
	if err != nil {
		t.Fatal(err)
	}
	if len(lns) != 2 || lns[0].Addr().String() != a1 || lns[1].Addr().String() != a2 {
		t.Errorf("Listeners() = %v, want %s and %s in order", lns, a1, a2)
	}
	for _, l := range lns {
		_ = l.Close()
	}

	pcs, err := PacketConns()
	if err != nil {
		t.Fatal(err)
	}
	if len(pcs) != 1 || pcs[0].LocalAddr().String() != a3 {
		t.Errorf("PacketConns() = %v, want %s", pcs, a3)
	}
	for _, p := range pcs {
		_ = p.Close()
	}
}

func TestHealthListener(t *testing.T) {
	reset(t)

	t.Run("unset", func(t *testing.T) {
		unsetenv(t, EnvHealthPort)
		ln, err := HealthListener()
		if err != nil || ln != nil {
			t.Errorf("HealthListener() = %v, %v, want nil, nil", ln, err)
		}
	})

	t.Run("set", func(t *testing.T) {
		_, fd, addr := inheritTCP(t)
		t.Setenv(EnvHealthPort, addr+"="+strconv.Itoa(fd))
		ln, err := HealthListener()
		if err != nil {
			t.Fatalf("HealthListener() error = %v", err)
		}
		defer func() { _ = ln.Close() }()
		if got := ln.Addr().String(); got != addr {
			t.Errorf("HealthListener() returned %s, want %s", got, addr)
		}
	})

	t.Run("malformed", func(t *testing.T) {
		t.Setenv(EnvHealthPort, "127.0.0.1:1")
		if _, err := HealthListener(); err == nil {
			t.Error("HealthListener() error = nil, want an error")
		}
	})
}
