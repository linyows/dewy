package dewy

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
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
	keepReleases = 7

	// currentkeyName is a name whose value is the version of the currently running server application.
	// For example, if you are using a file for the cache store, running `cat current` will show `v1.2.3--app_linux_amd64.tar.gz`, which is a combination of the tag and artifact.
	// dewy uses this value as a key (**cachekeyName**) to manage the artifacts in the cache store.
	currentkeyName = "current"
)

// Dewy struct.
type Dewy struct {
	config          Config
	registry        registry.Registry
	artifact        artifact.Artifact
	cache           kvs.KVS
	isServerRunning bool
	disableReport   bool
	root            string
	job             *scheduler.Job
	notifier        notifier.Notifier
	logger          *logging.Logger
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

	msg := fmt.Sprintf("Automatic shipping started by *Dewy* (v%s)", d.config.Version)
	d.logger.Info("Dewy start notification", slog.String("message", msg))
	d.notifier.Send(ctx, msg)

	d.job, err = scheduler.Every(i).Seconds().Run(func() {
		e := d.Run()
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
		// Check if this is an artifact not found error within 30 minute grace period
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
			if d.isServerRunning {
				return nil
			}
			// when the server fails to start
			break
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
			ID:  res.ID,
			Tag: res.Tag,
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
	if err := d.execHook(d.config.BeforeDeployHook); err != nil {
		d.logger.Error("Before deploy hook failure", slog.String("error", err.Error()))
		return err
	}
	defer func() {
		if err != nil {
			return
		}
		// When deploy is success, run after deploy hook
		if err := d.execHook(d.config.AfterDeployHook); err != nil {
			d.logger.Error("After deploy hook failure", slog.String("error", err.Error()))
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

func (d *Dewy) execHook(cmd string) error {
	if cmd == "" {
		return nil
	}
	sh, err := safeexec.LookPath("sh")
	if err != nil {
		return err
	}
	stdout := new(bytes.Buffer)
	stderr := new(bytes.Buffer)
	c := exec.Command(sh, "-c", cmd)
	c.Dir = d.root
	c.Env = os.Environ()
	c.Stdout = stdout
	c.Stderr = stderr
	defer func() {
		d.logger.Info("Execute hook",
			slog.String("command", cmd),
			slog.String("stdout", stdout.String()),
			slog.String("stderr", stderr.String()))
	}()
	if err := c.Run(); err != nil {
		return err
	}
	return nil
}
