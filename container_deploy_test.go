package dewy

import (
	"context"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/linyows/dewy/container"
	"github.com/linyows/dewy/logging"
)

func testDewy(c *ContainerConfig) *Dewy {
	return &Dewy{
		config: Config{Container: c},
		logger: logging.SetupLogger("error", "text", io.Discard),
	}
}

func TestHealthCheckTimeout(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		config *ContainerConfig
		want   time.Duration
	}{
		{
			name:   "honors configured health timeout",
			config: &ContainerConfig{HealthTimeout: 60 * time.Second},
			want:   60 * time.Second,
		},
		{
			name:   "falls back when unset",
			config: &ContainerConfig{},
			want:   defaultHealthCheckTotalTimeout,
		},
		{
			name:   "falls back when negative",
			config: &ContainerConfig{HealthTimeout: -1 * time.Second},
			want:   defaultHealthCheckTotalTimeout,
		},
		{
			name:   "falls back when container config is nil",
			config: nil,
			want:   defaultHealthCheckTotalTimeout,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := testDewy(tt.config).healthCheckTimeout(); got != tt.want {
				t.Errorf("healthCheckTimeout() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestProbeHealth_Success(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	d := testDewy(&ContainerConfig{})
	if err := d.probeHealth(context.Background(), server.URL, 5*time.Second); err != nil {
		t.Errorf("probeHealth() error = %v, want nil", err)
	}
}

func TestProbeHealth_RedirectStatusIsHealthy(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotModified)
	}))
	defer server.Close()

	d := testDewy(&ContainerConfig{})
	if err := d.probeHealth(context.Background(), server.URL, 5*time.Second); err != nil {
		t.Errorf("probeHealth() error = %v, want nil", err)
	}
}

func TestProbeHealth_DoesNotFollowRedirect(t *testing.T) {
	t.Parallel()

	// The redirect target is unhealthy. Following it would fail the probe even
	// though the health endpoint itself answered with a status the rule calls
	// healthy, so the 3xx has to be judged as itself.
	var targetHits atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/elsewhere" {
			targetHits.Add(1)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		http.Redirect(w, r, "/elsewhere", http.StatusFound)
	}))
	defer server.Close()

	d := testDewy(&ContainerConfig{})
	if err := d.probeHealth(context.Background(), server.URL+"/health", 5*time.Second); err != nil {
		t.Errorf("probeHealth() error = %v, want nil", err)
	}
	if got := targetHits.Load(); got != 0 {
		t.Errorf("redirect target was requested %d times, want 0", got)
	}
}

// probeProxyChildEnv carries the health URL to the child process started by
// TestProbeHealth_IgnoresProxy. Its presence makes the test act as the child.
const probeProxyChildEnv = "DEWY_TEST_PROBE_PROXY_URL"

func TestProbeHealth_IgnoresProxy(t *testing.T) {
	// Go reads the proxy environment variables once per process, so setting
	// HTTP_PROXY here could be ignored if an earlier test already caused them
	// to be read. The probe runs in a child process that starts with
	// HTTP_PROXY set instead.
	if u := os.Getenv(probeProxyChildEnv); u != "" {
		d := testDewy(&ContainerConfig{})
		if err := d.probeHealth(context.Background(), u, 3*time.Second); err != nil {
			t.Fatalf("probeHealth() error = %v, want nil", err)
		}
		return
	}

	// Go never proxies a loopback target, so the server has to listen on
	// another interface for the proxy to be used at all, as with --host.
	var appHits atomic.Int32
	app := startNonLoopbackServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/health" {
			appHits.Add(1)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer app.Close()

	// The proxy answers 403, so a probe that goes through it fails.
	var proxyHits atomic.Int32
	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		proxyHits.Add(1)
		w.WriteHeader(http.StatusForbidden)
	}))
	defer proxy.Close()

	cmd := exec.Command(os.Args[0], "-test.run=^TestProbeHealth_IgnoresProxy$", "-test.count=1") //nolint:gosec // G204: re-executes this test binary with fixed arguments
	cmd.Env = append(os.Environ(),
		probeProxyChildEnv+"="+app.URL+"/health",
		"HTTP_PROXY="+proxy.URL,
		"http_proxy="+proxy.URL,
		"NO_PROXY=",
		"no_proxy=",
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Errorf("probe in child process failed: %v\n%s", err, out)
	}
	if got := proxyHits.Load(); got != 0 {
		t.Errorf("proxy received %d requests, want 0", got)
	}
	if got := appHits.Load(); got == 0 {
		t.Error("server received no requests, want at least 1")
	}
}

// startNonLoopbackServer starts handler on an IPv4 address of an interface
// other than the loopback, or skips the test when there is none. A VPN
// tunnel or a host firewall can keep the host from reaching its own address,
// so an address is used only after a request to it gets a response.
func startNonLoopbackServer(t *testing.T, handler http.Handler) *httptest.Server {
	t.Helper()
	ifaces, err := net.Interfaces()
	if err != nil {
		t.Skipf("cannot list interfaces: %v", err)
	}
	check := &http.Client{Timeout: time.Second, Transport: &http.Transport{}}
	defer check.CloseIdleConnections()
	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, addr := range addrs {
			ipnet, ok := addr.(*net.IPNet)
			if !ok {
				continue
			}
			ip := ipnet.IP.To4()
			if ip == nil || ip.IsLoopback() || ip.IsLinkLocalUnicast() {
				continue
			}
			l, err := net.Listen("tcp4", net.JoinHostPort(ip.String(), "0"))
			if err != nil {
				continue
			}
			srv := httptest.NewUnstartedServer(handler)
			srv.Listener = l
			srv.Start()
			resp, err := check.Get(srv.URL + "/reachability")
			if err != nil {
				srv.Close()
				continue
			}
			_ = resp.Body.Close()
			return srv
		}
	}
	t.Skip("no non-loopback IPv4 address reachable from this host")
	return nil
}

func TestProbeHealth_RetriesUntilHealthy(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if calls.Add(1) < 3 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	d := testDewy(&ContainerConfig{})
	if err := d.probeHealth(context.Background(), server.URL, 30*time.Second); err != nil {
		t.Errorf("probeHealth() error = %v, want nil", err)
	}
	if got := calls.Load(); got != 3 {
		t.Errorf("probe attempts = %d, want 3", got)
	}
}

func TestProbeHealth_BudgetExhausted(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()

	// A budget shorter than the back-off leaves room for one attempt only.
	d := testDewy(&ContainerConfig{})
	err := d.probeHealth(context.Background(), server.URL, 500*time.Millisecond)
	if err == nil {
		t.Fatal("probeHealth() error = nil, want failure")
	}
	if !strings.Contains(err.Error(), "health check failed after 1 attempts") {
		t.Errorf("probeHealth() error = %q, want it to report 1 attempt", err)
	}
	if got := calls.Load(); got != 1 {
		t.Errorf("probe attempts = %d, want 1", got)
	}
}

// A larger budget must buy more attempts, which is the whole point of
// --health-timeout being wired through.
func TestProbeHealth_LargerBudgetBuysMoreAttempts(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()

	d := testDewy(&ContainerConfig{})
	// defaultHealthCheckDelay is 2s, so a 5s budget allows attempts at
	// t=0, t=2 and t=4.
	if err := d.probeHealth(context.Background(), server.URL, 5*time.Second); err == nil {
		t.Fatal("probeHealth() error = nil, want failure")
	}
	if got := calls.Load(); got != 3 {
		t.Errorf("probe attempts = %d, want 3", got)
	}
}

func TestProbeHealth_ContextCancellation(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	d := testDewy(&ContainerConfig{})
	start := time.Now()
	if err := d.probeHealth(ctx, server.URL, 30*time.Second); err == nil {
		t.Fatal("probeHealth() error = nil, want failure")
	}
	// Canceling the parent must abort immediately rather than burning the
	// full budget in the back-off sleep.
	if elapsed := time.Since(start); elapsed > 5*time.Second {
		t.Errorf("probeHealth() took %v after cancellation, want it to return promptly", elapsed)
	}
}

func TestCreateHealthCheckFunc_DisabledWithoutHealthPath(t *testing.T) {
	t.Parallel()

	d := testDewy(&ContainerConfig{})
	if fn := d.createHealthCheckFunc(nil, []container.PortMapping{{ContainerPort: 8000, ProxyPort: 8080}}); fn != nil {
		t.Error("createHealthCheckFunc() != nil, want nil when health path is empty")
	}
}

func TestCreateHealthCheckFunc_DisabledWithoutPortMappings(t *testing.T) {
	t.Parallel()

	d := testDewy(&ContainerConfig{HealthPath: "/health"})
	if fn := d.createHealthCheckFunc(nil, nil); fn != nil {
		t.Error("createHealthCheckFunc() != nil, want nil when no port mappings are resolved")
	}
}
