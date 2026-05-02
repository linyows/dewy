package dewy

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/carlescere/scheduler"
	"github.com/cli/safeexec"
	"github.com/linyows/dewy/artifact"
	"github.com/linyows/dewy/container"
	"github.com/linyows/dewy/cache"
	"github.com/linyows/dewy/logging"
	"github.com/linyows/dewy/notifier"
	"github.com/linyows/dewy/registry"
	"github.com/linyows/dewy/telemetry"
	starter "github.com/linyows/server-starter"
	"go.opentelemetry.io/otel/attribute"
	otelmetric "go.opentelemetry.io/otel/metric"
)

const (
	ISO8601      = "20060102T150405Z0700"
	releaseDir   = ISO8601
	releasesDir  = "releases"
	symlinkDir   = "current"
	keepReleases = 7 // Keep last 7 releases (for server/assets) or images (for container)

	// currentkeyName is a name whose value is the version of the currently running server application.
	// For example, if you are using a file for the cache store, running `cat current` will show `v1.2.3--app_linux_amd64.tar.gz`, which is a combination of the tag and artifact.
	// dewy uses this value as a key (**cachekeyName**) to manage the artifacts in the cache store.
	currentkeyName = "current"

	// MaxArtifactSize is the maximum allowed artifact download size (512MB).
	MaxArtifactSize int64 = 512 * 1024 * 1024

	// defaultProxyIdleTimeout is the default idle timeout for TCP proxy connections.
	defaultProxyIdleTimeout = 5 * time.Minute
)

// Dewy struct.
type Dewy struct {
	config           Config
	registry         registry.Registry
	artifact         artifact.Artifact
	cache            cache.Cache
	isServerRunning  bool
	disableReport    bool
	root             string
	job              *scheduler.Job
	notifier         notifier.Notifier
	logger           *logging.Logger
	tcpProxies       map[int]*tcpProxy // TCP proxies keyed by proxy port
	proxyMutex       sync.RWMutex
	adminServer      *http.Server // Admin API server for CLI communication
	containerRuntime *container.Runtime
	cVer             string // Current deployed version (tag)
	telemetry        *telemetry.Provider
	sync.RWMutex
}

// tcpProxy manages a TCP proxy for a single port.
type tcpProxy struct {
	proxyPort    int
	listener     net.Listener
	backends     []tcpBackend
	backendIndex uint64 // Atomic counter for round-robin
	mu           sync.RWMutex
	done         chan struct{}
	logger       *logging.Logger
	idleTimeout  time.Duration // 0 means no timeout
	metrics      *telemetry.Metrics
}

// tcpBackend represents a backend server.
type tcpBackend struct {
	host string
	port int
}

// New returns Dewy.
func New(c Config, log *logging.Logger) (*Dewy, error) {
	kv, err := cache.New(context.Background(), c.Cache.URL, log.Logger)
	if err != nil {
		return nil, fmt.Errorf("failed to init cache backend: %w", err)
	}

	wd, err := os.Getwd()
	if err != nil {
		return nil, err
	}

	su := strings.SplitN(c.Registry, "://", 2)
	if len(su) != 2 || su[0] == "" || su[1] == "" {
		return nil, fmt.Errorf("invalid registry format (expected scheme://host/path): %s", c.Registry)
	}
	u, err := url.Parse(su[1])
	if err != nil {
		return nil, err
	}
	if c.CalVer != "" {
		if _, err := registry.NewCalVerFormat(c.CalVer); err != nil {
			return nil, fmt.Errorf("invalid calver format: %w", err)
		}
		q := u.Query()
		q.Set("calver", c.CalVer)
		u.RawQuery = q.Encode()
	}
	c.Registry = fmt.Sprintf("%s://%s", su[0], u.String())

	return &Dewy{
		config:          c,
		cache:           kv,
		isServerRunning: false,
		root:            wd,
		logger:          log,
	}, nil
}

// SetTelemetry sets the telemetry provider.
func (d *Dewy) SetTelemetry(tp *telemetry.Provider) {
	d.telemetry = tp
}

// Start dewy.
func (d *Dewy) Start(i int) {
	d.logger.Info("Dewy started", slog.String("version", d.config.Version),
		slog.String("date", d.config.Date), slog.String("commit", d.config.ShortCommit()))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var err error

	d.registry, err = registry.New(ctx, d.config.Registry, d.logger)
	if err != nil {
		d.logger.Error("Registry failure", slog.String("error", err.Error()))
	}

	// Wrap the registry with a shared result cache when the cache backend
	// supports atomic writes and the operator opted in via ?registry-ttl=...
	// on the cache URL.
	if d.registry != nil {
		if ttl := d.cache.RegistryTTL(); ttl > 0 {
			if ac, ok := d.cache.(cache.AtomicCache); ok {
				d.registry = registry.NewCached(d.registry, d.config.Registry, ac, ttl, d.logger)
				d.logger.Info("Registry result cache enabled",
					slog.Duration("ttl", ttl))
			} else {
				d.logger.Warn("registry-ttl set but cache backend does not support atomic writes; ignoring",
					slog.Duration("ttl", ttl))
			}
		}
	}

	d.notifier, err = notifier.New(ctx, d.config.Notifier, d.logger.Logger)
	if err != nil {
		d.logger.Error("Notifier failure", slog.String("error", err.Error()))
	}

	// Load notifier context from current deployment if present (for restarts)
	d.notifier.OnDeploy(filepath.Join(d.root, symlinkDir))

	msg := fmt.Sprintf("Automatic shipping started by *Dewy* (v%s: %s)", d.config.Version, d.config.Command.String())
	d.logger.Info("Dewy start notification", slog.String("message", msg))
	d.notifier.Send(ctx, msg)

	if d.config.Command == CONTAINER {
		// Start built-in reverse proxy
		if err := d.startProxy(ctx); err != nil {
			d.logger.Error("Proxy startup failed", slog.String("error", err.Error()))
			d.notifier.SendError(ctx, err)
			return
		}

		// Start admin API server
		if err := d.startAdminAPI(ctx); err != nil {
			d.logger.Error("Admin API startup failed", slog.String("error", err.Error()))
			d.notifier.SendError(ctx, err)
			return
		}
	}

	d.job, err = scheduler.Every(i).Seconds().Run(func() {
		var e error
		if d.config.Command == CONTAINER {
			e = d.RunContainer()
		} else {
			e = d.Run()
		}
		if e != nil {
			d.logger.Error("Dewy run failure", slog.String("error", e.Error()))
			d.notifier.SendError(context.Background(), e)
		} else {
			d.notifier.ResetErrorCount()
		}
	})
	if err != nil {
		d.logger.Error("Scheduler failure", slog.String("error", err.Error()))
	}

	d.waitSigs(ctx)
}

func (d *Dewy) waitSigs(ctx context.Context) {
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGHUP, syscall.SIGUSR1, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)

	for sig := range sigCh {
		d.logger.Debug("PID received signal", slog.Int("pid", os.Getpid()), slog.String("signal", sig.String()))
		switch sig {
		case syscall.SIGHUP:
			continue

		case syscall.SIGUSR1:
			if err := d.restartServer(); err != nil {
				d.logger.Error("Restart failure", slog.String("error", err.Error()))
			} else {
				msg := fmt.Sprintf("Restarted receiving by `%s` signal", "SIGUSR1")
				d.logger.Info("Restart notification", slog.String("message", msg))
				d.notifier.Send(ctx, msg)
			}
			continue

		case syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT:
			d.job.Quit <- true

			// Stop managed containers and reverse proxy if running
			if d.config.Command == CONTAINER {
				if err := d.stopManagedContainers(ctx); err != nil {
					d.logger.Error("Failed to stop managed containers", slog.String("error", err.Error()))
				}

				if err := d.stopProxy(ctx); err != nil {
					d.logger.Error("Failed to stop proxy", slog.String("error", err.Error()))
				}

				if err := d.stopAdminAPI(ctx); err != nil {
					d.logger.Error("Failed to stop admin API", slog.String("error", err.Error()))
				}
			}

			// Shutdown telemetry with fresh context to ensure flush completes
			if d.telemetry != nil {
				shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
				if err := d.telemetry.Shutdown(shutdownCtx); err != nil {
					d.logger.Error("Failed to shutdown telemetry", slog.String("error", err.Error()))
				}
				shutdownCancel()
			}

			msg := fmt.Sprintf("Stop receiving by `%s` signal", sig)
			d.logger.Info("Shutdown notification", slog.String("message", msg))
			d.notifier.Send(ctx, msg)
			return
		}
	}
}

// cachekeyName is "tag--artifact"
// example: v1.2.3--testapp_linux_amd64.tar.gz
func (d *Dewy) cachekeyName(res *registry.CurrentResponse) string {
	u := strings.SplitN(res.ArtifactURL, "?", 2)
	return fmt.Sprintf("%s--%s", res.Tag, filepath.Base(u[0]))
}

// Run is the per-tick deploy state machine for SERVER and ASSETS commands.
// It is intentionally short: each phase lives as a method on Dewy in
// dewy_phases.go and can be exercised in isolation.
func (d *Dewy) Run() error {
	ctx, cancel := d.makeRunContext()
	defer cancel()

	res, err := d.resolveCurrent(ctx)
	if err != nil || res == nil {
		return err
	}

	st, err := d.resolveCacheState(ctx, res)
	if err != nil {
		return err
	}
	if st.skip {
		return nil
	}

	if err := d.downloadAndCache(ctx, res, st); err != nil {
		return err
	}
	if err := d.applyDeployment(ctx, res, st.key); err != nil {
		return err
	}
	return d.promoteAndReport(ctx, res)
}

func (d *Dewy) deploy(key string) (err error) {
	ctx := context.Background()

	beforeResult, beforeErr := d.execHook(d.config.BeforeDeployHook)
	if beforeResult != nil {
		d.notifier.SendHookResult(ctx, "Before Deploy", beforeResult)
	}
	if beforeErr != nil {
		d.logger.Error("Before deploy hook failure", slog.String("error", beforeErr.Error()))
		// Continue with deploy even if before hook fails
	}

	defer func() {
		if err != nil {
			return
		}
		// When deploy is success, run after deploy hook
		afterResult, afterErr := d.execHook(d.config.AfterDeployHook)
		if afterResult != nil {
			d.notifier.SendHookResult(ctx, "After Deploy", afterResult)
		}
		if afterErr != nil {
			d.logger.Error("After deploy hook failure", slog.String("error", afterErr.Error()))
		}
	}()
	p := filepath.Join(d.cache.GetDir(), key)
	linkFrom, err := d.preserve(p)
	if err != nil {
		d.logger.Error("Preserve failure", slog.String("error", err.Error()))
		return err
	}
	d.logger.Info("Extract archive", slog.String("path", linkFrom))

	d.notifier.OnDeploy(linkFrom)

	linkTo := filepath.Join(d.root, symlinkDir)

	// Atomic symlink replacement: create temp symlink, then rename
	tmpLink := linkTo + ".tmp"
	os.Remove(tmpLink) // Ensure no stale temp link exists
	if err := os.Symlink(linkFrom, tmpLink); err != nil {
		return err
	}

	d.logger.Info("Create symlink",
		slog.String("from", linkFrom),
		slog.String("to", linkTo))
	if err := os.Rename(tmpLink, linkTo); err != nil {
		os.Remove(tmpLink) // Cleanup on failure
		return err
	}

	return nil
}

func (d *Dewy) preserve(p string) (string, error) {
	dst := filepath.Join(d.root, releasesDir, time.Now().UTC().Format(releaseDir))
	if err := os.MkdirAll(dst, 0755); err != nil {
		return "", err
	}

	if err := cache.ExtractArchive(p, dst); err != nil {
		return "", err
	}

	return dst, nil
}

func (d *Dewy) restartServer() error {
	d.Lock()
	defer d.Unlock()

	pid := os.Getpid()
	p, _ := os.FindProcess(pid)
	err := p.Signal(syscall.SIGHUP)
	if err != nil {
		return err
	}
	d.logger.Info("Send SIGHUP for server restart", slog.String("version", d.cVer), slog.Int("pid", pid))

	return nil
}

func (d *Dewy) startServer() error {
	d.Lock()
	defer d.Unlock()

	d.logger.Info("Start server", slog.String("version", d.cVer))

	// Try to create starter first (synchronous validation)
	s, err := starter.NewStarter(d.config.Starter)
	if err != nil {
		d.logger.Error("Starter failure", slog.String("error", err.Error()))
		return err
	}

	// Start server in background
	go func() {
		err := s.Run()
		if err != nil {
			d.logger.Error("Server run failure", slog.String("error", err.Error()))
			d.Lock()
			d.isServerRunning = false
			d.Unlock()
		}
	}()

	d.isServerRunning = true
	return nil
}

func (d *Dewy) keepReleases() error {
	dir := filepath.Join(d.root, releasesDir)
	files, err := os.ReadDir(dir)
	if err != nil {
		return err
	}

	sort.Slice(files, func(i, j int) bool {
		fi, err := files[i].Info()
		if err != nil {
			return false
		}
		fj, err := files[j].Info()
		if err != nil {
			return true
		}
		return fi.ModTime().Unix() > fj.ModTime().Unix()
	})

	for i, f := range files {
		if i < keepReleases {
			continue
		}
		if err := os.RemoveAll(filepath.Join(dir, f.Name())); err != nil {
			return err
		}
	}

	return nil
}

// cleanupOldImages removes old container images, keeping only the most recent ones.
func (d *Dewy) cleanupOldImages(ctx context.Context, imageRef string) error {
	if d.containerRuntime == nil {
		return fmt.Errorf("container runtime not initialized")
	}

	return d.containerRuntime.CleanupOldImages(ctx, imageRef, keepReleases)
}

func (d *Dewy) execHook(cmd string) (*notifier.HookResult, error) {
	if cmd == "" {
		return nil, nil
	}

	start := time.Now()
	sh, err := safeexec.LookPath("sh")
	if err != nil {
		return nil, err
	}

	stdout := new(bytes.Buffer)
	stderr := new(bytes.Buffer)
	c := exec.Command(sh, "-c", cmd)
	c.Dir = d.root
	c.Env = os.Environ()
	c.Stdout = stdout
	c.Stderr = stderr

	result := &notifier.HookResult{
		Command: cmd,
	}

	if err := c.Run(); err != nil {
		result.Duration = time.Since(start)
		result.Stdout = strings.TrimSpace(stdout.String())
		result.Stderr = strings.TrimSpace(stderr.String())
		result.Success = false

		var exitError *exec.ExitError
		if errors.As(err, &exitError) {
			result.ExitCode = exitError.ExitCode()
		} else {
			result.ExitCode = 1
		}

		d.logger.Info("Execute hook failed",
			slog.String("command", cmd),
			slog.String("stdout", result.Stdout),
			slog.String("stderr", result.Stderr),
			slog.Int("exit_code", result.ExitCode),
			slog.Duration("duration", result.Duration))

		return result, err
	}

	result.Duration = time.Since(start)
	result.Stdout = strings.TrimSpace(stdout.String())
	result.Stderr = strings.TrimSpace(stderr.String())
	result.Success = true
	result.ExitCode = 0

	d.logger.Info("Execute hook",
		slog.String("command", cmd),
		slog.String("stdout", result.Stdout),
		slog.String("stderr", result.Stderr),
		slog.Duration("duration", result.Duration))

	return result, nil
}

// RunContainer runs the container deployment process.
// RunContainer is the per-tick deploy state machine for the CONTAINER command.
// Like Run, each phase lives as a method on Dewy in dewy_phases.go.
func (d *Dewy) RunContainer() error {
	ctx, cancel := d.makeRunContext()
	defer cancel()

	res, err := d.resolveContainerCurrent(ctx)
	if err != nil || res == nil {
		return err
	}

	st, err := d.resolveContainerState(ctx, res)
	if err != nil {
		return err
	}
	if st.skip {
		return nil
	}

	if err := d.pullContainerImage(ctx, res, st); err != nil {
		return err
	}

	deployedCount, err := d.applyContainerDeployment(ctx, res)
	if err != nil {
		return err
	}

	return d.promoteContainerAndReport(ctx, res, deployedCount, st.imageRef)
}

// deployContainer performs the actual container deployment using rolling update strategy.
// Returns the number of successfully deployed containers and any error encountered.
func (d *Dewy) deployContainer(ctx context.Context, res *registry.CurrentResponse) (int, error) {
	if d.config.Container == nil {
		return 0, fmt.Errorf("container config is nil")
	}

	// Create container runtime
	runtime, err := container.New(d.config.Container.Runtime, d.logger.Logger, d.config.Container.DrainTime)
	if err != nil {
		return 0, fmt.Errorf("failed to create container runtime: %w", err)
	}

	// Extract image reference from artifact URL
	// Format: img://registry/repo:tag
	imageRef := strings.TrimPrefix(res.ArtifactURL, "img://")

	// Determine app name from config or image
	appName := d.config.Container.Name
	if appName == "" {
		parts := strings.Split(imageRef, "/")
		if len(parts) > 0 {
			lastPart := parts[len(parts)-1]
			appName = strings.Split(lastPart, ":")[0]
		}
	}

	// Resolve port mappings (auto-detect ContainerPort==0 from image EXPOSE).
	resolvedMappings, err := runtime.ResolvePortMappings(ctx, imageRef, d.config.Container.PortMappings)
	if err != nil {
		return 0, fmt.Errorf("failed to resolve port mappings: %w", err)
	}

	// Create health check function (telemetry-aware, stays in dewy.go)
	healthCheck := d.createHealthCheckFunc(runtime, resolvedMappings)

	// Deploy via container runtime
	report, err := runtime.Deploy(ctx, container.RollingDeployOptions{
		ImageRef:     imageRef,
		AppName:      appName,
		Replicas:     d.config.Container.Replicas,
		PortMappings: resolvedMappings,
		Command:      d.config.Container.Command,
		ExtraArgs:    d.config.Container.ExtraArgs,
		HealthCheck:  healthCheck,
	}, container.BackendCallback{
		OnAdd: func(host string, mappedPort, proxyPort int) error {
			return d.addProxyBackend(host, mappedPort, proxyPort)
		},
		OnRemove: func(host string, mappedPort, proxyPort int) error {
			return d.removeProxyBackend(host, mappedPort, proxyPort)
		},
	})
	if err != nil {
		return 0, err
	}

	// Record telemetry: net change = new containers - removed containers
	if d.telemetry != nil && d.telemetry.Enabled() {
		delta := int64(len(report.Results)) - int64(report.RemovedCount)
		d.telemetry.Metrics().ContainerReplicas.Add(ctx, delta)
	}

	return len(report.Results), nil
}

// createHealthCheckFunc creates a health check function based on configuration.
// Health check is performed on the first port mapping.
func (d *Dewy) createHealthCheckFunc(rt *container.Runtime, resolvedMappings []container.PortMapping) container.HealthCheckFunc {
	if d.config.Container.HealthPath == "" {
		d.logger.Info("Health check disabled - container will start without health verification")
		return nil
	}

	if len(resolvedMappings) == 0 {
		d.logger.Warn("No port mappings configured, health check disabled")
		return nil
	}

	// Use first port mapping for health check
	firstMapping := resolvedMappings[0]

	return func(ctx context.Context, containerID string) error {
		mappedPort, err := rt.GetMappedPort(ctx, containerID, firstMapping.ContainerPort)
		if err != nil {
			return fmt.Errorf("failed to get mapped port for health check: %w", err)
		}

		healthURL := fmt.Sprintf("http://localhost:%d%s", mappedPort, d.config.Container.HealthPath)
		client := &http.Client{Timeout: 5 * time.Second}

		retries := 5
		for i := range retries {
			if d.telemetry != nil && d.telemetry.Enabled() {
				d.telemetry.Metrics().HealthChecksTotal.Add(ctx, 1)
			}
			resp, err := client.Get(healthURL)
			if err == nil {
				resp.Body.Close()
				if resp.StatusCode >= 200 && resp.StatusCode < 400 {
					d.logger.Debug("Health check passed",
						slog.String("url", healthURL),
						slog.Int("status", resp.StatusCode))
					return nil
				}
			}
			if d.telemetry != nil && d.telemetry.Enabled() {
				d.telemetry.Metrics().HealthCheckFailures.Add(ctx, 1)
			}
			if i < retries-1 {
				time.Sleep(2 * time.Second)
			}
		}
		return fmt.Errorf("health check failed after %d retries", retries)
	}
}

// startProxy starts TCP proxies for all configured port mappings.
func (d *Dewy) startProxy(ctx context.Context) error {
	if len(d.config.Container.PortMappings) == 0 {
		return fmt.Errorf("no port mappings configured for proxy")
	}

	d.proxyMutex.Lock()
	d.tcpProxies = make(map[int]*tcpProxy)
	d.proxyMutex.Unlock()

	// Start a TCP proxy for each port mapping
	var metrics *telemetry.Metrics
	if d.telemetry != nil && d.telemetry.Enabled() {
		metrics = d.telemetry.Metrics()
	}
	for _, mapping := range d.config.Container.PortMappings {
		proxy, err := newTCPProxy(mapping.ProxyPort, d.logger, d.config.Container.ProxyIdleTimeout, metrics)
		if err != nil {
			// Clean up already started proxies
			if stopErr := d.stopProxy(ctx); stopErr != nil {
				d.logger.Error("Failed to stop proxies during cleanup", slog.String("error", stopErr.Error()))
			}
			return fmt.Errorf("failed to start proxy on port %d: %w", mapping.ProxyPort, err)
		}

		d.proxyMutex.Lock()
		d.tcpProxies[mapping.ProxyPort] = proxy
		d.proxyMutex.Unlock()

		d.logger.Info("TCP proxy started",
			slog.Int("proxy_port", mapping.ProxyPort))
	}

	d.logger.Info("All TCP proxies started successfully",
		slog.Int("count", len(d.config.Container.PortMappings)))

	return nil
}

// newTCPProxy creates and starts a new TCP proxy on the specified port.
func newTCPProxy(port int, logger *logging.Logger, idleTimeout time.Duration, metrics *telemetry.Metrics) (*tcpProxy, error) {
	addr := fmt.Sprintf(":%d", port)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("failed to listen on %s: %w", addr, err)
	}

	proxy := &tcpProxy{
		proxyPort:   port,
		listener:    listener,
		backends:    make([]tcpBackend, 0),
		done:        make(chan struct{}),
		logger:      logger,
		idleTimeout: idleTimeout,
		metrics:     metrics,
	}

	go proxy.acceptLoop()

	return proxy, nil
}

// acceptLoop accepts incoming connections and proxies them to backends.
func (p *tcpProxy) acceptLoop() {
	for {
		select {
		case <-p.done:
			return
		default:
		}

		conn, err := p.listener.Accept()
		if err != nil {
			select {
			case <-p.done:
				return
			default:
				p.logger.Debug("Accept error",
					slog.Int("proxy_port", p.proxyPort),
					slog.String("error", err.Error()))
				continue
			}
		}

		go p.handleConnection(conn)
	}
}

// handleConnection proxies a single connection to a backend.
func (p *tcpProxy) handleConnection(clientConn net.Conn) {
	defer clientConn.Close()

	ctx := context.Background()
	portAttr := otelmetric.WithAttributes(attribute.Int("proxy_port", p.proxyPort))

	// Record connection accepted metrics immediately
	if p.metrics != nil {
		p.metrics.ProxyConnectionsTotal.Add(ctx, 1, portAttr)
		p.metrics.ProxyActiveConnections.Add(ctx, 1, portAttr)
	}
	connStart := time.Now()
	defer func() {
		if p.metrics != nil {
			p.metrics.ProxyActiveConnections.Add(ctx, -1, portAttr)
			p.metrics.ProxyConnectionDuration.Record(ctx, time.Since(connStart).Seconds(), portAttr)
		}
	}()

	// Get backend using round-robin
	backend, ok := p.getNextBackend()
	if !ok {
		p.logger.Debug("No backend available",
			slog.Int("proxy_port", p.proxyPort))
		if p.metrics != nil {
			p.metrics.ProxyErrorsTotal.Add(ctx, 1, portAttr)
		}
		return
	}

	// Connect to backend with latency measurement
	backendAddr := net.JoinHostPort(backend.host, strconv.Itoa(backend.port))
	dialStart := time.Now()
	backendConn, err := net.DialTimeout("tcp", backendAddr, 5*time.Second)
	if p.metrics != nil {
		p.metrics.ProxyConnectLatency.Record(ctx, time.Since(dialStart).Seconds(), portAttr)
	}
	if err != nil {
		p.logger.Error("Failed to connect to backend",
			slog.Int("proxy_port", p.proxyPort),
			slog.String("backend", backendAddr),
			slog.String("error", err.Error()))
		if p.metrics != nil {
			p.metrics.ProxyErrorsTotal.Add(ctx, 1, portAttr)
		}
		return
	}
	defer backendConn.Close()

	p.logger.Debug("Proxying connection",
		slog.Int("proxy_port", p.proxyPort),
		slog.String("backend", backendAddr),
		slog.String("client", clientConn.RemoteAddr().String()))

	// Wrap connections with idle timeout (skip if timeout is 0)
	var src io.Reader = clientConn
	var dst io.Writer = backendConn
	var srcBack io.Reader = backendConn
	var dstBack io.Writer = clientConn
	if p.idleTimeout > 0 {
		tcClient := &timeoutConn{Conn: clientConn, idleTimeout: p.idleTimeout}
		tcBackend := &timeoutConn{Conn: backendConn, idleTimeout: p.idleTimeout}
		src = tcClient
		dst = tcBackend
		srcBack = tcBackend
		dstBack = tcClient
	}

	// Bidirectional copy
	done := make(chan struct{}, 2)

	go func() {
		n, _ := io.Copy(dst, src)
		if p.metrics != nil && n > 0 {
			p.metrics.ProxyBytesTransferred.Add(ctx, n, portAttr)
		}
		done <- struct{}{}
	}()

	go func() {
		n, _ := io.Copy(dstBack, srcBack)
		if p.metrics != nil && n > 0 {
			p.metrics.ProxyBytesTransferred.Add(ctx, n, portAttr)
		}
		done <- struct{}{}
	}()

	// Wait for either direction to complete
	<-done
}

// getNextBackend returns the next backend using round-robin.
func (p *tcpProxy) getNextBackend() (tcpBackend, bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if len(p.backends) == 0 {
		return tcpBackend{}, false
	}

	index := atomic.AddUint64(&p.backendIndex, 1) - 1
	return p.backends[index%uint64(len(p.backends))], true
}

// addBackend adds a backend to this proxy.
func (p *tcpProxy) addBackend(host string, port int) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.backends = append(p.backends, tcpBackend{host: host, port: port})
	p.logger.Info("Backend added to TCP proxy",
		slog.Int("proxy_port", p.proxyPort),
		slog.String("backend_host", host),
		slog.Int("backend_port", port),
		slog.Int("total_backends", len(p.backends)))

	if p.metrics != nil {
		p.metrics.ProxyBackendCount.Add(context.Background(), 1,
			otelmetric.WithAttributes(attribute.Int("proxy_port", p.proxyPort)))
	}
}

// removeBackend removes a backend from this proxy.
func (p *tcpProxy) removeBackend(host string, port int) bool {
	p.mu.Lock()
	defer p.mu.Unlock()

	for i, b := range p.backends {
		if b.host == host && b.port == port {
			p.backends = append(p.backends[:i], p.backends[i+1:]...)
			p.logger.Info("Backend removed from TCP proxy",
				slog.Int("proxy_port", p.proxyPort),
				slog.String("backend_host", host),
				slog.Int("backend_port", port),
				slog.Int("remaining_backends", len(p.backends)))

			if p.metrics != nil {
				p.metrics.ProxyBackendCount.Add(context.Background(), -1,
					otelmetric.WithAttributes(attribute.Int("proxy_port", p.proxyPort)))
			}
			return true
		}
	}
	return false
}

// stop stops the TCP proxy.
func (p *tcpProxy) stop() error {
	close(p.done)
	return p.listener.Close()
}

// backendCount returns the number of backends.
func (p *tcpProxy) backendCount() int {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return len(p.backends)
}

// addProxyBackend adds a new backend to the appropriate TCP proxy.
func (d *Dewy) addProxyBackend(host string, port int, proxyPort int) error {
	d.proxyMutex.RLock()
	proxy, exists := d.tcpProxies[proxyPort]
	d.proxyMutex.RUnlock()

	if !exists {
		return fmt.Errorf("no proxy configured for port %d", proxyPort)
	}

	proxy.addBackend(host, port)
	return nil
}

// removeProxyBackend removes a backend from the appropriate TCP proxy.
func (d *Dewy) removeProxyBackend(host string, port int, proxyPort int) error {
	d.proxyMutex.RLock()
	proxy, exists := d.tcpProxies[proxyPort]
	d.proxyMutex.RUnlock()

	if !exists {
		return fmt.Errorf("no proxy configured for port %d", proxyPort)
	}

	if !proxy.removeBackend(host, port) {
		return fmt.Errorf("backend %s:%d not found for proxy port %d", host, port, proxyPort)
	}
	return nil
}

// stopProxy gracefully shuts down all TCP proxies.
func (d *Dewy) stopProxy(ctx context.Context) error {
	d.proxyMutex.Lock()
	defer d.proxyMutex.Unlock()

	if d.tcpProxies == nil {
		return nil
	}

	d.logger.Info("Stopping TCP proxies", slog.Int("count", len(d.tcpProxies)))

	var errs []error
	for port, proxy := range d.tcpProxies {
		if err := proxy.stop(); err != nil {
			errs = append(errs, fmt.Errorf("failed to stop proxy on port %d: %w", port, err))
		}
	}

	d.tcpProxies = nil

	if len(errs) > 0 {
		return errors.Join(errs...)
	}

	d.logger.Info("All TCP proxies stopped")
	return nil
}

// totalProxyBackends returns the total number of backends across all TCP proxies.
func (d *Dewy) totalProxyBackends() int {
	d.proxyMutex.RLock()
	defer d.proxyMutex.RUnlock()

	total := 0
	for _, proxy := range d.tcpProxies {
		total += proxy.backendCount()
	}
	return total
}

// stopManagedContainers stops all containers managed by this dewy instance.
func (d *Dewy) stopManagedContainers(ctx context.Context) error {
	if d.containerRuntime == nil {
		return nil
	}

	d.logger.Info("Stopping managed containers")

	// Determine app name from config or registry
	appName := d.config.Container.Name
	if appName == "" {
		registryURL := d.config.Registry
		parts := strings.SplitN(registryURL, "://", 2)
		if len(parts) == 2 {
			pathParts := strings.Split(parts[1], "/")
			if len(pathParts) > 0 {
				lastPart := pathParts[len(pathParts)-1]
				appName = strings.Split(lastPart, "?")[0]
				appName = strings.Split(appName, ":")[0]
			}
		}
	}

	_, _, err := d.containerRuntime.StopManagedContainers(ctx, appName)
	return err
}

// startAdminAPI starts the admin API server on TCP localhost.
func (d *Dewy) startAdminAPI(ctx context.Context) error {
	// Default admin port is 17539 (DEWY: D=4, E=5, W=23, Y=25 -> 4+5+2+3+2+5=21, but 17539 is more unique)
	adminPort := d.config.AdminPort
	if adminPort == 0 {
		adminPort = 17539
	}

	// Try to bind to the port, increment if already in use
	var listener net.Listener
	var err error
	maxAttempts := 10

	for i := range maxAttempts {
		currentPort := adminPort + i
		addr := fmt.Sprintf("localhost:%d", currentPort)
		listener, err = net.Listen("tcp", addr)
		if err == nil {
			// Successfully bound to port
			adminPort = currentPort
			d.logger.Info("Admin API port bound successfully",
				slog.Int("port", adminPort))
			break
		}
		d.logger.Debug("Admin API port in use, trying next",
			slog.Int("port", currentPort),
			slog.String("error", err.Error()))
	}

	if listener == nil {
		return fmt.Errorf("failed to bind admin API after %d attempts: %w", maxAttempts, err)
	}

	// Create HTTP mux for admin API
	mux := http.NewServeMux()
	mux.HandleFunc("/api/containers", d.handleGetContainers)
	mux.HandleFunc("/api/status", d.handleGetStatus)

	// Add Prometheus metrics endpoint if telemetry is enabled
	if d.telemetry != nil && d.telemetry.Enabled() {
		mux.Handle("/metrics", d.telemetry.PrometheusHandler())
		d.logger.Info("Prometheus metrics endpoint enabled", slog.String("path", "/metrics"))
	}

	d.adminServer = &http.Server{
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	// Start server in background
	go func() {
		d.logger.Info("Starting admin API server",
			slog.Int("port", adminPort))

		if err := d.adminServer.Serve(listener); err != nil && err != http.ErrServerClosed {
			d.logger.Error("Admin API server error", slog.String("error", err.Error()))
		}
	}()

	d.logger.Info("Admin API server started",
		slog.Int("port", adminPort),
		slog.String("address", fmt.Sprintf("http://localhost:%d", adminPort)))

	return nil
}

// stopAdminAPI stops the admin API server.
func (d *Dewy) stopAdminAPI(ctx context.Context) error {
	if d.adminServer == nil {
		return nil
	}

	d.logger.Info("Stopping admin API server")

	shutdownCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	if err := d.adminServer.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("failed to shutdown admin API: %w", err)
	}

	d.logger.Info("Admin API server stopped")
	return nil
}

// handleGetContainers handles GET /api/containers endpoint.
func (d *Dewy) handleGetContainers(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	ctx := r.Context()

	// Get containers managed by dewy
	labels := map[string]string{
		"dewy.managed": "true",
		"dewy.app":     d.config.Container.Name,
	}

	var containers []*container.Info
	var err error

	if d.config.Command == CONTAINER {
		// Use first port mapping for listing containers (0 = auto-detect / not specified)
		containerPort := 0
		if len(d.config.Container.PortMappings) > 0 {
			containerPort = d.config.Container.PortMappings[0].ContainerPort
		}
		containers, err = d.containerRuntime.ListContainersByLabels(ctx, labels, containerPort)
		if err != nil {
			d.logger.Error("Failed to list containers",
				slog.String("error", err.Error()))
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}
	}

	// Return JSON response
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]any{
		"containers": containers,
	}); err != nil {
		d.logger.Error("Failed to encode response",
			slog.String("error", err.Error()))
	}
}

// handleGetStatus handles GET /api/status endpoint.
func (d *Dewy) handleGetStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	d.RLock()
	defer d.RUnlock()

	// Count total backends across all proxies
	totalBackends := d.totalProxyBackends()

	// Return JSON response
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]any{
		"name":            d.config.Container.Name,
		"command":         d.config.Command,
		"current_version": d.cVer,
		"proxy_backends":  totalBackends,
		"is_running":      d.isServerRunning,
	}); err != nil {
		d.logger.Error("Failed to encode response",
			slog.String("error", err.Error()))
	}
}

// resolvePortMappings resolves port mappings by auto-detecting container ports if needed.
// Returns fully resolved port mappings with both proxy and container ports specified.

// limitedWriter wraps an io.Writer and limits the total bytes written.
// Returns an error when the limit is exceeded.
type limitedWriter struct {
	W       io.Writer
	N       int64
	written int64
}

func (lw *limitedWriter) Write(p []byte) (int, error) {
	if lw.written+int64(len(p)) > lw.N {
		return 0, fmt.Errorf("write limit exceeded: maximum %d bytes", lw.N)
	}
	n, err := lw.W.Write(p)
	lw.written += int64(n)
	return n, err
}

// timeoutConn wraps net.Conn and resets the deadline on every read/write.
type timeoutConn struct {
	net.Conn
	idleTimeout time.Duration
}

func (c *timeoutConn) Read(b []byte) (int, error) {
	if err := c.SetDeadline(time.Now().Add(c.idleTimeout)); err != nil {
		return 0, err
	}
	return c.Conn.Read(b)
}

func (c *timeoutConn) Write(b []byte) (int, error) {
	if err := c.SetDeadline(time.Now().Add(c.idleTimeout)); err != nil {
		return 0, err
	}
	return c.Conn.Write(b)
}
