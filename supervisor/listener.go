package supervisor

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net"
	"net/netip"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// EnvListeners names the environment variable that lists the inherited
// listening sockets for the worker.
//
// The value is a ";"-separated list of "<address>=<fd>" entries, one per
// listener, in the order the addresses were configured. <address> is the
// address exactly as configured ("8080", "127.0.0.1:8080", "[::1]:8080") and
// <fd> is the file descriptor number the socket has in the worker. A UDP
// socket is marked with a "/udp" suffix on the address ("53/udp=4"); the
// worker opens it with net.FilePacketConn instead of net.FileListener. A Unix
// domain socket is listed by its path ("/run/app.sock=5"), or on Linux by
// its abstract name ("@app=5"); the worker opens it with net.FileListener.
const EnvListeners = "DEWY_PORT"

// EnvHealthPort names the environment variable that announces the worker's
// health socket when a health check is configured. The value is a single
// "<address>=<fd>" entry in the EnvListeners format, such as
// "127.0.0.1:41234=5". The socket is a TCP listener on the loopback
// interface that only this worker holds; the worker serves its health
// endpoint on it, and the supervisor probes it before the worker replaces
// the current one.
const EnvHealthPort = "DEWY_HEALTH_PORT"

// EnvListenersCompat carries the same value as EnvListeners under the name
// server-starter uses. The format is compatible with server-starter, so an
// application written for it keeps working unchanged.
const EnvListenersCompat = "SERVER_STARTER_PORT"

// firstListenerFD is the descriptor number of the first inherited socket.
// 0, 1 and 2 are stdin, stdout and stderr; exec.Cmd.ExtraFiles places the
// remaining files from 3 on.
const firstListenerFD = 3

// udpSuffix marks a UDP address ("53/udp", "[::1]:53/udp").
const udpSuffix = "/udp"

// abstractPrefix marks a socket in the Linux abstract namespace ("@app").
const abstractPrefix = "@"

// listener is a socket the supervisor owns for the whole life of the managed
// application: a listening TCP socket or a bound UDP socket. Workers come and
// go; the socket stays open, so a TCP connection arriving during a worker
// swap waits in the kernel's accept queue instead of being refused, and a
// UDP datagram is queued on the socket for whichever worker reads next.
type listener struct {
	addr  string // as configured; used verbatim in EnvListeners
	local net.Addr
	sock  io.Closer // the net.Listener or net.PacketConn
	file  *os.File  // dup of the socket's descriptor, handed to each worker

	// path and info identify the socket file of a Unix domain socket bound
	// to a filesystem path, so close removes the file only while it is the
	// one this listener created.
	path string
	info os.FileInfo
}

func (l *listener) close() {
	_ = l.file.Close()
	_ = l.sock.Close()
	if l.info == nil {
		return
	}
	if fi, err := os.Lstat(l.path); err == nil && os.SameFile(fi, l.info) {
		_ = os.Remove(l.path)
	}
}

// isUnixAddr reports whether addr names a Unix domain socket: an absolute
// path, or a name in the Linux abstract namespace starting with "@".
func isUnixAddr(addr string) bool {
	return strings.HasPrefix(addr, "/") || strings.HasPrefix(addr, abstractPrefix)
}

// checkUnixAddr validates a Unix domain socket address. ";" and "=" are
// rejected because they delimit the entries of EnvListeners.
func checkUnixAddr(addr string) error {
	if strings.ContainsAny(addr, ";=") {
		return fmt.Errorf("invalid unix socket %q: \";\" and \"=\" are not allowed", addr)
	}
	if strings.HasPrefix(addr, abstractPrefix) {
		if runtime.GOOS != "linux" {
			return fmt.Errorf("invalid unix socket %q: abstract sockets are supported only on Linux", addr)
		}
		if addr == abstractPrefix {
			return fmt.Errorf("invalid unix socket %q: abstract name is empty", addr)
		}
		return nil
	}
	if filepath.Clean(addr) != addr || strings.HasSuffix(addr, "/") {
		return fmt.Errorf("invalid unix socket %q: path must be clean and name a file", addr)
	}
	return nil
}

// removeStaleSocket prepares path for binding. A missing path is fine. A
// socket file that refuses connections was left behind by a process that
// exited without removing it, such as a previous dewy that was killed, and
// is removed. Any other file, and a socket that accepts connections, is an
// error: the path belongs to someone else.
func removeStaleSocket(path string) error {
	fi, err := os.Lstat(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if fi.Mode().Type() != fs.ModeSocket {
		return fmt.Errorf("%s exists and is not a socket", path)
	}
	c, err := net.DialTimeout("unix", path, time.Second)
	if err == nil {
		_ = c.Close()
		return fmt.Errorf("%s is in use by another process", path)
	}
	if !errors.Is(err, syscall.ECONNREFUSED) {
		return fmt.Errorf("%s: %w", path, err)
	}
	return os.Remove(path)
}

// resolveAddr maps a configured address to the network and address passed to
// net.Listen, or to net.ListenPacket for a UDP address. The rules below are
// the same for UDP, with "udp", "udp4" and "udp6" in place of the TCP
// networks.
//
//   - "8080" listens on every interface. The "tcp" network is dual stack: Go
//     opens an IPv6 socket that also accepts IPv4 connections, and falls back
//     to IPv4 alone on a host without IPv6.
//   - "127.0.0.1:8080" listens on that IPv4 address only ("tcp4").
//   - "[::1]:8080" or "[fe80::1%eth0]:8080" listens on that IPv6 address only
//     ("tcp6").
//   - "0.0.0.0:8080" listens on every IPv4 interface ("tcp4") and "[::]:8080"
//     on every IPv6 interface ("tcp6", which Go opens with IPV6_V6ONLY).
//   - "/run/app.sock" or, on Linux, "@app" listens on that Unix domain socket
//     ("unix").
//
// The host part must be an IP literal. A hostname may resolve to several
// addresses, or to a different one on each start, and would make the bound
// interface depend on DNS.
func resolveAddr(addr string) (network, address string, err error) {
	if isUnixAddr(addr) {
		if err := checkUnixAddr(addr); err != nil {
			return "", "", err
		}
		return "unix", addr, nil
	}

	proto := "tcp"
	hostport := addr
	if a, ok := strings.CutSuffix(addr, udpSuffix); ok {
		proto, hostport = "udp", a
	}

	if isPort(hostport) {
		return proto, ":" + hostport, nil
	}

	host, port, err := net.SplitHostPort(hostport)
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
		return proto + "4", net.JoinHostPort(ip.String(), port), nil
	}
	return proto + "6", net.JoinHostPort(ip.String(), port), nil
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
		if network == "unix" && !strings.HasPrefix(address, abstractPrefix) {
			if err := removeStaleSocket(address); err != nil {
				return fail(fmt.Errorf("listen %s: %w", addr, err))
			}
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

// filer is implemented by *net.TCPListener, *net.UnixListener and
// *net.UDPConn.
type filer interface {
	File() (*os.File, error)
}

// bind opens the socket for network and address and duplicates its
// descriptor for the workers.
func bind(network, address string) (*listener, error) {
	var (
		sock  io.Closer
		local net.Addr
	)
	if strings.HasPrefix(network, "udp") {
		pc, err := net.ListenPacket(network, address)
		if err != nil {
			return nil, err
		}
		sock, local = pc, pc.LocalAddr()
	} else {
		ln, err := net.Listen(network, address)
		if err != nil {
			return nil, err
		}
		sock, local = ln, ln.Addr()
	}

	l := &listener{local: local, sock: sock}
	if ul, ok := sock.(*net.UnixListener); ok && !strings.HasPrefix(address, abstractPrefix) {
		fi, err := os.Lstat(address)
		if err != nil {
			_ = sock.Close()
			return nil, err
		}
		// Close would remove the path even after another process replaced
		// the file; listener.close removes it only while it is ours.
		ul.SetUnlinkOnClose(false)
		l.path, l.info = address, fi
	}

	fl, ok := sock.(filer)
	if !ok {
		l.close()
		return nil, fmt.Errorf("%T cannot hand over its descriptor", sock)
	}
	f, err := fl.File()
	if err != nil {
		l.close()
		return nil, err
	}
	l.file = f
	return l, nil
}

// openHealthListener binds a TCP socket on a free loopback port, on IPv4 when
// the host has it and on IPv6 otherwise.
func openHealthListener() (*listener, error) {
	l, err := bind("tcp4", "127.0.0.1:0")
	if err != nil {
		var err6 error
		if l, err6 = bind("tcp6", "[::1]:0"); err6 != nil {
			return nil, errors.Join(err, err6)
		}
	}
	l.addr = l.local.String()
	return l, nil
}

// healthEnv builds the EnvHealthPort value for a health socket bound to addr
// and inherited as descriptor fd: a single entry in the EnvListeners format.
func healthEnv(addr net.Addr, fd int) string {
	return addr.String() + "=" + strconv.Itoa(fd)
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
// configured, and so are Unix domain sockets, whose path may end in "/udp"
// without being UDP.
func envAddr(addr string) string {
	if isUnixAddr(addr) {
		return addr
	}
	hostport, udp := strings.CutSuffix(addr, udpSuffix)
	host, port, err := net.SplitHostPort(hostport)
	if err != nil {
		return addr
	}
	if ip, err := netip.ParseAddr(host); err != nil || !ip.IsUnspecified() {
		return addr
	}
	if udp {
		return port + udpSuffix
	}
	return port
}
