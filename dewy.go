package dewy

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/carlescere/scheduler"
	"github.com/cli/safeexec"
	"github.com/linyows/dewy/artifact"
	"github.com/linyows/dewy/container"
	"github.com/linyows/dewy/kvs"
	"github.com/linyows/dewy/logging"
	"github.com/linyows/dewy/notifier"
	"github.com/linyows/dewy/registry"
	starter "github.com/linyows/server-starter"
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
)

// Dewy struct.
type Dewy struct {
	config           Config
	registry         registry.Registry
	artifact         artifact.Artifact
	cache            kvs.KVS
	isServerRunning  bool
	disableReport    bool
	root             string
	job              *scheduler.Job
	notifier         notifier.Notifier
	logger           *logging.Logger
	proxyServer      *http.Server
	proxyBackend     *url.URL
	proxyMutex       sync.RWMutex
	containerRuntime container.Runtime
	sync.RWMutex
}

// New returns Dewy.
func New(c Config, log *logging.Logger) (*Dewy, error) {
	kv := &kvs.File{}
	kv.Default()
	kv.SetLogger(log.Logger)

	wd, err := os.Getwd()
	if err != nil {
		return nil, err
	}

	su := strings.SplitN(c.Registry, "://", 2)
	u, err := url.Parse(su[1])
	if err != nil {
		return nil, err
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

	d.notifier, err = notifier.New(ctx, d.config.Notifier, d.logger.Logger)
	if err != nil {
		d.logger.Error("Notifier failure", slog.String("error", err.Error()))
	}

	msg := fmt.Sprintf("Automatic shipping started by *Dewy* (v%s: %s)", d.config.Version, d.config.Command.String())
	d.logger.Info("Dewy start notification", slog.String("message", msg))
	d.notifier.Send(ctx, msg)

	if d.config.Command == CONTAINER {
		runtime := d.config.Container.Runtime
		msg := "Container logs are not displayed in dewy output, To view application logs."
		d.logger.Info(fmt.Sprintf("%s Use: %s logs -f $(%s ps -q --filter \"label=dewy.managed=true\")", msg, runtime, runtime))

		// Start built-in reverse proxy
		if err := d.startProxy(ctx); err != nil {
			d.logger.Error("Proxy startup failed", slog.String("error", err.Error()))
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

// Run dewy.
func (d *Dewy) Run() error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Get current
	res, err := d.registry.Current(ctx)
	if err != nil {
		// Check if this is an artifact not found error within 30 minute grace period.
		// This prevents false alerts when GitHub Actions or other CI systems are still
		// building and uploading artifacts after release creation.
		var artifactNotFoundErr *registry.ArtifactNotFoundError
		if errors.As(err, &artifactNotFoundErr) {
			gracePeriod := 30 * time.Minute
			if artifactNotFoundErr.IsWithinGracePeriod(gracePeriod) {
				d.logger.Debug("Artifact not found within grace period",
					slog.String("message", artifactNotFoundErr.Message),
					slog.Duration("grace_period", 30*time.Minute))
				return nil // Return nil to avoid error notification
			}
		}
		d.logger.Error("Current failure", slog.String("error", err.Error()))
		return err
	}

	// Check cache
	cachekeyName := d.cachekeyName(res)
	currentkeyValue, _ := d.cache.Read(currentkeyName)
	found := false
	list, err := d.cache.List()
	if err != nil {
		return err
	}

	for _, key := range list {
		// same current version and already cached
		if string(currentkeyValue) == cachekeyName && key == cachekeyName {
			d.logger.Debug("Deploy skipped")
			if d.config.Command == SERVER {
				if d.isServerRunning {
					return nil
				}
				// when the server fails to start (SERVER mode only)
				break
			} else if d.config.Command == ASSETS {
				return nil // always skip for assets command
			}
		}

		// no current version but already cached
		if key == cachekeyName {
			found = true
			if err := d.cache.Write(currentkeyName, []byte(cachekeyName)); err != nil {
				return err
			}
			break
		}
	}

	// Download artifact and cache
	if !found {
		buf := new(bytes.Buffer)

		if d.artifact == nil {
			d.artifact, err = artifact.New(ctx, res.ArtifactURL, d.logger.Logger)
			if err != nil {
				return fmt.Errorf("failed artifact.New: %w", err)
			}
		}
		err := d.artifact.Download(ctx, buf)
		d.artifact = nil
		if err != nil {
			return fmt.Errorf("failed artifact.Download: %w", err)
		}

		if err := d.cache.Write(cachekeyName, buf.Bytes()); err != nil {
			return fmt.Errorf("failed cache.Write cachekeyName: %w", err)
		}
		if err := d.cache.Write(currentkeyName, []byte(cachekeyName)); err != nil {
			return fmt.Errorf("failed cache.Write currentkeyName: %w", err)
		}
		d.logger.Info("Cached artifact", slog.String("cache_key", cachekeyName))
	}

	msg := fmt.Sprintf("Downloaded artifact for `%s`", res.Tag)
	d.logger.Info("Download notification", slog.String("message", msg))
	d.notifier.Send(ctx, msg)

	if err := d.deploy(cachekeyName); err != nil {
		return err
	}

	if d.config.Command == SERVER {
		if d.isServerRunning {
			err = d.restartServer()
			if err == nil {
				msg := fmt.Sprintf("Server restarted for `%s`", res.Tag)
				d.logger.Info("Restart notification", slog.String("message", msg))
				d.notifier.Send(ctx, msg)
			}
		} else {
			err = d.startServer()
			if err == nil {
				msg := fmt.Sprintf("Server started for `%s`", res.Tag)
				d.logger.Info("Start notification", slog.String("message", msg))
				d.notifier.Send(ctx, msg)
			}
		}
		if err != nil {
			d.logger.Error("Server failure", slog.String("error", err.Error()))
			return err
		}
	}

	if !d.disableReport {
		d.logger.Debug("Report shipping")
		err := d.registry.Report(ctx, &registry.ReportRequest{
			ID:      res.ID,
			Tag:     res.Tag,
			Command: d.config.Command.String(),
		})
		if err != nil {
			d.logger.Error("Report shipping failure", slog.String("error", err.Error()))
		}
	}

	d.logger.Info("Keep releases", slog.Int("count", keepReleases))
	err = d.keepReleases()
	if err != nil {
		d.logger.Error("Keep releases failure", slog.String("error", err.Error()))
	}

	return nil
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

	linkTo := filepath.Join(d.root, symlinkDir)
	if _, err := os.Lstat(linkTo); err == nil {
		os.Remove(linkTo)
	}

	d.logger.Info("Create symlink",
		slog.String("from", linkFrom),
		slog.String("to", linkTo))
	if err := os.Symlink(linkFrom, linkTo); err != nil {
		return err
	}

	return nil
}

func (d *Dewy) preserve(p string) (string, error) {
	dst := filepath.Join(d.root, releasesDir, time.Now().UTC().Format(releaseDir))
	if err := os.MkdirAll(dst, 0755); err != nil {
		return "", err
	}

	if err := kvs.ExtractArchive(p, dst); err != nil {
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
	d.logger.Info("Send SIGHUP for server restart", slog.Int("pid", pid))

	return nil
}

func (d *Dewy) startServer() error {
	d.Lock()
	defer d.Unlock()

	d.logger.Info("Start server")

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

	// Extract repository from imageRef (remove tag if present)
	// Example: "ghcr.io/linyows/myapp:v1.0.0" -> "ghcr.io/linyows/myapp"
	repository := imageRef
	if idx := strings.LastIndex(imageRef, ":"); idx != -1 {
		repository = imageRef[:idx]
	}

	// List all images for this repository
	images, err := d.containerRuntime.ListImages(ctx, repository)
	if err != nil {
		return fmt.Errorf("failed to list images: %w", err)
	}

	if len(images) <= keepReleases {
		d.logger.Debug("No old images to clean up",
			slog.String("repository", repository),
			slog.Int("count", len(images)),
			slog.Int("keep", keepReleases))
		return nil
	}

	// Sort images by creation time (newest first)
	sort.Slice(images, func(i, j int) bool {
		return images[i].Created.After(images[j].Created)
	})

	// Remove old images (keep only the most recent keepReleases)
	for i, img := range images {
		if i < keepReleases {
			d.logger.Debug("Keeping image",
				slog.String("id", img.ID),
				slog.String("tag", img.Tag),
				slog.Time("created", img.Created))
			continue
		}

		d.logger.Info("Removing old image",
			slog.String("id", img.ID),
			slog.String("tag", img.Tag),
			slog.Time("created", img.Created))

		if err := d.containerRuntime.RemoveImage(ctx, img.ID); err != nil {
			d.logger.Warn("Failed to remove image",
				slog.String("id", img.ID),
				slog.String("error", err.Error()))
			// Continue with other images even if one fails
			continue
		}
	}

	return nil
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
func (d *Dewy) RunContainer() error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Get current image information from registry
	res, err := d.registry.Current(ctx)
	if err != nil {
		d.logger.Error("Failed to get current image", slog.String("error", err.Error()))
		return err
	}

	d.logger.Debug("Found latest image",
		slog.String("tag", res.Tag),
		slog.String("digest", res.ID),
		slog.String("url", res.ArtifactURL))

	// Extract image reference from artifact URL
	imageRef := strings.TrimPrefix(res.ArtifactURL, "img://")

	// Check if this version is already running
	dockerRuntime, err := container.NewDocker(d.logger.Logger, d.config.Container.DrainTime)
	if err != nil {
		return fmt.Errorf("failed to create Docker runtime: %w", err)
	}

	// Store runtime for shutdown handling
	d.containerRuntime = dockerRuntime

	runningID, err := dockerRuntime.GetRunningContainerWithImage(ctx, imageRef)
	if err != nil {
		d.logger.Warn("Failed to check running containers", slog.String("error", err.Error()))
		// Continue with deployment even if check fails
	} else if runningID != "" {
		d.logger.Debug("Container with this image is already running, skipping deployment",
			slog.String("version", res.Tag),
			slog.String("container", runningID))
		return nil
	}

	// Pull the image (this will be cached by Docker itself)
	if d.artifact == nil {
		d.artifact, err = artifact.New(ctx, res.ArtifactURL, d.logger.Logger)
		if err != nil {
			return fmt.Errorf("failed artifact.New: %w", err)
		}
	}

	buf := new(bytes.Buffer)
	err = d.artifact.Download(ctx, buf)
	d.artifact = nil
	if err != nil {
		return fmt.Errorf("failed to pull image: %w", err)
	}

	msg := fmt.Sprintf("Pulled image for `%s`", res.Tag)
	d.logger.Info("Pull notification", slog.String("message", msg))
	d.notifier.Send(ctx, msg)

	// Execute before deploy hook
	beforeResult, beforeErr := d.execHook(d.config.BeforeDeployHook)
	if beforeResult != nil {
		d.notifier.SendHookResult(ctx, "Before Deploy", beforeResult)
	}
	if beforeErr != nil {
		d.logger.Error("Before deploy hook failure", slog.String("error", beforeErr.Error()))
	}

	// Perform Blue-Green deployment
	if err := d.deployContainer(ctx, res); err != nil {
		d.logger.Error("Container deployment failed", slog.String("error", err.Error()))
		return err
	}

	// Execute after deploy hook
	afterResult, afterErr := d.execHook(d.config.AfterDeployHook)
	if afterResult != nil {
		d.notifier.SendHookResult(ctx, "After Deploy", afterResult)
	}
	if afterErr != nil {
		d.logger.Error("After deploy hook failure", slog.String("error", afterErr.Error()))
	}

	// Report deployment
	if !d.disableReport {
		d.logger.Debug("Report shipping")
		err := d.registry.Report(ctx, &registry.ReportRequest{
			ID:      res.ID,
			Tag:     res.Tag,
			Command: d.config.Command.String(),
		})
		if err != nil {
			d.logger.Error("Report shipping failure", slog.String("error", err.Error()))
		}
	}

	msg = fmt.Sprintf("Container deployed successfully for `%s`", res.Tag)
	d.logger.Info("Deployment notification", slog.String("message", msg))
	d.notifier.Send(ctx, msg)

	// Clean up old images
	d.logger.Info("Keep images", slog.Int("count", keepReleases))
	err = d.cleanupOldImages(ctx, imageRef)
	if err != nil {
		d.logger.Error("Keep images failure", slog.String("error", err.Error()))
	}

	return nil
}

// deployContainer performs the actual container deployment using Blue-Green strategy.
func (d *Dewy) deployContainer(ctx context.Context, res *registry.CurrentResponse) error {
	if d.config.Container == nil {
		return fmt.Errorf("container config is nil")
	}

	// Create container runtime
	var runtime container.Runtime
	var err error

	switch d.config.Container.Runtime {
	case "docker":
		runtime, err = container.NewDocker(d.logger.Logger, d.config.Container.DrainTime)
	case "podman":
		// TODO: Phase 2 - Podman support
		return fmt.Errorf("podman runtime not yet supported")
	default:
		return fmt.Errorf("unsupported runtime: %s", d.config.Container.Runtime)
	}

	if err != nil {
		return fmt.Errorf("failed to create container runtime: %w", err)
	}

	// Extract image reference from artifact URL
	// Format: img://registry/repo:tag
	imageRef := strings.TrimPrefix(res.ArtifactURL, "img://")

	// Determine app name from config or image
	appName := d.config.Container.Name
	if appName == "" {
		// Use repository name as app name
		parts := strings.Split(imageRef, "/")
		if len(parts) > 0 {
			lastPart := parts[len(parts)-1]
			appName = strings.Split(lastPart, ":")[0]
		}
	}

	// Deploy using Blue-Green strategy
	dockerRuntime, ok := runtime.(*container.Docker)
	if !ok {
		return fmt.Errorf("runtime is not Docker")
	}

	// Create health check function
	var healthCheck container.HealthCheckFunc
	if d.config.Container.HealthPath != "" {
		// Health check will use the mapped localhost port after deployment
		// For now, we'll implement a simple HTTP check via localhost
		healthCheck = func(ctx context.Context, containerID string) error {
			// Get mapped port
			mappedPort, err := dockerRuntime.GetMappedPort(ctx, containerID, d.config.Container.ContainerPort)
			if err != nil {
				return fmt.Errorf("failed to get mapped port for health check: %w", err)
			}

			// Check HTTP endpoint on localhost
			healthURL := fmt.Sprintf("http://localhost:%d%s", mappedPort, d.config.Container.HealthPath)
			client := &http.Client{Timeout: 5 * time.Second}

			retries := 5
			for i := 0; i < retries; i++ {
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
				if i < retries-1 {
					time.Sleep(2 * time.Second)
				}
			}
			return fmt.Errorf("health check failed after %d retries", retries)
		}
	} else {
		d.logger.Info("Health check disabled - container will start without health verification")
	}

	// Deploy with localhost-only port mapping (127.0.0.1::containerPort)
	// Docker will assign a random host port on localhost only for security
	// The built-in reverse proxy will connect to localhost:mappedPort
	deployOpts := container.DeployOptions{
		ImageRef:      imageRef,
		AppName:       appName,
		ContainerPort: d.config.Container.ContainerPort,
		Env:           d.config.Container.Env,
		Volumes:       d.config.Container.Volumes,
		Ports:         nil, // No explicit port mapping - using ContainerPort for localhost-only mapping
		HealthCheck:   healthCheck,
	}

	// Deploy container with callback to update proxy backend
	newContainerID, err := dockerRuntime.DeployContainerWithCallback(ctx, deployOpts, func(containerID string) error {
		// This callback is called after the new container passes health checks
		// but before the old container is stopped

		// Get the mapped host port
		mappedPort, err := dockerRuntime.GetMappedPort(ctx, containerID, d.config.Container.ContainerPort)
		if err != nil {
			return fmt.Errorf("failed to get mapped port: %w", err)
		}

		d.logger.Info("Container port mapped",
			slog.Int("container_port", d.config.Container.ContainerPort),
			slog.Int("host_port", mappedPort))

		// Atomically update proxy backend to point to the new container
		if err := d.updateProxyBackend("localhost", mappedPort); err != nil {
			return fmt.Errorf("failed to update proxy backend: %w", err)
		}

		return nil
	})

	if err != nil {
		return err
	}

	d.logger.Info("Container deployment completed",
		slog.String("container", newContainerID))

	return nil
}

// startProxy starts the built-in reverse proxy HTTP server.
func (d *Dewy) startProxy(ctx context.Context) error {
	// Create a reverse proxy handler with atomic backend switching
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		d.proxyMutex.RLock()
		backend := d.proxyBackend
		d.proxyMutex.RUnlock()

		if backend == nil {
			http.Error(w, "Service Unavailable - No backend configured", http.StatusServiceUnavailable)
			d.logger.Debug("Proxy request rejected - no backend configured",
				slog.String("method", r.Method),
				slog.String("path", r.URL.Path))
			return
		}

		// Create reverse proxy for current backend
		proxy := httputil.NewSingleHostReverseProxy(backend)

		// Custom error handler
		proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
			d.logger.Error("Proxy error",
				slog.String("backend", backend.String()),
				slog.String("method", r.Method),
				slog.String("path", r.URL.Path),
				slog.String("error", err.Error()))
			http.Error(w, "Bad Gateway", http.StatusBadGateway)
		}

		// Proxy the request
		d.logger.Debug("Proxying request",
			slog.String("backend", backend.String()),
			slog.String("method", r.Method),
			slog.String("path", r.URL.Path))
		proxy.ServeHTTP(w, r)
	})

	// Create HTTP server using the configured port
	addr := fmt.Sprintf(":%d", d.config.Port)
	d.proxyServer = &http.Server{
		Addr:    addr,
		Handler: handler,
	}

	// Start server in background
	go func() {
		d.logger.Info("Starting built-in reverse proxy",
			slog.String("address", addr))

		if err := d.proxyServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			d.logger.Error("Proxy server error", slog.String("error", err.Error()))
		}
	}()

	// Wait a moment to ensure server starts
	time.Sleep(100 * time.Millisecond)

	// Verify server is listening
	conn, err := net.DialTimeout("tcp", addr, 1*time.Second)
	if err != nil {
		return fmt.Errorf("proxy server failed to start: %w", err)
	}
	conn.Close()

	d.logger.Info("Built-in reverse proxy started successfully",
		slog.String("listen", addr))

	return nil
}

// updateProxyBackend atomically updates the reverse proxy backend.
func (d *Dewy) updateProxyBackend(host string, port int) error {
	backend, err := url.Parse(fmt.Sprintf("http://%s:%d", host, port))
	if err != nil {
		return fmt.Errorf("failed to parse backend URL: %w", err)
	}

	d.proxyMutex.Lock()
	d.proxyBackend = backend
	d.proxyMutex.Unlock()

	d.logger.Info("Proxy backend updated",
		slog.String("backend", backend.String()))

	return nil
}

// stopProxy gracefully shuts down the reverse proxy server.
func (d *Dewy) stopProxy(ctx context.Context) error {
	if d.proxyServer == nil {
		return nil
	}

	d.logger.Info("Stopping reverse proxy")

	shutdownCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	if err := d.proxyServer.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("failed to shutdown proxy: %w", err)
	}

	d.logger.Info("Reverse proxy stopped")
	return nil
}

// stopManagedContainers stops all containers managed by dewy.
func (d *Dewy) stopManagedContainers(ctx context.Context) error {
	if d.containerRuntime == nil {
		return nil
	}

	d.logger.Info("Stopping managed containers")

	// Find all containers with dewy.managed=true label
	containerID, err := d.containerRuntime.FindContainerByLabel(ctx, map[string]string{
		"dewy.managed": "true",
	})
	if err != nil {
		if err.Error() == "container not found" {
			d.logger.Debug("No managed containers found to stop")
			return nil
		}
		return fmt.Errorf("failed to find managed containers: %w", err)
	}

	// Stop the container with graceful timeout
	timeout := 10 * time.Second
	if err := d.containerRuntime.Stop(ctx, containerID, timeout); err != nil {
		d.logger.Error("Failed to stop container",
			slog.String("container", containerID),
			slog.String("error", err.Error()))
		return fmt.Errorf("failed to stop container: %w", err)
	}

	d.logger.Info("Managed container stopped",
		slog.String("container", containerID))

	// Remove the container
	if err := d.containerRuntime.Remove(ctx, containerID); err != nil {
		d.logger.Warn("Failed to remove container",
			slog.String("container", containerID),
			slog.String("error", err.Error()))
		// Don't return error as the important part (stopping) succeeded
	} else {
		d.logger.Info("Managed container removed",
			slog.String("container", containerID))
	}

	return nil
}
