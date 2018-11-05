package dewy

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"os/user"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"github.com/carlescere/scheduler"
	starter "github.com/lestrrat-go/server-starter"
	"github.com/linyows/dewy/kvs"
	"github.com/linyows/dewy/notice"
)

const (
	// ISO8601 for time format
	ISO8601     = "20060102T150405Z0700"
	releaseDir  = ISO8601
	releasesDir = "releases"
	symlinkDir  = "current"
)

// Dewy struct
type Dewy struct {
	config          Config
	repository      Repository
	cache           kvs.KVS
	isServerRunning bool
	root            string
	job             *scheduler.Job
	notice          notice.Notice
	sync.RWMutex
}

// New returns Dewy
func New(c Config) *Dewy {
	kv := &kvs.File{}
	kv.Default()

	wd, err := os.Getwd()
	if err != nil {
		panic(err)
	}

	return &Dewy{
		config:          c,
		cache:           kv,
		repository:      NewRepository(c.Repository, kv),
		isServerRunning: false,
		root:            wd,
	}
}

// Start dewy
func (d *Dewy) Start(i int) {
	ctx, cancel := context.WithCancel(context.WithValue(context.Background(), notice.MetaContextKey, true))
	defer cancel()

	d.notice = notice.New(&notice.Slack{Meta: &notice.Config{
		RepoOwnerLink:    d.repository.OwnerURL(),
		RepoOwnerIcon:    d.repository.OwnerIconURL(),
		RepoLink:         d.repository.URL(),
		RepoOwner:        d.config.Repository.Owner,
		RepoName:         d.config.Repository.Name,
		Source:           d.config.Repository.Artifact,
		Command:          d.config.Command.String(),
		Host:             hostname(),
		User:             username(),
		WorkingDirectory: cwd(),
	}})

	d.notice.Notify(ctx, "Automatic shipping started by Dewy")
	ctx, cancel = context.WithCancel(context.Background())

	var err error
	d.job, err = scheduler.Every(i).Seconds().Run(func() {
		d.Run()
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

	if err := d.repository.Fetch(); err != nil {
		log.Printf("[ERROR] Fetch failure: %#v", err)
		return err
	}

	if !d.repository.IsDownloadNecessary() {
		log.Print("[DEBUG] Download skipped")
		return nil
	}

	key, err := d.repository.Download()
	if err != nil {
		log.Printf("[DEBUG] Download failure: %#v", err)
		return nil
	}

	d.notice.Notify(ctx, fmt.Sprintf("New release <%s|%s> was downloaded",
		d.repository.ReleaseURL(), d.repository.ReleaseTag()))

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
	}

	log.Print("[DEBUG] Record shipment")
	err = d.repository.RecordShipment()
	if err != nil {
		log.Printf("[ERROR] Record shipment failure: %#v", err)
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

func hostname() string {
	n, err := os.Hostname()
	if err != nil {
		return fmt.Sprintf("%#v", err)
	}
	return n
}

func cwd() string {
	c, err := os.Getwd()
	if err != nil {
		return fmt.Sprintf("%#v", err)
	}
	return c
}

func username() string {
	u, err := user.Current()
	if err != nil {
		return fmt.Sprintf("%#v", err)
	}
	return u.Name
}
