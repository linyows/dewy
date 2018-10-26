package dewy

import (
	"context"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"github.com/carlescere/scheduler"
	starter "github.com/lestrrat-go/server-starter"
	"github.com/linyows/dewy/kvs"
	"github.com/linyows/dewy/notice"
)

type Dewy struct {
	config          Config
	repository      Repository
	cache           kvs.KVS
	isServerRunning bool
	sync.RWMutex
	root   string
	job    *scheduler.Job
	notice notice.Notice
}

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
		isServerRunning: false,
		root:            wd,
	}
}

func (d *Dewy) Start(i int) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	d.notice = notice.New(&notice.Slack{
		RepositoryURL: "https://" + d.config.Repository.String(),
		Token:         os.Getenv("SLACK_TOKEN"),
	})
	d.notice.Notify("Scheduler starting", ctx)

	var err error
	d.job, err = scheduler.Every(i).Seconds().Run(func() {
		d.Run()
	})

	if err != nil {
		log.Printf("[ERROR] Scheduler failure: %#v", err)
	}

	d.waitSigs()
	d.notice.Notify("Scheduler killed", ctx)
}

func (d *Dewy) waitSigs() {
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)
	sigReceived := <-sigCh
	log.Printf("[DEBUG] PID %d received signal as %s", os.Getpid(), sigReceived)
	d.job.Quit <- true
}

func (d *Dewy) Run() error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	d.config.Repository.String()
	d.repository = NewRepository(d.config.Repository, d.cache)

	if err := d.repository.Fetch(); err != nil {
		log.Printf("[ERROR] Fetch failure: %#v", err)
		return err
	}

	if !d.repository.IsDownloadNecessary() {
		log.Print("[DEBUG] Download skipped")
		return nil
	}

	d.notice.Notify("Release downloading", ctx)
	key, err := d.repository.Download()
	if err != nil {
		log.Printf("[ERROR] Download failure: %#v", err)
		return nil
	}

	if err := d.deploy(key); err != nil {
		return err
	}

	if d.isServerRunning {
		ntc.Notify("Server restarting", ctx)
		return d.restartServer()
	}

	ntc.Notify("Server starting", ctx)
	return d.startServer()
}

func (d *Dewy) deploy(key string) error {
	p := filepath.Join(d.cache.GetDir(), key)
	linkFrom, err := d.preserve(p)
	if err != nil {
		return err
	}

	linkTo := filepath.Join(d.root, "current")
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
	const prefix = "20060102150405MST"
	dst := filepath.Join(d.root, "preserves", time.Now().Format(prefix))
	if err := os.MkdirAll(dst, 0755); err != nil {
		return "", err
	}

	if err := kvs.ExtractArchive(p, dst); err != nil {
		return "", err
	}
	log.Printf("[INFO] Extract archive to %s", dst)

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
