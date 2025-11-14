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
	proxyBackends    []*url.URL // Multiple backends for load balancing
	proxyIndex       int        // Round-robin counter
	proxyMutex       sync.RWMutex
	containerRuntime container.Runtime
	cVer             string // Current deployed version (tag)
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

	// Save current version
	d.Lock()
	d.cVer = res.Tag
	d.Unlock()

	if d.config.Command == SERVER {
		if d.isServerRunning {
			err = d.restartServer()
			if err == nil {
				msg := fmt.Sprintf("Server restarted for `%s`", d.cVer)
				d.logger.Info("Restart notification", slog.String("message", msg))
				d.notifier.Send(ctx, msg)
			}
		} else {
			err = d.startServer()
			if err == nil {
				msg := fmt.Sprintf("Server started for `%s`", d.cVer)
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

	// Save current version
	d.Lock()
	d.cVer = res.Tag
	d.Unlock()

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

	msg = fmt.Sprintf("Container deployed successfully for `%s`", d.cVer)
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

// deployContainer performs the actual container deployment using rolling update strategy.
func (d *Dewy) deployContainer(ctx context.Context, res *registry.CurrentResponse) error {
	if d.config.Container == nil {
		return fmt.Errorf("container config is nil")
	}

	// Get replicas count (default: 1)
	replicas := d.config.Container.Replicas
	if replicas <= 0 {
		replicas = 1
	}

	d.logger.Info("Starting container deployment",
		slog.Int("replicas", replicas))

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

	// Get Docker runtime
	dockerRuntime, ok := runtime.(*container.Docker)
	if !ok {
		return fmt.Errorf("runtime is not Docker")
	}

	// Pull the new image first
	d.logger.Info("Pulling new image", slog.String("image", imageRef))
	if err := dockerRuntime.Pull(ctx, imageRef); err != nil {
		return fmt.Errorf("pull failed: %w", err)
	}

	// Find existing containers
	existingContainers, err := dockerRuntime.FindContainersByLabel(ctx, map[string]string{
		"dewy.managed": "true",
		"dewy.app":     appName,
	})
	if err != nil {
		return fmt.Errorf("failed to find existing containers: %w", err)
	}

	d.logger.Info("Found existing containers",
		slog.Int("count", len(existingContainers)))

	// Create health check function
	healthCheck := d.createHealthCheckFunc(dockerRuntime)

	// Rolling update: start new containers one by one
	newContainers := make([]string, 0, replicas)
	newBackends := make([]*url.URL, 0, replicas)

	for i := 0; i < replicas; i++ {
		d.logger.Info("Starting new container",
			slog.String("version", res.Tag),
			slog.Int("replica", i+1),
			slog.Int("total", replicas))

		// Start new container
		containerName := fmt.Sprintf("%s-%d", appName, time.Now().Unix())
		containerID, mappedPort, err := d.startSingleContainer(ctx, dockerRuntime, imageRef, appName, containerName, healthCheck)
		if err != nil {
			// Rollback: remove newly created containers
			d.logger.Error("Failed to start container, rolling back",
				slog.Int("replica", i+1),
				slog.String("error", err.Error()))
			d.rollbackContainers(ctx, dockerRuntime, newContainers)
			return err
		}

		newContainers = append(newContainers, containerID)
		backend, _ := url.Parse(fmt.Sprintf("http://localhost:%d", mappedPort))
		newBackends = append(newBackends, backend)

		// Add to proxy backends
		if err := d.addProxyBackend("localhost", mappedPort); err != nil {
			d.logger.Error("Failed to add proxy backend",
				slog.String("error", err.Error()))
			d.rollbackContainers(ctx, dockerRuntime, newContainers)
			return err
		}

		d.logger.Info("Container added to load balancer",
			slog.String("container", containerID),
			slog.Int("backend_count", len(newBackends)))
	}

	// Remove old containers one by one
	for i, oldContainerID := range existingContainers {
		d.logger.Info("Removing old container",
			slog.Int("index", i+1),
			slog.Int("total", len(existingContainers)),
			slog.String("container", oldContainerID))

		// Get old container port to remove from proxy
		oldPort, err := dockerRuntime.GetMappedPort(ctx, oldContainerID, d.config.Container.ContainerPort)
		if err == nil {
			// Remove from proxy backends
			if err := d.removeProxyBackend("localhost", oldPort); err != nil {
				d.logger.Warn("Failed to remove old backend from proxy",
					slog.String("error", err.Error()))
			}
		}

		// Stop and remove old container
		if err := dockerRuntime.Stop(ctx, oldContainerID, 10*time.Second); err != nil {
			d.logger.Error("Failed to stop old container",
				slog.String("container", oldContainerID),
				slog.String("error", err.Error()))
		}
		if err := dockerRuntime.Remove(ctx, oldContainerID); err != nil {
			d.logger.Error("Failed to remove old container",
				slog.String("container", oldContainerID),
				slog.String("error", err.Error()))
		}
	}

	d.logger.Info("Container deployment completed",
		slog.Int("new_containers", len(newContainers)),
		slog.Int("removed_containers", len(existingContainers)))

	return nil
}

// createHealthCheckFunc creates a health check function based on configuration.
func (d *Dewy) createHealthCheckFunc(dockerRuntime *container.Docker) container.HealthCheckFunc {
	if d.config.Container.HealthPath == "" {
		d.logger.Info("Health check disabled - container will start without health verification")
		return nil
	}

	return func(ctx context.Context, containerID string) error {
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
}

// startSingleContainer starts a single container and returns its ID and mapped port.
func (d *Dewy) startSingleContainer(ctx context.Context, dockerRuntime *container.Docker, imageRef, appName, containerName string, healthCheck container.HealthCheckFunc) (string, int, error) {
	// Prepare port mapping for localhost-only access
	ports := []string{fmt.Sprintf("127.0.0.1::%d", d.config.Container.ContainerPort)}

	// Start container
	containerID, err := dockerRuntime.Run(ctx, container.RunOptions{
		Image:   imageRef,
		Name:    containerName,
		Env:     d.config.Container.Env,
		Volumes: d.config.Container.Volumes,
		Ports:   ports,
		Labels: map[string]string{
			"dewy.managed": "true",
			"dewy.app":     appName,
		},
		Detach: true,
	})
	if err != nil {
		return "", 0, fmt.Errorf("failed to start container: %w", err)
	}

	// Get mapped port
	mappedPort, err := dockerRuntime.GetMappedPort(ctx, containerID, d.config.Container.ContainerPort)
	if err != nil {
		rErr := dockerRuntime.Remove(ctx, containerID)
		return "", 0, errors.Join(
			fmt.Errorf("failed to get mapped port: %w", err),
			fmt.Errorf("runtime remove failed: %w", rErr),
		)
	}

	d.logger.Info("Container started",
		slog.String("container", containerID),
		slog.String("name", containerName),
		slog.Int("mapped_port", mappedPort))

	// Perform health check if configured
	if healthCheck != nil {
		// Give the container a moment to start
		time.Sleep(3 * time.Second)

		d.logger.Info("Performing health check", slog.String("container", containerID))
		if err := healthCheck(ctx, containerID); err != nil {
			sErr := dockerRuntime.Stop(ctx, containerID, 5*time.Second)
			rErr := dockerRuntime.Remove(ctx, containerID)
			return "", 0, errors.Join(
				fmt.Errorf("health check failed: %w", err),
				fmt.Errorf("runtime stop failed: %w", sErr),
				fmt.Errorf("runtime remove failed: %w", rErr),
			)
		}
	}

	return containerID, mappedPort, nil
}

// rollbackContainers removes all containers in the list (used for rollback).
func (d *Dewy) rollbackContainers(ctx context.Context, dockerRuntime *container.Docker, containerIDs []string) {
	d.logger.Info("Rolling back containers", slog.Int("count", len(containerIDs)))
	for _, containerID := range containerIDs {
		if err := dockerRuntime.Stop(ctx, containerID, 5*time.Second); err != nil {
			d.logger.Error("Failed to stop container during rollback",
				slog.String("container", containerID),
				slog.String("error", err.Error()))
		}
		if err := dockerRuntime.Remove(ctx, containerID); err != nil {
			d.logger.Error("Failed to remove container during rollback",
				slog.String("container", containerID),
				slog.String("error", err.Error()))
		}
	}
}

// startProxy starts the built-in reverse proxy HTTP server.
func (d *Dewy) startProxy(ctx context.Context) error {
	// Create a reverse proxy handler with round-robin load balancing
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		d.proxyMutex.Lock()
		backends := d.proxyBackends
		if len(backends) == 0 {
			d.proxyMutex.Unlock()
			http.Error(w, "Service Unavailable - No backend configured", http.StatusServiceUnavailable)
			d.logger.Debug("Proxy request rejected - no backend configured",
				slog.String("method", r.Method),
				slog.String("path", r.URL.Path))
			return
		}

		// Round-robin: select next backend
		backend := backends[d.proxyIndex%len(backends)]
		d.proxyIndex++
		d.proxyMutex.Unlock()

		// Create reverse proxy for selected backend
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
		Addr:              addr,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second, // For slowloris attack
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

// addProxyBackend adds a new backend to the load balancer pool.
func (d *Dewy) addProxyBackend(host string, port int) error {
	backend, err := url.Parse(fmt.Sprintf("http://%s:%d", host, port))
	if err != nil {
		return fmt.Errorf("failed to parse backend URL: %w", err)
	}

	d.proxyMutex.Lock()
	d.proxyBackends = append(d.proxyBackends, backend)
	d.proxyMutex.Unlock()

	d.logger.Info("Proxy backend added",
		slog.String("backend", backend.String()),
		slog.Int("total_backends", len(d.proxyBackends)))

	return nil
}

// removeProxyBackend removes a backend from the load balancer pool.
func (d *Dewy) removeProxyBackend(host string, port int) error {
	targetURL := fmt.Sprintf("http://%s:%d", host, port)

	d.proxyMutex.Lock()
	defer d.proxyMutex.Unlock()

	newBackends := make([]*url.URL, 0, len(d.proxyBackends))
	for _, backend := range d.proxyBackends {
		if backend.String() != targetURL {
			newBackends = append(newBackends, backend)
		}
	}

	if len(newBackends) == len(d.proxyBackends) {
		return fmt.Errorf("backend not found: %s", targetURL)
	}

	d.proxyBackends = newBackends
	d.proxyIndex = 0 // Reset index

	d.logger.Info("Proxy backend removed",
		slog.String("backend", targetURL),
		slog.Int("remaining_backends", len(d.proxyBackends)))

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

	// Type assert to get access to FindContainersByLabel
	dockerRuntime, ok := d.containerRuntime.(*container.Docker)
	if !ok {
		return fmt.Errorf("runtime is not Docker")
	}

	// Find all containers with dewy.managed=true label
	containerIDs, err := dockerRuntime.FindContainersByLabel(ctx, map[string]string{
		"dewy.managed": "true",
	})
	if err != nil {
		return fmt.Errorf("failed to find managed containers: %w", err)
	}

	if len(containerIDs) == 0 {
		d.logger.Debug("No managed containers found to stop")
		return nil
	}

	d.logger.Info("Found managed containers to stop", slog.Int("count", len(containerIDs)))

	// Stop and remove all containers
	timeout := 10 * time.Second
	stoppedCount := 0
	removedCount := 0

	for _, containerID := range containerIDs {
		// Stop the container with graceful timeout
		if err := d.containerRuntime.Stop(ctx, containerID, timeout); err != nil {
			d.logger.Error("Failed to stop container",
				slog.String("container", containerID),
				slog.String("error", err.Error()))
			// Continue to try stopping other containers
			continue
		}

		d.logger.Info("Managed container stopped",
			slog.String("container", containerID))
		stoppedCount++

		// Remove the container
		if err := d.containerRuntime.Remove(ctx, containerID); err != nil {
			d.logger.Warn("Failed to remove container",
				slog.String("container", containerID),
				slog.String("error", err.Error()))
			// Don't return error as the important part (stopping) succeeded
		} else {
			d.logger.Info("Managed container removed",
				slog.String("container", containerID))
			removedCount++
		}
	}

	d.logger.Info("Cleanup completed",
		slog.Int("stopped", stoppedCount),
		slog.Int("removed", removedCount),
		slog.Int("total", len(containerIDs)))

	return nil
}
