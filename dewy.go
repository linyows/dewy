package dewy

import (
	"log"

	"github.com/lestrrat-go/server-starter"
	"github.com/linyows/dewy/kvs"
)

type Dewy struct {
	config     Config
	repository Repository
	cache      kvs.KVS
	starter    starter.Config
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

	return nil
}
