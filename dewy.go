package dewy

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"sort"
	"sync"
	"syscall"
	"time"

	"github.com/carlescere/scheduler"
	starter "github.com/lestrrat-go/server-starter"
	"github.com/linyows/dewy/kvs"
	"github.com/linyows/dewy/notice"
	"github.com/linyows/dewy/repo"
)

const (
	releaseDir   = repo.ISO8601
	releasesDir  = "releases"
	symlinkDir   = "current"
	keepReleases = 7
)

// Dewy struct
type Dewy struct {
	config          Config
	repo            repo.Repo
	cache           kvs.KVS
	isServerRunning bool
	root            string
	job             *scheduler.Job
	notice          notice.Notice
	sync.RWMutex
}

// New returns Dewy
func New(c Config) (*Dewy, error) {
	kv := &kvs.File{}
	kv.Default()

	wd, err := os.Getwd()
	if err != nil {
		return nil, err
	}

	r, err := repo.New(c.Repository, kv)
	if err != nil {
		return nil, err
	}

	return &Dewy{
		config:          c,
		cache:           kv,
		repo:            r,
		isServerRunning: false,
		root:            wd,
	}, nil
}

// Start dewy
func (d *Dewy) Start(i int) {
	ctx, cancel := context.WithCancel(context.WithValue(context.Background(), notice.MetaContextKey, true))
	defer cancel()
	var err error

	d.notice, err = notice.New(&notice.Slack{Meta: &notice.Config{
		RepoOwnerLink: d.repo.OwnerURL(),
		RepoOwnerIcon: d.repo.OwnerIconURL(),
		RepoLink:      d.repo.URL(),
		RepoOwner:     d.config.Repository.Owner,
		RepoName:      d.config.Repository.Name,
		Source:        d.config.Repository.Artifact,
		Command:       d.config.Command.String(),
	}})
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

// Run dewy
func (d *Dewy) Run() error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := d.repo.Fetch(); err != nil {
		log.Printf("[ERROR] Fetch failure: %#v", err)
		return err
	}

	key, err := d.repo.GetDeploySourceKey()
	if err != nil {
		if err.Error() == "No need to deploy" {
			log.Print("[DEBUG] Deploy skipped")
		} else {
			log.Printf("[DEBUG] Download failure: %#v", err)
		}
		return nil
	}

	if d.notice != nil {
		d.notice.Notify(ctx, fmt.Sprintf("New shipping <%s|%s> was detected",
			d.repo.ReleaseURL(), d.repo.ReleaseTag()))
	}

	if err := d.deploy(key); err != nil {
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

	log.Print("[DEBUG] Record shipping")
	err = d.repo.RecordShipping()
	if err != nil {
		log.Printf("[ERROR] Record shipping failure: %#v", err)
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
