package dewy

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/carlescere/scheduler"
	"github.com/linyows/dewy/artifact"
	"github.com/linyows/dewy/cache"
	"github.com/linyows/dewy/container"
	"github.com/linyows/dewy/logging"
	"github.com/linyows/dewy/notifier"
	"github.com/linyows/dewy/registry"
	"github.com/linyows/dewy/telemetry"
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

// New returns Dewy.
func New(c Config, log *logging.Logger) (*Dewy, error) {
	kv, err := cache.New(context.Background(), c.Cache.URL, log.Slog())
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

	d.notifier, err = notifier.New(ctx, d.config.Notifier, d.logger.Slog())
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
// lifecycle.go and can be exercised in isolation.
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

// RunContainer runs the container deployment process.
// RunContainer is the per-tick deploy state machine for the CONTAINER command.
// Like Run, each phase lives as a method on Dewy in lifecycle.go.
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
