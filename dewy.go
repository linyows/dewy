package dewy

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"net/url"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
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
	"github.com/linyows/dewy/notify"
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
	cache           kvs.KVS
	isServerRunning bool
	disableReport   bool
	root            string
	job             *scheduler.Job
	notify          notify.Notify
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

	// Add deprecated flags to registry url.
	su := strings.SplitN(c.Registry, "://", 2)
	u, err := url.Parse(su[1])
	if err != nil {
		return nil, err
	}
	c.Registry = fmt.Sprintf("%s://%s", su[0], u.String())

	r, err := registry.New(c.Registry)
	if err != nil {
		return nil, err
	}

	n, err := notify.New(c.Notify)
	if err != nil {
		return nil, err
	}

	return &Dewy{
		config:          c,
		cache:           kv,
		registry:        r,
		notify:          n,
		isServerRunning: false,
		root:            wd,
	}, nil
}

// Start dewy.
func (d *Dewy) Start(i int) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	d.notify.Send(ctx, "Automatic shipping started by Dewy")

	var err error
	d.job, err = scheduler.Every(i).Seconds().Run(func() {
		e := d.Run()
		if e != nil {
			log.Printf("[ERROR] Dewy run failure: %#v", e)
		}
	})
	if err != nil {
		log.Printf("[ERROR] Scheduler failure: %#v", err)
	}

	d.notify.Send(ctx, fmt.Sprintf("Stop receiving \"%s\" signal", d.waitSigs()))
}

func (d *Dewy) waitSigs() os.Signal {
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)
	sigReceived := <-sigCh
	log.Printf("[DEBUG] PID %d received signal as %s", os.Getpid(), sigReceived)
	d.job.Quit <- true
	return sigReceived
}

// cachekeyName is "tag--artifact"
// example: v1.2.3--testapp_linux_amd64.tar.gz
func (d *Dewy) cachekeyName(res *registry.CurrentResponse) string {
	return fmt.Sprintf("%s--%s", res.Tag, filepath.Base(res.ArtifactURL))
}

// Run dewy.
func (d *Dewy) Run() error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Get current
	res, err := d.registry.Current(ctx, &registry.CurrentRequest{
		Arch:         runtime.GOARCH,
		OS:           runtime.GOOS,
		ArtifactName: d.config.ArtifactName,
	})
	if err != nil {
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
			break
		}
	}

	// Download artifact and cache
	if !found {
		buf := new(bytes.Buffer)
		if err := artifact.Fetch(res.ArtifactURL, buf); err != nil {
			return err
		}
		if err := d.cache.Write(cachekeyName, buf.Bytes()); err != nil {
			return err
		}
		if err := d.cache.Write(currentkeyName, []byte(cachekeyName)); err != nil {
			return err
		}
		log.Printf("[INFO] Cached as %s", cachekeyName)
	}

	d.notify.Send(ctx, fmt.Sprintf("New shipping <%s|%s> was detected", res.ArtifactURL, res.Tag))

	if err := d.deploy(cachekeyName); err != nil {
		return err
	}

	if d.config.Command == SERVER {
		if d.isServerRunning {
			d.notify.Send(ctx, "Server restarting")
			err = d.restartServer()
		} else {
			d.notify.Send(ctx, "Server starting")
			err = d.startServer()
		}
		if err != nil {
			log.Printf("[ERROR] Server failure: %#v", err)
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

	p, _ := os.FindProcess(os.Getpid())
	err := p.Signal(syscall.SIGHUP)
	if err != nil {
		return err
	}
	log.Print("[INFO] Send SIGHUP for server restart")

	return nil
}

func (d *Dewy) startServer() error {
	d.Lock()
	defer d.Unlock()

	d.isServerRunning = true

	log.Print("[INFO] Start server")
	ch := make(chan error)

	go func() {
		s, err := starter.NewStarter(d.config.Starter)
		if err != nil {
			log.Printf("[ERROR] Starter failure: %#v", err)
			return
		}

		ch <- s.Run()
	}()

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
