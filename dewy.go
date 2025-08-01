package dewy

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log"
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
	starter "github.com/lestrrat-go/server-starter"
	"github.com/linyows/dewy/artifact"
	"github.com/linyows/dewy/kvs"
	"github.com/linyows/dewy/notifier"
	"github.com/linyows/dewy/registry"
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
	sync.RWMutex
}

// New returns Dewy.
func New(c Config) (*Dewy, error) {
	kv := &kvs.File{}
	kv.Default()

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
	}, nil
}

// Start dewy.
func (d *Dewy) Start(i int) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var err error

	d.registry, err = registry.New(ctx, d.config.Registry)
	if err != nil {
		log.Printf("[ERROR] Registry failure: %#v", err)
	}

	d.notifier, err = notifier.New(ctx, d.config.Notifier)
	if err != nil {
		log.Printf("[ERROR] Notifier failure: %#v", err)
	}

	d.notifier.Send(ctx, "Automatic shipping started by *Dewy*")

	d.job, err = scheduler.Every(i).Seconds().Run(func() {
		e := d.Run()
		if e != nil {
			log.Printf("[ERROR] Dewy run failure: %#v", e)
			d.notifier.SendError(context.Background(), e)
		} else {
			d.notifier.ResetErrorCount()
		}
	})
	if err != nil {
		log.Printf("[ERROR] Scheduler failure: %#v", err)
	}

	d.waitSigs(ctx)
}

func (d *Dewy) waitSigs(ctx context.Context) {
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGHUP, syscall.SIGUSR1, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)

	for sig := range sigCh {
		log.Printf("[DEBUG] PID %d received signal as %s", os.Getpid(), sig)
		switch sig {
		case syscall.SIGHUP:
			continue

		case syscall.SIGUSR1:
			if err := d.restartServer(); err != nil {
				log.Printf("[ERROR] Restart failure: %#v", err)
			} else {
				msg := fmt.Sprintf("Restarted receiving by \"%s\" signal", "SIGUSR1")
				log.Printf("[INFO] %s", msg)
				d.notifier.Send(ctx, msg)
			}
			continue

		case syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT:
			d.job.Quit <- true
			msg := fmt.Sprintf("Stop receiving by \"%s\" signal", sig)
			log.Printf("[INFO] %s", msg)
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
				log.Printf("[DEBUG] Artifact not found within 30 minute grace period, skipping error notification: %s", 
					artifactNotFoundErr.Message)
				return nil // Return nil to avoid error notification
			}
		}
		log.Printf("[ERROR] Current failure: %#v", err)
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
			log.Print("[DEBUG] Deploy skipped")
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
			d.artifact, err = artifact.New(ctx, res.ArtifactURL)
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
		log.Printf("[INFO] Cached as %s", cachekeyName)
	}

	d.notifier.Send(ctx, fmt.Sprintf("Ready for `%s`", res.Tag))

	if err := d.deploy(cachekeyName); err != nil {
		return err
	}

	if d.config.Command == SERVER {
		if d.isServerRunning {
			err = d.restartServer()
			if err == nil {
				d.notifier.Send(ctx, fmt.Sprintf("Server restarted for `%s`", res.Tag))
			}
		} else {
			err = d.startServer()
			if err == nil {
				d.notifier.Send(ctx, fmt.Sprintf("Server started for `%s`", res.Tag))
			}
		}
		if err != nil {
			log.Printf("[ERROR] Server failure: %#v", err)
			return err
		}
	}

	if !d.disableReport {
		log.Print("[DEBUG] Report shipping")
		err := d.registry.Report(ctx, &registry.ReportRequest{
			ID:  res.ID,
			Tag: res.Tag,
		})
		if err != nil {
			log.Printf("[ERROR] Report shipping failure: %#v", err)
		}
	}

	log.Printf("[INFO] Keep releases as %d", keepReleases)
	err = d.keepReleases()
	if err != nil {
		log.Printf("[ERROR] Keep releases failure: %#v", err)
	}

	return nil
}

func (d *Dewy) deploy(key string) (err error) {
	if err := d.execHook(d.config.BeforeDeployHook); err != nil {
		log.Printf("[ERROR] Before deploy hook failure: %#v", err)
		return err
	}
	defer func() {
		if err != nil {
			return
		}
		// When deploy is success, run after deploy hook
		if err := d.execHook(d.config.AfterDeployHook); err != nil {
			log.Printf("[ERROR] After deploy hook failure: %#v", err)
		}
	}()
	p := filepath.Join(d.cache.GetDir(), key)
	linkFrom, err := d.preserve(p)
	if err != nil {
		log.Printf("[ERROR] Preserve failure: %#v", err)
		return err
	}
	log.Printf("[INFO] Extract archive to %s", linkFrom)

	linkTo := filepath.Join(d.root, symlinkDir)
	if _, err := os.Lstat(linkTo); err == nil {
		os.Remove(linkTo)
	}

	log.Printf("[INFO] Create symlink to %s from %s", linkTo, linkFrom)
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
	log.Printf("[INFO] Send SIGHUP to PID:%d for server restart", pid)

	return nil
}

func (d *Dewy) startServer() error {
	d.Lock()
	defer d.Unlock()

	log.Print("[INFO] Start server")
	
	// Try to create starter first (synchronous validation)
	s, err := starter.NewStarter(d.config.Starter)
	if err != nil {
		log.Printf("[ERROR] Starter failure: %#v", err)
		return err
	}

	// Start server in background
	go func() {
		err := s.Run()
		if err != nil {
			log.Printf("[ERROR] Server run failure: %#v", err)
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
		log.Printf("[INFO] execute hook: command=%q stdout=%q stderr=%q", cmd, stdout.String(), stderr.String())
	}()
	if err := c.Run(); err != nil {
		return err
	}
	return nil
}

