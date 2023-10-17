package dewy

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/carlescere/scheduler"
	"github.com/gorilla/schema"
	starter "github.com/lestrrat-go/server-starter"
	"github.com/linyows/dewy/kvs"
	"github.com/linyows/dewy/notice"
	"github.com/linyows/dewy/registry"
	ghrelease "github.com/linyows/dewy/registry/github_release"
	grpcr "github.com/linyows/dewy/registry/grpc"
	"github.com/linyows/dewy/storage"
)

const (
	ISO8601      = "20060102T150405Z0700"
	releaseDir   = ISO8601
	releasesDir  = "releases"
	symlinkDir   = "current"
	keepReleases = 7
)

var decoder = schema.NewDecoder()

// Dewy struct.
type Dewy struct {
	config          Config
	registry        registry.Registry
	cache           kvs.KVS
	isServerRunning bool
	disableReport   bool
	root            string
	job             *scheduler.Job
	notice          notice.Notice
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
	if c.PreRelease {
		v := u.Query()
		v.Add("pre-release", "true")
		u.RawQuery = v.Encode()
	}
	if c.ArtifactName != "" {
		v := u.Query()
		v.Add("artifact", c.ArtifactName)
		u.RawQuery = v.Encode()
	}
	c.Registry = fmt.Sprintf("%s://%s", su[0], u.String())

	r, err := newRegistry(c.Registry)
	if err != nil {
		return nil, err
	}

	return &Dewy{
		config:          c,
		cache:           kv,
		registry:        r,
		isServerRunning: false,
		root:            wd,
	}, nil
}

// Start dewy.
func (d *Dewy) Start(i int) {
	ctx, cancel := context.WithCancel(context.WithValue(context.Background(), notice.MetaContextKey, true))
	defer cancel()
	var err error

	nc := &notice.Config{
		Source:  d.config.ArtifactName,
		Command: d.config.Command.String(),
	}
	repo, ok := d.registry.(*ghrelease.GithubRelease)
	if ok {
		nc.Owner = repo.Owner()
		nc.Repo = repo.Repo()
		nc.OwnerLink = repo.OwnerURL()
		nc.OwnerIcon = repo.OwnerIconURL()
		nc.RepoLink = repo.URL()
	}

	d.notice, err = notice.New(&notice.Slack{Meta: nc})
	if err != nil {
		log.Printf("[ERROR] Notice failure: %#v", err)
		return
	}
	d.notice.Notify(ctx, "Automatic shipping started by Dewy")
	ctx, cancel = context.WithCancel(context.Background())
	defer cancel()

	d.job, err = scheduler.Every(i).Seconds().Run(func() {
		e := d.Run()
		if e != nil {
			log.Printf("[ERROR] Dewy run failure: %#v", e)
		}
	})
	if err != nil {
		log.Printf("[ERROR] Scheduler failure: %#v", err)
	}

	d.notice.Notify(ctx, fmt.Sprintf("Stop receiving \"%s\" signal", d.waitSigs()))
}

func (d *Dewy) waitSigs() os.Signal {
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)
	sigReceived := <-sigCh
	log.Printf("[DEBUG] PID %d received signal as %s", os.Getpid(), sigReceived)
	d.job.Quit <- true
	return sigReceived
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
	cacheKey := fmt.Sprintf("%s-%s", res.Tag, filepath.Base(res.ArtifactURL))
	currentKey := "current.txt"
	currentSourceKey, _ := d.cache.Read(currentKey)
	found := false
	list, err := d.cache.List()
	if err != nil {
		return err
	}
	for _, key := range list {
		// same current version and already cached
		if string(currentSourceKey) == cacheKey && key == cacheKey {
			log.Print("[DEBUG] Deploy skipped")
			break
		}

		// no current version but already cached
		if key == cacheKey {
			found = true
			break
		}
	}

	// Download artifact and cache
	if !found {
		buf := new(bytes.Buffer)
		if err := storage.Fetch(res.ArtifactURL, buf); err != nil {
			return err
		}
		if err := d.cache.Write(cacheKey, buf.Bytes()); err != nil {
			return err
		}
		log.Printf("[INFO] Cached as %s", cacheKey)
	}

	if d.notice != nil {
		d.notice.Notify(ctx, fmt.Sprintf("New shipping <%s|%s> was detected",
			res.ArtifactURL, res.Tag))
	}

	if err := d.deploy(cacheKey); err != nil {
		return err
	}

	if d.config.Command == SERVER {
		if d.isServerRunning {
			d.notice.Notify(ctx, "Server restarting")
			err = d.restartServer()
		} else {
			d.notice.Notify(ctx, "Server starting")
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

func (d *Dewy) deploy(key string) error {

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

func newRegistry(urlstr string) (registry.Registry, error) {
	su := strings.SplitN(urlstr, "://", 2)
	switch su[0] {
	case ghrelease.Scheme:
		u, err := url.Parse(su[1])
		if err != nil {
			return nil, err
		}
		var c ghrelease.Config
		if err := decoder.Decode(&c, u.Query()); err != nil {
			return nil, err
		}
		ownerrepo := strings.SplitN(u.Path, "/", 2)
		c.Owner = ownerrepo[0]
		c.Repo = ownerrepo[1]
		return ghrelease.New(c)
	case grpcr.Scheme:
		u, err := url.Parse(urlstr)
		if err != nil {
			return nil, err
		}
		var c grpcr.Config
		if err := decoder.Decode(&c, u.Query()); err != nil {
			return nil, err
		}
		c.Target = u.Host
		return grpcr.New(c)
	}
	return nil, fmt.Errorf("unsupported registry: %s", urlstr)
}
