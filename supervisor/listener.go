package supervisor

import (
	"fmt"
	"io"
	"net"
	"net/netip"
	"os"
	"strconv"
	"strings"
)

// EnvListeners names the environment variable that lists the inherited
// listening sockets for the worker.
//
// The value is a ";"-separated list of "<address>=<fd>" entries, one per
// listener, in the order the addresses were configured. <address> is the
// address exactly as configured ("8080", "127.0.0.1:8080", "[::1]:8080") and
// <fd> is the file descriptor number the socket has in the worker.
const EnvListeners = "DEWY_PORT"

// EnvListenersCompat carries the same value as EnvListeners under the name
// server-starter uses. The format is compatible with server-starter, so an
// application written for it keeps working unchanged.
const EnvListenersCompat = "SERVER_STARTER_PORT"

// firstListenerFD is the descriptor number of the first inherited socket.
// 0, 1 and 2 are stdin, stdout and stderr; exec.Cmd.ExtraFiles places the
// remaining files from 3 on.
const firstListenerFD = 3

// listener is a listening socket the supervisor owns for the whole life of
// the managed application. Workers come and go; the socket stays open, so a
// connection arriving during a worker swap waits in the kernel's accept queue
// instead of being refused.
type listener struct {
	addr  string // as configured; used verbatim in EnvListeners
	local net.Addr
	sock  io.Closer // the net.Listener
	file  *os.File  // dup of the socket's descriptor, handed to each worker
}

func (l *listener) close() {
	_ = l.file.Close()
	_ = l.sock.Close()
}

// resolveAddr maps a configured address to the network and address passed to
// net.Listen.
//
//   - "8080" listens on every interface. The "tcp" network is dual stack: Go
//     opens an IPv6 socket that also accepts IPv4 connections, and falls back
//     to IPv4 alone on a host without IPv6.
//   - "127.0.0.1:8080" listens on that IPv4 address only ("tcp4").
//   - "[::1]:8080" or "[fe80::1%eth0]:8080" listens on that IPv6 address only
//     ("tcp6").
//   - "0.0.0.0:8080" listens on every IPv4 interface ("tcp4") and "[::]:8080"
//     on every IPv6 interface ("tcp6", which Go opens with IPV6_V6ONLY).
//
// The host part must be an IP literal. A hostname may resolve to several
// addresses, or to a different one on each start, and would make the bound
// interface depend on DNS.
func resolveAddr(addr string) (network, address string, err error) {
	if isPort(addr) {
		return "tcp", ":" + addr, nil
	}

	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return "", "", fmt.Errorf("invalid listen address %q: %w", addr, err)
	}
	if !isPort(port) {
		return "", "", fmt.Errorf("invalid listen address %q: bad port %q", addr, port)
	}
	ip, err := netip.ParseAddr(host)
	if err != nil {
		return "", "", fmt.Errorf("invalid listen address %q: host must be an IP address", addr)
	}
	if ip.Is4() {
		return "tcp4", net.JoinHostPort(ip.String(), port), nil
	}
	return "tcp6", net.JoinHostPort(ip.String(), port), nil
}

// isPort reports whether s is a decimal port number in 0-65535. Port 0 asks
// the kernel for a free port, which is useful in tests.
func isPort(s string) bool {
	if s == "" || strings.TrimLeft(s, "0123456789") != "" {
		return false
	}
	n, err := strconv.Atoi(s)
	return err == nil && n <= 65535
}

// openListeners binds every configured address. On failure the sockets
// already opened are closed, so a failed start leaves no port held.
func openListeners(addrs []string) ([]*listener, error) {
	lns := make([]*listener, 0, len(addrs))
	fail := func(err error) ([]*listener, error) {
		for _, l := range lns {
			l.close()
		}
		return nil, err
	}

	seen := make(map[string]bool, len(addrs))
	for _, addr := range addrs {
		if seen[addr] {
			return fail(fmt.Errorf("duplicate listen address %q", addr))
		}
		seen[addr] = true

		network, address, err := resolveAddr(addr)
		if err != nil {
			return fail(err)
		}
		l, err := bind(network, address)
		if err != nil {
			return fail(fmt.Errorf("listen %s: %w", addr, err))
		}
		l.addr = addr
		lns = append(lns, l)
	}
	return lns, nil
}

// filer is implemented by *net.TCPListener.
type filer interface {
	File() (*os.File, error)
}

// bind opens the socket for network and address and duplicates its
// descriptor for the workers.
func bind(network, address string) (*listener, error) {
	ln, err := net.Listen(network, address)
	if err != nil {
		return nil, err
	}
	var sock io.Closer = ln
	local := ln.Addr()

	fl, ok := sock.(filer)
	if !ok {
		_ = sock.Close()
		return nil, fmt.Errorf("%T cannot hand over its descriptor", sock)
	}
	f, err := fl.File()
	if err != nil {
		_ = sock.Close()
		return nil, err
	}
	return &listener{local: local, sock: sock, file: f}, nil
}

// listenersEnv builds the EnvListeners value for lns.
func listenersEnv(lns []*listener) string {
	entries := make([]string, len(lns))
	for i, l := range lns {
		entries[i] = envAddr(l.addr) + "=" + strconv.Itoa(firstListenerFD+i)
	}
	return strings.Join(entries, ";")
}

// envAddr returns how addr is listed in DEWY_PORT. An unspecified host
// ("0.0.0.0:8080" or "[::]:8080") is listed as the bare port, the same as an
// address without a host, so an application that reads only the port keeps
// working when the address family is narrowed. Other addresses are listed as
// configured.
func envAddr(addr string) string {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return addr
	}
	if ip, err := netip.ParseAddr(host); err != nil || !ip.IsUnspecified() {
		return addr
	}
	return port
}
