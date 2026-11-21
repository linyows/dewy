// Package sock gives an application the sockets that dewy opened for it.
//
// The dewy server command binds every --port before it starts the
// application and passes the sockets as inherited file descriptors, listed in
// the DEWY_PORT environment variable ("8080=3;53/udp=4"). Serving on these
// sockets, instead of binding the ports again, is what keeps the ports open
// while dewy replaces one process with the next.
//
// Listen and ListenPacket take the same arguments as net.Listen and
// net.ListenPacket. Under dewy they return the inherited socket for the
// requested port; elsewhere, such as on a developer machine, they bind the
// address themselves. The same code therefore runs in both places:
//
//	ln, err := sock.Listen("tcp", ":8080")
//	if err != nil {
//		log.Fatal(err)
//	}
//	log.Fatal(http.Serve(ln, handler))
//
// The package depends only on the standard library.
package sock

import (
	"errors"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"sync"
)

const (
	// EnvPort lists the sockets dewy passes to the application.
	EnvPort = "DEWY_PORT"
	// EnvPortCompat carries the same value as EnvPort under the name used
	// by server-starter. It is read when EnvPort is not set.
	EnvPortCompat = "SERVER_STARTER_PORT"
	// EnvHealthPort names the health check socket dewy passes to each
	// process when the server command runs with --health-before-switch.
	EnvHealthPort = "DEWY_HEALTH_PORT"

	udpSuffix = "/udp"
)

// ErrTaken is returned when an inherited socket has already been returned
// by an earlier call. A file descriptor can be turned into a socket once.
var ErrTaken = errors.New("sock: inherited socket already taken")

// Entry describes one socket listed in DEWY_PORT.
type Entry struct {
	// Addr is the address as dewy lists it, without the "/udp" suffix:
	// "8080", "127.0.0.1:8080" or "[::1]:8080".
	Addr string
	// Network is "tcp" or "udp".
	Network string
	// FD is the inherited file descriptor.
	FD uintptr
}

// Port returns the port number of the socket.
func (s Entry) Port() string {
	if _, port, err := net.SplitHostPort(s.Addr); err == nil {
		return port
	}
	return s.Addr
}

var (
	mu    sync.Mutex
	taken = map[uintptr]bool{}
)

// Inherited reports whether the process was given sockets by dewy (or by
// server-starter).
func Inherited() bool {
	_, ok := portSpec()
	return ok
}

// Entries parses DEWY_PORT, or SERVER_STARTER_PORT when DEWY_PORT is not
// set, and returns the sockets it lists in order. It returns nil when
// neither variable is set.
func Entries() ([]Entry, error) {
	spec, ok := portSpec()
	if !ok {
		return nil, nil
	}
	var socks []Entry
	for _, entry := range strings.Split(spec, ";") {
		if entry == "" {
			continue
		}
		s, err := parseEntry(entry)
		if err != nil {
			return nil, fmt.Errorf("sock: invalid %s: %w", EnvPort, err)
		}
		socks = append(socks, s)
	}
	return socks, nil
}

// Listen returns the inherited TCP socket whose port matches address, or
// calls net.Listen(network, address) when there is none.
//
// Only the port of address is compared. The interface a socket listens on is
// chosen with dewy's --host, so ":8080" and "127.0.0.1:8080" both return the
// socket dewy opened for port 8080. A port that dewy did not open, a port of
// 0 and a network other than "tcp", "tcp4" or "tcp6" are bound with
// net.Listen.
func Listen(network, address string) (net.Listener, error) {
	s, ok, err := find("tcp", network, address)
	if err != nil {
		return nil, err
	}
	if !ok {
		return net.Listen(network, address)
	}
	return fileListener(s)
}

// ListenPacket returns the inherited UDP socket whose port matches address,
// or calls net.ListenPacket(network, address) when there is none. Ports are
// matched as in Listen, for the networks "udp", "udp4" and "udp6".
func ListenPacket(network, address string) (net.PacketConn, error) {
	s, ok, err := find("udp", network, address)
	if err != nil {
		return nil, err
	}
	if !ok {
		return net.ListenPacket(network, address)
	}
	return filePacketConn(s)
}

// Listeners returns every inherited TCP socket not yet returned by an
// earlier call, in the order DEWY_PORT lists them. It returns nil when the
// process was not given sockets.
func Listeners() ([]net.Listener, error) {
	socks, err := untaken("tcp")
	if err != nil {
		return nil, err
	}
	lns := make([]net.Listener, 0, len(socks))
	for _, s := range socks {
		ln, err := fileListener(s)
		if err != nil {
			for _, l := range lns {
				_ = l.Close()
			}
			return nil, err
		}
		lns = append(lns, ln)
	}
	if len(lns) == 0 {
		return nil, nil
	}
	return lns, nil
}

// PacketConns returns every inherited UDP socket not yet returned by an
// earlier call, in the order DEWY_PORT lists them. It returns nil when the
// process was not given sockets.
func PacketConns() ([]net.PacketConn, error) {
	socks, err := untaken("udp")
	if err != nil {
		return nil, err
	}
	pcs := make([]net.PacketConn, 0, len(socks))
	for _, s := range socks {
		pc, err := filePacketConn(s)
		if err != nil {
			for _, p := range pcs {
				_ = p.Close()
			}
			return nil, err
		}
		pcs = append(pcs, pc)
	}
	if len(pcs) == 0 {
		return nil, nil
	}
	return pcs, nil
}

// HealthListener returns the health check socket named by DEWY_HEALTH_PORT.
// It returns nil when the variable is not set, which means dewy does not
// probe this process before switching to it. The application serves its
// health endpoint on the returned listener as well as on its own port.
func HealthListener() (net.Listener, error) {
	spec := os.Getenv(EnvHealthPort)
	if spec == "" {
		return nil, nil
	}
	s, err := parseEntry(spec)
	if err != nil {
		return nil, fmt.Errorf("sock: invalid %s: %w", EnvHealthPort, err)
	}
	return fileListener(s)
}

func portSpec() (string, bool) {
	if v, ok := os.LookupEnv(EnvPort); ok {
		return v, true
	}
	return os.LookupEnv(EnvPortCompat)
}

// parseEntry parses one "<address>=<fd>" entry. The address may itself hold
// '=' only in theory, so the descriptor is taken after the last one.
func parseEntry(entry string) (Entry, error) {
	i := strings.LastIndex(entry, "=")
	if i <= 0 {
		return Entry{}, fmt.Errorf("entry %q is not <address>=<fd>", entry)
	}
	fd, err := strconv.ParseUint(entry[i+1:], 10, 0)
	if err != nil {
		return Entry{}, fmt.Errorf("entry %q: bad file descriptor: %w", entry, err)
	}
	s := Entry{Addr: entry[:i], Network: "tcp", FD: uintptr(fd)}
	if a, ok := strings.CutSuffix(s.Addr, udpSuffix); ok {
		s.Addr, s.Network = a, "udp"
	}
	if s.Addr == "" {
		return Entry{}, fmt.Errorf("entry %q has no address", entry)
	}
	return s, nil
}

// find looks up the untaken inherited socket of proto for a Listen or
// ListenPacket call. ok is false when the call should bind by itself.
func find(proto, network, address string) (Entry, bool, error) {
	switch network {
	case proto, proto + "4", proto + "6":
	default:
		return Entry{}, false, nil
	}
	_, port, err := net.SplitHostPort(address)
	if err != nil || port == "" || port == "0" {
		return Entry{}, false, nil
	}
	socks, err := Entries()
	if err != nil {
		return Entry{}, false, err
	}
	for _, s := range socks {
		if s.Network == proto && s.Port() == port {
			return s, true, nil
		}
	}
	return Entry{}, false, nil
}

func untaken(proto string) ([]Entry, error) {
	socks, err := Entries()
	if err != nil {
		return nil, err
	}
	mu.Lock()
	defer mu.Unlock()
	var out []Entry
	for _, s := range socks {
		if s.Network == proto && !taken[s.FD] {
			out = append(out, s)
		}
	}
	return out, nil
}

// take marks s as returned and hands its descriptor over as a file. The
// caller converts the file to a socket and closes it.
func take(s Entry) (*os.File, error) {
	mu.Lock()
	defer mu.Unlock()
	if taken[s.FD] {
		return nil, fmt.Errorf("%w: %s", ErrTaken, s.Addr)
	}
	taken[s.FD] = true
	return os.NewFile(s.FD, s.Addr), nil
}

func fileListener(s Entry) (net.Listener, error) {
	f, err := take(s)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	ln, err := net.FileListener(f)
	if err != nil {
		return nil, fmt.Errorf("sock: file descriptor %d for %s: %w", s.FD, s.Addr, err)
	}
	return ln, nil
}

func filePacketConn(s Entry) (net.PacketConn, error) {
	f, err := take(s)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	pc, err := net.FilePacketConn(f)
	if err != nil {
		return nil, fmt.Errorf("sock: file descriptor %d for %s: %w", s.FD, s.Addr, err)
	}
	return pc, nil
}
