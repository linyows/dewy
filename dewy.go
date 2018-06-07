package dewy

import (
	"errors"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/linyows/dewy/kvs"
)

type Dewy struct {
	config     Config
	repository Repository
	cache      kvs.KVS
}

func New(c Config) *Dewy {
	kv := &kvs.File{}
	kv.Default()
	return &Dewy{
		config: c,
		cache:  kv,
	}
}

func (d *Dewy) Run() error {
	// c := New("file", kvs.Config)
	// c.Read(d.config.Repository.String())
	d.config.Repository.String()
	r := NewRepository(d.config.Repository, d.cache)

	if err := r.Fetch(); err != nil {
		return err
	}

	if !r.IsDownloadNecessary() {
		log.Print("[DEBUG] Download skipped")
		return nil
	}

	if err := r.Download(); err != nil {
		return err
	}

	//preserveDir := "/var/dewy/preserves"
	preserveDir, err := os.Getwd()
	if err != nil {
		return err
	}

	linkFrom, err := r.Preserve(preserveDir)
	if err != nil {
		return err
	}

	//linkTo := d.config.Starter.Command()
	linkTo := filepath.Join(preserveDir, "mox")
	if err := os.Symlink(linkFrom, linkTo); err != nil {
		return err
	}

	ch := make(chan error)
	go func() {
		counter := 0
		for {
			counter++
			time.Sleep(1 * time.Second)
			log.Printf("==> %d", counter)
		}
		//s, err := starter.NewStarter(d.config.Starter)
		//if err != nil {
		//	return
		//}
		//ch <- s.Run()
		ch <- errors.New("yo")
	}()

	return nil
}
