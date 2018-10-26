package dewy

import (
	"context"
	"log"
	"os"
	"path/filepath"
	"sync"
	"syscall"
	"time"

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
	root string
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

func (d *Dewy) Run() error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ntc := notice.New(&notice.Slack{Token: os.Getenv("SLACK_TOKEN")})
	ntc.Notify("Run starting", ctx)

	d.config.Repository.String()
	r := NewRepository(d.config.Repository, d.cache)

	if err := r.Fetch(); err != nil {
		log.Printf("[ERROR] Fetch failure: %#v", err)
		return err
	}

	if !r.IsDownloadNecessary() {
		log.Print("[DEBUG] Download skipped")
		return nil
	}
	ntc.Notify("Release downloading", ctx)

	key, err := r.Download()
	if err != nil {
		return nil
	}

	p := filepath.Join(d.cache.GetDir(), key)
	linkFrom, err := d.preserve(p)
	if err != nil {
		return err
	}

	linkTo := filepath.Join(d.root, "current")
	if _, err := os.Lstat(linkTo); err == nil {
		os.Remove(linkTo)
	}
	if err := os.Symlink(linkFrom, linkTo); err != nil {
		return err
	}

	if d.isServerRunning {
		ntc.Notify("Server restarting", ctx)
		return d.restartServer()
	}

	ntc.Notify("Server starting", ctx)
	return d.startServer()
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
